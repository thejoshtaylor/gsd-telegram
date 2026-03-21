// Package dispatch provides the Dispatcher, which receives commands from a
// ConnectionSender, manages Claude CLI instance lifecycles, streams output
// back to the server, and handles kill/status commands.
package dispatch

import (
	"context"
	"encoding/json"
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/user/gsd-tele-go/internal/audit"
	"github.com/user/gsd-tele-go/internal/claude"
	"github.com/user/gsd-tele-go/internal/config"
	"github.com/user/gsd-tele-go/internal/protocol"
	"github.com/user/gsd-tele-go/internal/security"
)

// ConnectionSender is the interface Dispatcher uses to send and receive messages.
// ConnectionManager satisfies this interface; mock implementations are used in tests.
type ConnectionSender interface {
	Send(data []byte) error
	Receive() <-chan *protocol.Envelope
}

// instanceState tracks a running Claude CLI instance.
type instanceState struct {
	instanceID string
	project    string
	sessionID  string
	cancel     context.CancelFunc
	startedAt  time.Time
	done       sync.Once // gates terminal event (InstanceFinished or InstanceError)
}

// Dispatcher receives commands from a ConnectionSender, manages Claude CLI
// instance lifecycles, and streams output back to the server.
type Dispatcher struct {
	conn    ConnectionSender
	cfg     *config.Config
	nodeCfg *config.NodeConfig
	log     zerolog.Logger
	audit   *audit.Logger
	limiter *security.ProjectRateLimiter

	mu        sync.RWMutex
	instances map[string]*instanceState

	wg     sync.WaitGroup
	stopCh chan struct{}
}

// New creates a Dispatcher. Call Run() to start processing commands.
func New(
	conn ConnectionSender,
	cfg *config.Config,
	nodeCfg *config.NodeConfig,
	auditLog *audit.Logger,
	limiter *security.ProjectRateLimiter,
	log zerolog.Logger,
) *Dispatcher {
	return &Dispatcher{
		conn:      conn,
		cfg:       cfg,
		nodeCfg:   nodeCfg,
		log:       log,
		audit:     auditLog,
		limiter:   limiter,
		instances: make(map[string]*instanceState),
		stopCh:    make(chan struct{}),
	}
}

// Run processes inbound commands from conn.Receive() until ctx is cancelled or
// Stop() is called. Run blocks until either exit condition is met.
func (d *Dispatcher) Run(ctx context.Context) {
	recvCh := d.conn.Receive()
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case env, ok := <-recvCh:
			if !ok {
				return
			}
			d.dispatch(ctx, env)
		}
	}
}

// dispatch routes an inbound envelope to the appropriate handler.
func (d *Dispatcher) dispatch(ctx context.Context, env *protocol.Envelope) {
	// Audit the inbound command. Best-effort decode of payload for extra fields.
	evt := audit.NewEvent(env.Type, env.ID, d.nodeCfg.NodeID)

	// Try to extract instance_id and project from the payload for richer audit events.
	switch env.Type {
	case protocol.TypeExecute:
		var cmd protocol.ExecuteCmd
		if err := env.Decode(&cmd); err == nil {
			evt.InstanceID = cmd.InstanceID
			evt.Project = cmd.Project
		}
	case protocol.TypeKill:
		var cmd protocol.KillCmd
		if err := env.Decode(&cmd); err == nil {
			evt.InstanceID = cmd.InstanceID
		}
	}

	if err := d.audit.Log(evt); err != nil {
		d.log.Warn().Err(err).Str("type", env.Type).Msg("failed to write audit log")
	}

	d.log.Info().Str("type", env.Type).Str("msg_id", env.ID).Msg("received command")

	switch env.Type {
	case protocol.TypeExecute:
		d.handleExecute(ctx, env)
	case protocol.TypeKill:
		d.handleKill(env)
	case protocol.TypeStatusRequest:
		d.handleStatusRequest(env)
	default:
		d.log.Warn().Str("type", env.Type).Msg("unknown command type")
	}
}

