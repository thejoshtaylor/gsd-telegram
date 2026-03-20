---
gsd_state_version: 1.0
milestone: v1.2
milestone_name: Custom Webapp
status: unknown
stopped_at: Completed 11-01-PLAN.md
last_updated: "2026-03-20T23:48:54.483Z"
progress:
  total_phases: 5
  completed_phases: 1
  total_plans: 4
  completed_plans: 3
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-20)

**Core value:** Run and orchestrate multiple Claude Code instances across projects from a central server, with each node managing its own local Claude sessions independently.
**Current focus:** Phase 11 — websocket-connection-manager

## Current Position

Phase: 11 (websocket-connection-manager) — EXECUTING
Plan: 2 of 2

## Accumulated Context

### Decisions

- [Phase 09]: Echo filter runs before channel auth; admin lookup auth moots channel-ID config
- [v1.2]: Outbound WebSocket — nodes connect to server, not the other way around
- [v1.2]: Multiple Claude CLI instances per project directory
- [v1.2]: Remove Telegram and TypeScript entirely
- [v1.2]: Use `coder/websocket` — not gorilla (archived 2022, panics on concurrent writes)
- [v1.2]: Single writer goroutine for all WebSocket sends — correctness requirement, not optional
- [v1.2]: Deliver protocol spec + server backend spec as documentation deliverables
- [Phase 10]: RunningInstances field on NodeRegister has no omitempty tag — guarantees [] not null when empty
- [Phase 10]: Envelope.Payload is json.RawMessage — dispatch on Type before decoding avoids allocating unknown structs
- [Phase 10]: protocol package uses stdlib only (encoding/json) — no external dependencies
- [Phase 10-02]: machineid.ProtectedID used as primary node ID source, hostname sha256 fallback for containers/CI
- [Phase 10-02]: NodeConfig separate from Config — no Telegram env vars required for WebSocket config
- [Phase 11]: Two-phase stopCh check in Send() prevents race when sendCh has buffer capacity after Stop()
- [Phase 11]: recvCh uses non-blocking send with drop+warn to avoid stalling reader goroutine on inbound frames

### Pending Todos

None.

### Blockers/Concerns

- [Phase 10]: Server authentication handshake exact first-frame format and server ACK must be pinned during protocol design — coordinate with server team or decide unilaterally
- [Phase 11]: `coder/websocket` read deadline API differs from gorilla — verify method names before writing connection lifecycle code
- [Phase 12]: `sessions.json` migration: entries keyed by channels with no `mappings.json` match are unrecoverable — migration script must log losses; test on production copy before first v1.2 deploy

## Performance Metrics

**Velocity:**

- v1.1: 2 plans across 2 phases
- v1.0 reference: 24 plans across 7 phases

## Session Continuity

Last session: 2026-03-20T23:48:54.472Z
Stopped at: Completed 11-01-PLAN.md
Resume file: None
