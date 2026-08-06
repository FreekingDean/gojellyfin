package socket

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/http/middleware"
)

// The client treats an unanswered KeepAlive as a dead socket and reconnects, so
// the timeout is advertised as 60s and ForceKeepAlive is pushed at half that.
const (
	keepAliveTimeout  = 60
	keepAliveInterval = 30 * time.Second
)

var upgrader = websocket.Upgrader{
	// jellyfin-web is same-origin in prod; relax for dev, tighten later
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsMessage struct {
	MessageType string      `json:"MessageType"`
	Data        interface{} `json:"Data,omitempty"`
	MessageId   string      `json:"MessageId"`
}

type Socket struct {
	auth *auth.Service
}

func New(auth *auth.Service) *Socket {
	return &Socket{auth: auth}
}

// Browsers cannot set headers on a websocket handshake, so clients pass the
// access token as a query parameter instead.
func (s *Socket) Handle(w http.ResponseWriter, r *http.Request) {
	token := middleware.TokenFrom(r)
	if token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if _, err := s.auth.SessionByToken(r.Context(), token); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Gorilla allows one concurrent writer, so replies are funnelled to the
	// write loop below rather than sent from the reader.
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

			select {
			case replies <- wsMessage{MessageType: "KeepAlive", MessageId: newGUID()}:
			case <-done:
				return
			}
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

func forceKeepAlive() wsMessage {
	return wsMessage{MessageType: "ForceKeepAlive", Data: keepAliveTimeout, MessageId: newGUID()}
}

func newGUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b) // 32 hex chars, no dashes — matches Jellyfin's format
}
