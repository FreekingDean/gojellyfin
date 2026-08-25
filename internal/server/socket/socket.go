package socket

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
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
}

func New(sessions *sessions.Service) *Socket {
	return &Socket{sessions: sessions}
}

func (s *Socket) Handle(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFrom(r)
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if _, err := s.sessions.ByToken(r.Context(), token); err != nil {
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
