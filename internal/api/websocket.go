package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"

	"nhooyr.io/websocket"
)

// ──────────────────── WebSocket Hub ────────────────────

type WSHub struct {
	mu          sync.RWMutex
	clients     map[*WSClient]bool
	activeTasks map[string]json.RawMessage // task_id → last task:update payload
	tasksMu     sync.RWMutex
}

type WSClient struct {
	conn   *websocket.Conn
	userID string
	send   chan []byte
}

type WSMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients:     make(map[*WSClient]bool),
		activeTasks: make(map[string]json.RawMessage),
	}
}

func (h *WSHub) Broadcast(event string, data interface{}) {
	msg, err := json.Marshal(WSMessage{Event: event, Data: data})
	if err != nil {
		return
	}

	// Track active task state for new client sync
	if event == "task:update" {
		h.trackTask(data, msg)
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		select {
		case client.send <- msg:
		default:
		}
	}
}

// trackTask keeps a snapshot of each running task so new clients get current state.
func (h *WSHub) trackTask(data interface{}, raw []byte) {
	m, ok := data.(map[string]interface{})
	if !ok {
		return
	}
	taskID, _ := m["task_id"].(string)
	status, _ := m["status"].(string)
	if taskID == "" {
		return
	}

	h.tasksMu.Lock()
	defer h.tasksMu.Unlock()
	if status == "complete" || status == "failed" {
		delete(h.activeTasks, taskID)
	} else {
		h.activeTasks[taskID] = json.RawMessage(raw)
	}
}

// sendActiveTasks replays current task state to a newly connected client.
func (h *WSHub) sendActiveTasks(client *WSClient) {
	h.tasksMu.RLock()
	defer h.tasksMu.RUnlock()
	for _, msg := range h.activeTasks {
		select {
		case client.send <- msg:
		default:
		}
	}
}

func (h *WSHub) addClient(c *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = true
}

func (h *WSHub) removeClient(c *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		close(c.send)
		delete(h.clients, c)
	}
}

func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ──────────────────── WebSocket Handler ────────────────────

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate via query param token
	token := r.URL.Query().Get("token")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	claims, err := s.auth.ValidateToken(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("WebSocket accept error: %v", err)
		return
	}

	client := &WSClient{
		conn:   conn,
		userID: claims.UserID.String(),
		send:   make(chan []byte, 64),
	}

	s.wsHub.addClient(client)
	s.wsHub.sendActiveTasks(client)
	log.Printf("WebSocket client connected: %s", claims.Username)

	ctx := r.Context()

	// Writer goroutine
	go func() {
		defer conn.Close(websocket.StatusNormalClosure, "")
		for msg := range client.send {
			if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
				return
			}
		}
	}()

	// Reader goroutine (keep connection alive, handle pings)
	for {
		_, _, err := conn.Read(ctx)
		if err != nil {
			break
		}
	}

	s.wsHub.removeClient(client)
	log.Printf("WebSocket client disconnected: %s", claims.Username)
}
