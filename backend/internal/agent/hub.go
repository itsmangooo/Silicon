package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var (
	ErrDisconnected          = errors.New("Silicon Agent is disconnected")
	ErrCapabilityUnavailable = errors.New("Silicon Agent does not advertise the required capability")
)

type HeartbeatRecorder func(context.Context, uuid.UUID, uuid.UUID, Heartbeat) error

type Hub struct {
	logger           *slog.Logger
	record           HeartbeatRecorder
	heartbeatTimeout time.Duration
	mu               sync.RWMutex
	connections      map[uuid.UUID]*connection
}

type connection struct {
	agentID        uuid.UUID
	serverID       uuid.UUID
	capabilities   map[string]bool
	capabilitiesMu sync.RWMutex
	websocket      *websocket.Conn
	writeMu        sync.Mutex
	pendingMu      sync.Mutex
	pending        map[string]chan Result
	done           chan struct{}
	closeOnce      sync.Once
}

func NewHub(logger *slog.Logger, recorder HeartbeatRecorder, heartbeatTimeout ...time.Duration) *Hub {
	timeout := 45 * time.Second
	if len(heartbeatTimeout) > 0 && heartbeatTimeout[0] > 0 {
		timeout = heartbeatTimeout[0]
	}
	return &Hub{logger: logger, record: recorder, heartbeatTimeout: timeout, connections: make(map[uuid.UUID]*connection)}
}

func (h *Hub) Connected(serverID uuid.UUID) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.connections[serverID]
	return ok
}

func (h *Hub) Disconnect(serverID uuid.UUID) {
	h.mu.RLock()
	conn := h.connections[serverID]
	h.mu.RUnlock()
	if conn != nil {
		conn.close()
	}
}

func (h *Hub) Serve(w http.ResponseWriter, r *http.Request, agentID, serverID uuid.UUID, capabilities []string) error {
	upgrader := websocket.Upgrader{HandshakeTimeout: 10 * time.Second, CheckOrigin: func(request *http.Request) bool { return request.Header.Get("Origin") == "" }}
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}
	// Clear net/http's request deadlines after the connection is hijacked, then
	// enforce liveness with the Agent heartbeat rather than a normal HTTP timeout.
	_ = socket.UnderlyingConn().SetDeadline(time.Time{})
	_ = socket.SetReadDeadline(time.Now().Add(h.heartbeatTimeout))
	socket.SetReadLimit(2 << 20)
	conn := &connection{agentID: agentID, serverID: serverID, capabilities: capabilitySet(capabilities), websocket: socket, pending: make(map[string]chan Result), done: make(chan struct{})}
	h.mu.Lock()
	old := h.connections[serverID]
	h.connections[serverID] = conn
	h.mu.Unlock()
	if old != nil {
		old.close()
	}
	h.logger.Info("agent connected", "agent_id", agentID, "server_id", serverID)
	defer func() {
		conn.close()
		h.mu.Lock()
		if h.connections[serverID] == conn {
			delete(h.connections, serverID)
		}
		h.mu.Unlock()
		h.logger.Info("agent connection closed", "agent_id", agentID, "server_id", serverID)
	}()
	for {
		var envelope Envelope
		if err := socket.ReadJSON(&envelope); err != nil {
			return err
		}
		_ = socket.SetReadDeadline(time.Now().Add(h.heartbeatTimeout))
		switch envelope.Type {
		case "heartbeat":
			if envelope.Heartbeat == nil || h.record == nil {
				continue
			}
			if err := h.record(r.Context(), agentID, serverID, *envelope.Heartbeat); err != nil {
				return fmt.Errorf("record heartbeat: %w", err)
			}
			conn.capabilitiesMu.Lock()
			conn.capabilities = capabilitySet(envelope.Heartbeat.Capabilities)
			conn.capabilitiesMu.Unlock()
		case "result":
			if envelope.Result == nil {
				continue
			}
			conn.pendingMu.Lock()
			output := conn.pending[envelope.Result.CommandID]
			conn.pendingMu.Unlock()
			if output != nil {
				select {
				case output <- *envelope.Result:
				case <-conn.done:
				}
				if envelope.Result.Final {
					conn.removePending(envelope.Result.CommandID)
				}
			}
		}
	}
}

func (h *Hub) Call(ctx context.Context, serverID uuid.UUID, capability string, command Command) (<-chan Result, error) {
	h.mu.RLock()
	conn := h.connections[serverID]
	h.mu.RUnlock()
	if conn == nil {
		return nil, ErrDisconnected
	}
	conn.capabilitiesMu.RLock()
	available := conn.capabilities[capability]
	conn.capabilitiesMu.RUnlock()
	if !available {
		return nil, ErrCapabilityUnavailable
	}
	if command.ID == "" {
		command.ID = uuid.NewString()
	}
	output := make(chan Result, 64)
	conn.pendingMu.Lock()
	conn.pending[command.ID] = output
	conn.pendingMu.Unlock()
	conn.writeMu.Lock()
	err := conn.websocket.WriteJSON(Envelope{Type: "command", Command: &command})
	conn.writeMu.Unlock()
	if err != nil {
		conn.removePending(command.ID)
		return nil, ErrDisconnected
	}
	go func() {
		select {
		case <-ctx.Done():
			conn.removePending(command.ID)
		case <-conn.done:
			conn.removePending(command.ID)
		}
	}()
	return output, nil
}

func (h *Hub) SendArchive(ctx context.Context, serverID uuid.UUID, commandID string, source io.Reader) error {
	h.mu.RLock()
	conn := h.connections[serverID]
	h.mu.RUnlock()
	if conn == nil {
		return ErrDisconnected
	}
	buffer := make([]byte, 512<<10)
	for {
		read, err := source.Read(buffer)
		if read > 0 {
			chunk := append([]byte(nil), buffer[:read]...)
			conn.writeMu.Lock()
			writeErr := conn.websocket.WriteJSON(Envelope{Type: "archive", Archive: &ArchiveChunk{CommandID: commandID, Data: chunk}})
			conn.writeMu.Unlock()
			if writeErr != nil {
				return ErrDisconnected
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	conn.writeMu.Lock()
	err := conn.websocket.WriteJSON(Envelope{Type: "archive", Archive: &ArchiveChunk{CommandID: commandID, Final: true}})
	conn.writeMu.Unlock()
	return err
}

func (c *connection) removePending(id string) {
	c.pendingMu.Lock()
	delete(c.pending, id)
	c.pendingMu.Unlock()
}

func (c *connection) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.websocket.Close()
		c.pendingMu.Lock()
		for id, output := range c.pending {
			select {
			case output <- Result{CommandID: id, Final: true, Error: ErrDisconnected.Error()}:
			default:
			}
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()
	})
}

func capabilitySet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
