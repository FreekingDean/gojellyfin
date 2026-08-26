package socket

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
	"github.com/FreekingDean/gojellyfin/internal/notify"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
)

const (
	keepAliveTimeout  = 60
	keepAliveInterval = 30 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsMessage struct {
	MessageType string      `json:"MessageType"`
	Data        interface{} `json:"Data,omitempty"`
	MessageId   string      `json:"MessageId"`
}

type Socket struct {
	sessions *sessions.Service

	mu      sync.RWMutex
	clients map[uuid.UUID][]chan wsMessage
}

func New(sessions *sessions.Service) *Socket {
	return &Socket{sessions: sessions, clients: make(map[uuid.UUID][]chan wsMessage)}
}

func (s *Socket) Deliver(envelope notify.Envelope) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, sessionID := range envelope.SessionIDs {
		for _, out := range s.clients[sessionID] {
			enqueue(out, wsMessage{MessageType: envelope.Type, Data: envelope.Data, MessageId: newGUID()})
		}
	}
}

func (s *Socket) register(sessionID uuid.UUID, out chan wsMessage) func() {
	s.mu.Lock()
	s.clients[sessionID] = append(s.clients[sessionID], out)
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		remaining := slices.DeleteFunc(s.clients[sessionID], func(c chan wsMessage) bool { return c == out })
		if len(remaining) == 0 {
			delete(s.clients, sessionID)
			return
		}

		s.clients[sessionID] = remaining
	}
}

func (s *Socket) Handle(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFrom(r)
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	session, err := s.sessions.ByToken(r.Context(), token)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	replies := make(chan wsMessage, 8)
	done := make(chan struct{})

	defer s.register(session.ID, replies)()

	go func() {
		defer close(done)
		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var message wsMessage
			if err := json.Unmarshal(payload, &message); err != nil {
				continue
			}
			if message.MessageType != "KeepAlive" {
				continue
			}

			enqueue(replies, wsMessage{MessageType: "KeepAlive", MessageId: newGUID()})
		}
	}()

	if err := conn.WriteJSON(forceKeepAlive()); err != nil {
		return
	}

	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case reply := <-replies:
			if err := conn.WriteJSON(reply); err != nil {
				return
			}
		case <-ticker.C:
			if err := conn.WriteJSON(forceKeepAlive()); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}

func enqueue(out chan wsMessage, message wsMessage) {
	select {
	case out <- message:
	default:
	}
}

func forceKeepAlive() wsMessage {
	return wsMessage{MessageType: "ForceKeepAlive", Data: keepAliveTimeout, MessageId: newGUID()}
}

func newGUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