// handleExecute processes an execute command: validates rate limit, registers
// the instance, sends ACK, and spawns the instance goroutine.
func (d *Dispatcher) handleExecute(ctx context.Context, env *protocol.Envelope) {
	var cmd protocol.ExecuteCmd
	if err := env.Decode(&cmd); err != nil {
		d.log.Error().Err(err).Str("msg_id", env.ID).Msg("failed to decode execute command")
		return
	}

	// Check rate limit before doing anything else.
	if d.cfg.RateLimitEnabled && !d.limiter.Allow(cmd.Project) {
		d.log.Warn().Str("project", cmd.Project).Str("instance_id", cmd.InstanceID).Msg("rate limited")
		d.sendEnvelope(protocol.TypeInstanceError, protocol.NewMsgID(), protocol.InstanceError{
			InstanceID: cmd.InstanceID,
			Error:      "rate limited",
		})
		return
	}

	// Register instance in map BEFORE spawning goroutine (prevents kill race).
	instCtx, cancel := context.WithCancel(ctx)
	inst := &instanceState{
		instanceID: cmd.InstanceID,
		project:    cmd.Project,
		sessionID:  cmd.SessionID,
		cancel:     cancel,
		startedAt:  time.Now(),
	}
	d.mu.Lock()
	d.instances[cmd.InstanceID] = inst
	d.mu.Unlock()

	// Send ACK using env.ID for correlation.
	d.sendEnvelope(protocol.TypeACK, env.ID, protocol.ACK{InstanceID: cmd.InstanceID})

	// Spawn instance goroutine.
	d.wg.Add(1)
	go d.runInstance(instCtx, cmd)
}

// runInstance runs the Claude CLI for a single instance. It emits lifecycle
// events (InstanceStarted, StreamEvent*, InstanceFinished or InstanceError).
func (d *Dispatcher) runInstance(ctx context.Context, cmd protocol.ExecuteCmd) {
	defer func() {
		d.removeInstance(cmd.InstanceID)
		d.wg.Done()
	}()

	// Look up instance state for the done Once.
	d.mu.RLock()
	inst := d.instances[cmd.InstanceID]
	d.mu.RUnlock()

	// If somehow removed already, nothing to do.
	if inst == nil {
		return
	}

	// Send InstanceStarted.
	d.sendEnvelope(protocol.TypeInstanceStarted, protocol.NewMsgID(), protocol.InstanceStarted{
		InstanceID: cmd.InstanceID,
		Project:    cmd.Project,
		SessionID:  cmd.SessionID,
	})

	// Per-instance structured logger.
	instLog := d.log.With().
		Str("node_id", d.nodeCfg.NodeID).
		Str("instance_id", cmd.InstanceID).
		Str("project", cmd.Project).
		Logger()

	instLog.Info().Msg("instance started")

	// Build args — if SessionID is set, BuildArgs adds --resume (INST-07).
	args := claude.BuildArgs(cmd.SessionID, d.cfg.AllowedPaths, "", d.cfg.SafetyPrompt)

	// Determine working directory: prefer cmd.WorkDir, fall back to cfg.WorkingDir.
	workDir := cmd.WorkDir
	if workDir == "" {
		workDir = d.cfg.WorkingDir
	}

	// Start Claude CLI subprocess.
	proc, err := claude.NewProcess(ctx, d.cfg.ClaudeCLIPath, workDir, cmd.Prompt, args, config.FilteredEnv())
	if err != nil {
		instLog.Error().Err(err).Msg("failed to start Claude CLI")
		inst.done.Do(func() {
			d.sendEnvelope(protocol.TypeInstanceError, protocol.NewMsgID(), protocol.InstanceError{
				InstanceID: cmd.InstanceID,
				Error:      err.Error(),
			})
		})
		return
	}

	// Stream NDJSON events from Claude CLI.
	streamErr := proc.Stream(ctx, func(event claude.ClaudeEvent) error {
		data, marshalErr := json.Marshal(event)
		if marshalErr != nil {
			instLog.Warn().Err(marshalErr).Msg("failed to marshal stream event")
			return nil
		}
		d.sendEnvelope(protocol.TypeStreamEvent, protocol.NewMsgID(), protocol.StreamEvent{
			InstanceID: cmd.InstanceID,
			Data:       string(data),
		})
		return nil
	})

	// Capture final session ID from the process.
	finalSessionID := proc.SessionID()
	if finalSessionID != "" {
		d.mu.Lock()
		if cur, ok := d.instances[cmd.InstanceID]; ok {
			cur.sessionID = finalSessionID
		}
		d.mu.Unlock()
	}

	// Emit terminal event exactly once (kill + natural exit race guard).
	inst.done.Do(func() {
		if streamErr != nil && ctx.Err() == nil {
			// Stream error that is NOT due to context cancellation.
			instLog.Error().Err(streamErr).Msg("instance stream error")
			d.sendEnvelope(protocol.TypeInstanceError, protocol.NewMsgID(), protocol.InstanceError{
				InstanceID: cmd.InstanceID,
				Error:      streamErr.Error(),
			})
		} else {
			instLog.Info().Msg("instance finished")
			d.sendEnvelope(protocol.TypeInstanceFinished, protocol.NewMsgID(), protocol.InstanceFinished{
				InstanceID: cmd.InstanceID,
				ExitCode:   0,
			})
		}
	})
}

