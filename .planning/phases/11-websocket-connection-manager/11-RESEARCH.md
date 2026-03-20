# Phase 11: WebSocket Connection Manager - Research

**Researched:** 2026-03-20
**Domain:** Go WebSocket client — outbound dial, reconnect, heartbeat, single-writer serialization
**Confidence:** HIGH

## Summary

Phase 11 builds `internal/connection/` — a `ConnectionManager` that dials outbound to the server via `wss://`, reconnects automatically with exponential backoff, fires heartbeat pings, serializes all writes through a single goroutine, and sends a `NodeRegister` frame on every connect/reconnect. All downstream phases (dispatch, instance management, cleanup) depend on this being solid.

The library decision is settled in STATE.md: `github.com/coder/websocket` (formerly `nhooyr.io/websocket`). This library's `Ping()` blocks until pong arrives (requires a concurrent reader), supports concurrent `Write()` calls natively, and uses `context.Context` throughout — which maps cleanly onto the connection lifecycle goroutine model needed here.

The single writer goroutine (XPORT-06) is an explicit requirement even though `coder/websocket` supports concurrent writes internally. It exists as an architectural constraint to make frame ordering deterministic and to give the manager a single shutdown point that drains the send queue. This is the correct design regardless of library capability.

**Primary recommendation:** `coder/websocket` v1.8.14 + hand-rolled backoff (no extra dependency) + `uber-go/goleak` for goroutine leak assertions in tests + `httptest.NewTLSServer` + `websocket.Accept` for the mock server.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
All implementation choices are at Claude's discretion — pure infrastructure phase.

### Claude's Discretion
- WebSocket library choice (gorilla/websocket vs nhooyr.io/websocket)
- Internal channel/goroutine architecture for write serialization
- Reconnect backoff strategy implementation (500ms–30s with jitter per success criteria)
- Heartbeat ping/pong implementation approach
- Mock server design for tests
- Error handling and logging patterns

### Deferred Ideas (OUT OF SCOPE)
None
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| XPORT-01 | Node connects outbound to server via WebSocket (`wss://`) with TLS | `websocket.Dial` + `DialOptions.HTTPHeader` for Bearer token; `httptest.NewTLSServer` for test |
| XPORT-02 | Node sends heartbeat ping every 30s; detects dead connection via pong timeout | `conn.Ping(ctx)` with timeout context; reader goroutine must run concurrently |
| XPORT-03 | Node automatically reconnects with exponential backoff (500ms–30s) with jitter | Hand-rolled backoff loop; `time.Sleep` between attempts; cap at 30s |
| XPORT-04 | Node re-registers with server after every reconnect | Send `NodeRegister` frame immediately after successful dial in reconnect loop |
| XPORT-05 | Node sends explicit disconnect frame on clean shutdown | Write disconnect `Envelope` before `conn.Close(StatusNormalClosure, "")` |
| XPORT-06 | All outbound WebSocket writes go through a single writer goroutine | Buffered `chan []byte` drained by one goroutine; all callers use `Send()` method |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/coder/websocket` | v1.8.14 | WebSocket dial, read, write, ping, close | Actively maintained (gorilla archived 2022); idiomatic context-based API; concurrent writes; explicit successor to nhooyr.io/websocket |
| `github.com/coder/websocket/wsjson` | (same module) | JSON encode/decode over WebSocket | Wraps `wsjson.Write(ctx, conn, v)` / `wsjson.Read` — eliminates manual marshal/TextMessage boilerplate |
| `github.com/rs/zerolog` | already in go.mod | Structured logging with fields | Already established in project |

### Supporting (tests only)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `go.uber.org/goleak` | v1.3.0+ | Goroutine leak detection | `defer goleak.VerifyNone(t)` in every connection lifecycle test |
| `net/http/httptest` | stdlib | TLS mock server | `httptest.NewTLSServer(handler)` for `wss://` tests |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `coder/websocket` | `gorilla/websocket` | Gorilla archived 2022; panics on concurrent writes (race); no context support |
| hand-rolled backoff | `cenkalti/backoff/v4` | cenkalti adds a dependency; the 500ms–30s range with jitter is 10 lines of stdlib math/rand |
| `goleak` | `runtime.NumGoroutine()` assertions | goleak gives better diagnostics (names leaked goroutines); preferred for test clarity |

