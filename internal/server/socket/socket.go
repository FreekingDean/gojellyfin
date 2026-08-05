package socket

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
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

func SocketHandler(w http.ResponseWriter, r *http.Request) {
	// available if you want them: r.URL.Query().Get("api_key"), .Get("deviceId")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Reader goroutine: drain inbound (KeepAlive, SessionsStart, etc.).
	// We ignore the contents but MUST keep reading to detect disconnect.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Writer: push ForceKeepAlive every 30s (timeout is 60s, so send at half).
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		msg := wsMessage{MessageType: "ForceKeepAlive", Data: 60, MessageId: newGUID()}
		if err := conn.WriteJSON(msg); err != nil {
			return
		}
		select {
		case <-ticker.C:
		case <-done:
			return
		}
	}
}

func newGUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b) // 32 hex chars, no dashes — matches Jellyfin's format
}