// handleKill processes a kill command by cancelling the target instance context.
func (d *Dispatcher) handleKill(env *protocol.Envelope) {
	var cmd protocol.KillCmd
	if err := env.Decode(&cmd); err != nil {
		d.log.Error().Err(err).Str("msg_id", env.ID).Msg("failed to decode kill command")
		return
	}

	d.mu.RLock()
	inst, ok := d.instances[cmd.InstanceID]
	d.mu.RUnlock()

	if !ok {
		d.log.Warn().Str("instance_id", cmd.InstanceID).Msg("kill: instance not found")
		return
	}

	d.log.Info().Str("instance_id", cmd.InstanceID).Msg("killing instance")
	inst.cancel()
}

// handleStatusRequest sends a NodeRegister response with the current running instances.
func (d *Dispatcher) handleStatusRequest(env *protocol.Envelope) {
	d.mu.RLock()
	summaries := make([]protocol.InstanceSummary, 0, len(d.instances))
	for _, inst := range d.instances {
		summaries = append(summaries, protocol.InstanceSummary{
			InstanceID: inst.instanceID,
			Project:    inst.project,
			SessionID:  inst.sessionID,
		})
	}
	d.mu.RUnlock()

	reg := protocol.NodeRegister{
		NodeID:           d.nodeCfg.NodeID,
		Platform:         runtime.GOOS,
		Version:          protocol.Version,
		Projects:         []string{},
		RunningInstances: summaries,
	}

	d.sendEnvelope(protocol.TypeNodeRegister, env.ID, reg)
}

// sendEnvelope encodes and sends a typed payload. If msgID is empty, a new ID is generated.
// Send errors are logged but not propagated (expected during shutdown).
func (d *Dispatcher) sendEnvelope(msgType, msgID string, payload any) {
	if msgID == "" {
		msgID = protocol.NewMsgID()
	}

	env, err := protocol.Encode(msgType, msgID, payload)
	if err != nil {
		d.log.Error().Err(err).Str("type", msgType).Msg("failed to encode envelope")
		return
	}

	data, err := json.Marshal(env)
	if err != nil {
		d.log.Error().Err(err).Str("type", msgType).Msg("failed to marshal envelope")
		return
	}

	if err := d.conn.Send(data); err != nil {
		d.log.Warn().Err(err).Str("type", msgType).Msg("failed to send envelope")
	}
}

// removeInstance removes an instance from the running instances map.
func (d *Dispatcher) removeInstance(instanceID string) {
	d.mu.Lock()
	delete(d.instances, instanceID)
	d.mu.Unlock()
}

// Stop signals the dispatcher to stop processing new commands. It does not
// wait for running instances to finish — call Wait() for that.
func (d *Dispatcher) Stop() {
	select {
	case <-d.stopCh:
		// Already stopped.
	default:
		close(d.stopCh)
	}
}

// Wait blocks until all running instance goroutines have completed.
func (d *Dispatcher) Wait() {
	d.wg.Wait()
}