**Installation:**
```bash
go get github.com/coder/websocket@v1.8.14
go get go.uber.org/goleak
```

**Version verification (run before writing go.mod entries):**
```bash
go list -m github.com/coder/websocket@latest
go list -m go.uber.org/goleak@latest
```

---

## Architecture Patterns

### Recommended Package Structure
```
internal/connection/
├── manager.go          # ConnectionManager type, Start/Stop, Send public API
├── dial.go             # dial loop with exponential backoff (private)
├── heartbeat.go        # ping ticker goroutine (private)
├── writer.go           # single writer goroutine and send channel (private)
├── register.go         # buildRegisterFrame() helper (private)
└── manager_test.go     # all tests including mock server, race, goroutine leak
```

### Pattern 1: ConnectionManager Lifecycle

**What:** A `ConnectionManager` struct owns the WebSocket connection and exposes `Start(ctx)`, `Stop()`, and `Send([]byte) error`. Internally it runs exactly three goroutines per live connection: dial/reconnect loop, reader goroutine (required for Ping to work), and single writer goroutine.

**When to use:** Every phase that needs to send frames calls `manager.Send(data)`. Nothing outside `internal/connection/` calls `conn.Write` directly.

```go
// Source: design derived from coder/websocket docs + project patterns
type ConnectionManager struct {
    cfg     *config.NodeConfig
    log     zerolog.Logger
    sendCh  chan []byte        // buffered; writer goroutine owns draining
    stopCh  chan struct{}
    stopped chan struct{}
}

func NewConnectionManager(cfg *config.NodeConfig, log zerolog.Logger) *ConnectionManager {
    return &ConnectionManager{
        cfg:     cfg,
        log:     log,
        sendCh:  make(chan []byte, 64),
        stopCh:  make(chan struct{}),
        stopped: make(chan struct{}),
    }
}

func (m *ConnectionManager) Send(data []byte) error {
    select {
    case m.sendCh <- data:
        return nil
    case <-m.stopped:
        return ErrStopped
    }
}
```

### Pattern 2: Dial Loop with Exponential Backoff

**What:** A `for` loop that dials, handles the connection (reader + writer goroutines), and on any connection error sleeps a backoff duration before retrying.

**When to use:** Reconnect loop inside `ConnectionManager.Start()`.

```go
// Source: https://pkg.go.dev/github.com/coder/websocket#Dial
func (m *ConnectionManager) dialLoop(ctx context.Context) {
    backoff := newBackoff(500*time.Millisecond, 30*time.Second)
    for {
        conn, _, err := websocket.Dial(ctx, m.cfg.ServerURL, &websocket.DialOptions{
            HTTPHeader: http.Header{
                "Authorization": []string{"Bearer " + m.cfg.ServerToken},
            },
        })
        if err != nil {
            delay := backoff.Next()
            m.log.Warn().Err(err).Dur("retry_in", delay).Msg("dial failed")
            select {
            case <-time.After(delay):
            case <-ctx.Done():
                return
            }
            continue
        }
        backoff.Reset()
        m.handleConn(ctx, conn) // blocks until connection dies
        // loop back to reconnect
        select {
        case <-ctx.Done():
            return
        default:
        }
    }
}
```

### Pattern 3: Hand-Rolled Backoff with Full Jitter

**What:** A simple struct tracking current delay, doubles on each call, caps at max, adds full jitter via `math/rand`.

**When to use:** Reconnect delay calculation inside the dial loop.

```go
// Source: AWS "Exponential Backoff And Jitter" algorithm, stdlib only
type backoffState struct {
    current time.Duration
    min     time.Duration
    max     time.Duration
}

func newBackoff(min, max time.Duration) *backoffState {
    return &backoffState{current: min, min: min, max: max}
}

func (b *backoffState) Next() time.Duration {
    // Full jitter: sleep = random_between(0, min(cap, base * 2^attempt))
    jittered := time.Duration(rand.Int63n(int64(b.current) + 1))
    b.current = min(b.current*2, b.max)
    return jittered
}

func (b *backoffState) Reset() {
    b.current = b.min
}
```

### Pattern 4: handleConn — Reader + Writer + Heartbeat

**What:** Once dialed, three goroutines coordinate via a shared context derived from the connection. Reader goroutine is required because `conn.Ping()` waits for the reader to consume the pong frame.

```go
// Source: https://pkg.go.dev/github.com/coder/websocket#Conn.Ping
func (m *ConnectionManager) handleConn(ctx context.Context, conn *websocket.Conn) {
    defer conn.CloseNow()

    // Send registration frame first
    if err := m.sendRegister(ctx, conn); err != nil {
        m.log.Error().Err(err).Msg("registration failed")
        return
    }

    connCtx, cancel := context.WithCancel(ctx)
    defer cancel()

    var wg sync.WaitGroup

    // Reader goroutine — required for Ping() to receive pongs
    wg.Add(1)
    go func() {
        defer wg.Done()
        defer cancel()
        for {
            _, _, err := conn.Read(connCtx)
            if err != nil {
                if connCtx.Err() == nil {
                    m.log.Debug().Err(err).Msg("read error")
                }
                return
            }
            // dispatch inbound frames to handler (Phase 13)
        }
    }()

    // Heartbeat goroutine
    wg.Add(1)
    go func() {
        defer wg.Done()
        ticker := time.NewTicker(time.Duration(m.cfg.HeartbeatIntervalSecs) * time.Second)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                pingCtx, pcancel := context.WithTimeout(connCtx, 60*time.Second)
                err := conn.Ping(pingCtx)
                pcancel()
                if err != nil {
                    cancel()
                    return
                }
            case <-connCtx.Done():
                return
            }
        }
    }()

    // Single writer goroutine (XPORT-06)
    wg.Add(1)
    go func() {
        defer wg.Done()
        for {
            select {
            case data := <-m.sendCh:
                if err := conn.Write(connCtx, websocket.MessageText, data); err != nil {
                    cancel()
                    return
                }
            case <-connCtx.Done():
                return
            }
        }
    }()

    wg.Wait()
}
```

### Pattern 5: Clean Shutdown with Disconnect Frame

**What:** On `Stop()`, drain the connection with an explicit disconnect envelope, then close.

```go
// Send explicit disconnect before close (XPORT-05)
func (m *ConnectionManager) Stop() {
    close(m.stopCh)
    // caller signals context cancellation which unblocks dialLoop
    <-m.stopped // wait for full teardown
}
```

In `handleConn`, before returning on clean shutdown (ctx cancelled by caller, not by error), write a disconnect frame and call `conn.Close(websocket.StatusNormalClosure, "")`.

### Pattern 6: Mock Server for Tests

**What:** `httptest.NewTLSServer` with a `websocket.Accept` handler. Client uses `httptest.Server.Client()` as the HTTP client in `DialOptions` to trust the self-signed cert.

```go
// Source: net/http/httptest stdlib + https://pkg.go.dev/github.com/coder/websocket#Accept
func newMockServer(t *testing.T, handler http.HandlerFunc) (string, func()) {
    srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
            InsecureSkipVerify: true, // tests only
        })
        if err != nil {
            t.Logf("accept error: %v", err)
            return
        }
        handler(w, r) // or pass conn via closure
        _ = conn
    }))
    wsURL := "wss://" + srv.Listener.Addr().String()
    return wsURL, srv.Close
}

// Client DialOptions for self-signed TLS:
opts := &websocket.DialOptions{
    HTTPClient: httptest.NewTLSServer(nil).Client(), // reuse test server's client
    HTTPHeader: http.Header{"Authorization": []string{"Bearer test-token"}},
}
```

**Better pattern** — pass the server's own client directly:

```go
srv := httptest.NewTLSServer(handler)
opts := &websocket.DialOptions{
    HTTPClient: srv.Client(), // trusts the test cert
    HTTPHeader: http.Header{"Authorization": []string{"Bearer test"}},
}
conn, _, err := websocket.Dial(ctx, "wss://"+srv.Listener.Addr().String(), opts)
```

### Anti-Patterns to Avoid

- **Using gorilla/websocket:** Archived 2022; panics on concurrent writes; no context support.
- **Calling conn.Write from multiple goroutines without XPORT-06:** Even though coder/websocket handles it safely, the requirement mandates a single writer for ordering guarantees.
- **Calling conn.Ping without a concurrent reader goroutine:** Ping blocks waiting for the reader to consume the pong; without a running `conn.Read` loop, Ping deadlocks.
- **Ignoring context cancellation in reader:** The reader goroutine must exit when connCtx is cancelled — otherwise the WaitGroup leaks on reconnect.
- **Using `conn.Writer()` (streaming writer) concurrently:** Unlike `conn.Write()`, `conn.Writer()` is NOT concurrent-safe; only one open writer at a time.
- **Setting MaxElapsedTime on backoff without a stop mechanism:** The reconnect loop should run forever until the context is cancelled, not until a time limit expires.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WebSocket framing, masking, upgrades | Custom TCP frame parser | `coder/websocket` | WebSocket spec has 20+ edge cases; the library is 5 years battle-tested |
| JSON over WebSocket | Manual `json.Marshal` + `conn.Write` | `wsjson.Write(ctx, conn, v)` | Handles text frame type correctly; one call vs two |
| TLS test certificate | Self-signed cert generation code | `httptest.NewTLSServer` | stdlib provides it; no openssl dependency |
| Goroutine leak assertions | `runtime.NumGoroutine()` polling | `goleak.VerifyNone(t)` | goleak names leaked goroutines with stack traces |

**Key insight:** The WebSocket protocol has numerous correctness requirements (masking, fragmentation, control frame interleaving, close handshake ordering) that are subtle to get right. `coder/websocket` encapsulates all of this; the connection manager only needs to handle reconnect logic and goroutine lifecycle.

---

## Common Pitfalls

### Pitfall 1: Ping Without Concurrent Reader
**What goes wrong:** `conn.Ping(ctx)` blocks indefinitely, causing the heartbeat goroutine to stall.
**Why it happens:** `coder/websocket` Ping sends the ping frame but does NOT read the pong itself — it waits for the reader goroutine to consume the pong frame from the connection.
**How to avoid:** Always start the reader goroutine before the heartbeat goroutine. Both share `connCtx`; either can cancel on error.
**Warning signs:** Heartbeat goroutine blocks at `conn.Ping(...)` call in tests; pong timeout fires immediately.

### Pitfall 2: Reader Goroutine Missing After Reconnect
**What goes wrong:** After the first connection drop and reconnect, pings stop working silently.
**Why it happens:** `handleConn` starts a new reader goroutine each time it is called, but if the WaitGroup is misused or the goroutine exits early, the next connection has no reader.
**How to avoid:** The reconnect loop calls `handleConn` fresh for every connection; `handleConn` always starts all three goroutines and waits for all three via WaitGroup.
**Warning signs:** `go test -race` shows no race but heartbeat test hangs after simulated disconnect.

### Pitfall 3: Send Channel Not Drained on Disconnect
**What goes wrong:** Callers that call `Send()` while a reconnect is in progress fill the buffered channel; old messages are delivered on the new connection in wrong order.
**Why it happens:** `sendCh` persists across reconnects (correct) but the writer goroutine exits with the old connection. Messages queue up and are replayed on reconnect.
**How to avoid:** This is actually the correct behavior for XPORT-04 — re-registration happens first, then queued sends follow. Document this explicitly. But if the queue could contain stale state, drain it before registering.
**Warning signs:** Registration frame is not the first frame the server receives after reconnect.

### Pitfall 4: Goroutine Leak on Test Timeout
**What goes wrong:** Test exits, but heartbeat goroutine is still sleeping in `time.After` for up to 30s.
**Why it happens:** Test context cancelled but goroutine checks `connCtx.Done()` only on select — if `time.After` fires first, the goroutine re-enters the loop before checking done.
**How to avoid:** Use `ticker.Stop()` with defer; always check `case <-connCtx.Done(): return` BEFORE the ticker case, or at minimum ensure both cases are present in every select.
**Warning signs:** `goleak.VerifyNone(t)` reports timer goroutines after test ends.

### Pitfall 5: TLS Cert Not Trusted in Test
**What goes wrong:** `websocket.Dial` returns x509 certificate signed by unknown authority.
**Why it happens:** `httptest.NewTLSServer` uses a self-signed cert; the default HTTP client doesn't trust it.
**How to avoid:** Pass `srv.Client()` as `DialOptions.HTTPClient` — this is the test server's pre-configured client that already trusts the test cert.
**Warning signs:** Dial error contains "x509" in the test output.

### Pitfall 6: Missed `TypeNodeDisconnect` Message Type
**What goes wrong:** XPORT-05 requires an explicit disconnect frame but `protocol/messages.go` has no `TypeNodeDisconnect` constant.
**Why it happens:** The protocol package (Phase 10) only defined types that were known at the time; disconnect type was not added.
**How to avoid:** Add `TypeNodeDisconnect = "node_disconnect"` to `internal/protocol/messages.go` as part of this phase. This is a backward-compatible addition.
**Warning signs:** Tests pass but server logs show clean-close connections with no preceding frame.

---

## Code Examples

Verified patterns from official sources:

### Dial with Bearer Token Auth
```go
// Source: https://pkg.go.dev/github.com/coder/websocket#DialOptions
conn, _, err := websocket.Dial(ctx, cfg.ServerURL, &websocket.DialOptions{
    HTTPClient: httpClient,  // nil for default; srv.Client() in tests
    HTTPHeader: http.Header{
        "Authorization": []string{"Bearer " + cfg.ServerToken},
    },
})
```

### Send JSON Envelope
```go
// Source: https://pkg.go.dev/github.com/coder/websocket/wsjson
env, err := protocol.Encode(protocol.TypeNodeRegister, uuid.New().String(), reg)
if err != nil {
    return err
}
return wsjson.Write(ctx, conn, env)
```

### Ping with Pong Timeout
```go
// Source: https://pkg.go.dev/github.com/coder/websocket#Conn.Ping
// Heartbeat ticker goroutine:
pingCtx, cancel := context.WithTimeout(connCtx, 3*heartbeatInterval)
err := conn.Ping(pingCtx)
cancel()
if err != nil {
    // pong not received within timeout — trigger reconnect
    connCancel()
    return
}
```

### Goroutine Leak Check in Test
```go
// Source: https://pkg.go.dev/go.uber.org/goleak
func TestConnectionManager_NoLeak(t *testing.T) {
    defer goleak.VerifyNone(t)
    // ... test code
}
```

### Mock TLS Server Accept
```go
// Source: net/http/httptest stdlib + https://pkg.go.dev/github.com/coder/websocket#Accept
srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
        InsecureSkipVerify: true,
    })
    if err != nil { return }
    defer c.CloseNow()
    // respond to pings automatically via conn.Read loop
    for {
        _, _, err := c.Read(r.Context())
        if err != nil { return }
    }
}))
defer srv.Close()
wsURL := "wss://" + srv.Listener.Addr().String()
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `gorilla/websocket` | `coder/websocket` | Gorilla archived 2022; coder forked nhooyr 2024 | No more concurrent-write panics; idiomatic context API |
| `nhooyr.io/websocket` | `github.com/coder/websocket` | 2024 (coder took over maintenance) | Same API, actively maintained, v1.8.14 current |
| Manual ping with SetReadDeadline | `conn.Ping(ctx)` with context timeout | coder/websocket design | Cleaner timeout; context cancels the whole connection |

**Deprecated/outdated:**
- `gorilla/websocket`: Archived; no security patches; panics on concurrent writes — do not use
- `nhooyr.io/websocket`: Redirects to `coder/websocket`; use the coder import path directly

---

## Open Questions

1. **TypeNodeDisconnect constant**
   - What we know: XPORT-05 requires an explicit disconnect frame; `protocol/messages.go` has no `TypeNodeDisconnect`
   - What's unclear: The exact JSON type string the server expects; does the server even parse it or just observe the WS close handshake?
   - Recommendation: Add `TypeNodeDisconnect = "node_disconnect"` to `protocol/messages.go` as part of Phase 11 Wave 0. Define a `NodeDisconnect` struct (empty or with reason string). Server can be aligned on this string during server implementation.

2. **Inbound frame routing**
   - What we know: The reader goroutine in `handleConn` reads frames but Phase 11 has no dispatcher (Phase 13)
   - What's unclear: Where to put received frames — drop them? buffer them? return them via a channel?
   - Recommendation: Expose a `Receive() <-chan *protocol.Envelope` channel on `ConnectionManager`. Phase 13 reads from it. Phase 11 just provides the channel; if no one reads, use a non-blocking send with drop and warning log.

3. **Send channel behavior when manager is stopped**
   - What we know: `Send()` should not block after stop
   - What's unclear: Whether to return an error or silently drop
   - Recommendation: Return `ErrStopped` sentinel error from `Send()` after stop. Callers decide how to handle.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go standard `testing` package (Go 1.24) |
| Config file | none — `go test ./...` standard |
| Quick run command | `go test ./internal/connection/... -timeout 30s` |
| Full suite command | `go test -race ./internal/connection/... -timeout 60s` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| XPORT-01 | Dials `wss://` with Bearer token, connection established | integration | `go test ./internal/connection/... -run TestDial -timeout 30s` | ❌ Wave 0 |
| XPORT-02 | Ping fires every 30s; no pong → reconnect within 90s | integration | `go test ./internal/connection/... -run TestHeartbeat -timeout 60s` | ❌ Wave 0 |
| XPORT-03 | Reconnects after server close; backoff is 500ms–30s range | integration | `go test ./internal/connection/... -run TestReconnect -timeout 60s` | ❌ Wave 0 |
| XPORT-04 | NodeRegister sent on first connect AND after every reconnect | integration | `go test ./internal/connection/... -run TestRegister -timeout 30s` | ❌ Wave 0 |
| XPORT-05 | Explicit disconnect frame sent before clean shutdown | integration | `go test ./internal/connection/... -run TestCleanShutdown -timeout 30s` | ❌ Wave 0 |
| XPORT-06 | Concurrent Send() from 10 goroutines; no panic; race-clean | stress/race | `go test -race ./internal/connection/... -run TestConcurrentSend -timeout 30s` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/connection/... -timeout 30s`
- **Per wave merge:** `go test -race ./internal/connection/... -timeout 60s`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `internal/connection/manager_test.go` — covers all XPORT requirements
- [ ] Add `TypeNodeDisconnect` and `NodeDisconnect` struct to `internal/protocol/messages.go`
- [ ] `go get github.com/coder/websocket@v1.8.14` — add to go.mod
- [ ] `go get go.uber.org/goleak` — add to go.mod (test dependency)

---

## Sources

### Primary (HIGH confidence)
- `https://pkg.go.dev/github.com/coder/websocket` — Dial, Write, Read, Ping, Close, CloseNow, CloseRead, DialOptions, AcceptOptions method signatures and concurrency guarantees
- `https://pkg.go.dev/github.com/coder/websocket/wsjson` — wsjson.Write/Read import path confirmed
- `https://pkg.go.dev/go.uber.org/goleak` — VerifyNone, VerifyTestMain API

### Secondary (MEDIUM confidence)
- `https://github.com/coder/websocket` — v1.8.14 confirmed as latest stable (Sep 5, 2025); maintenance status; nhooyr handoff confirmed
- `https://pkg.go.dev/github.com/cenkalti/backoff/v4` — DefaultInitialInterval 500ms, DefaultMaxInterval 60s; used to verify the backoff algorithm; not adding as dependency

### Tertiary (LOW confidence)
- WebSearch results on reconnect patterns and single-writer hub pattern — corroborated by official docs but sourced from blog posts

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — coder/websocket API verified directly on pkg.go.dev; version v1.8.14 confirmed
- Architecture: HIGH — patterns derived directly from official API docs (Ping concurrency requirement verified)
- Pitfalls: HIGH for Ping-without-reader (explicit in docs); MEDIUM for others (derived from API semantics)

**Research date:** 2026-03-20
**Valid until:** 2026-06-20 (90 days — coder/websocket is stable; check for new minor version before starting)
