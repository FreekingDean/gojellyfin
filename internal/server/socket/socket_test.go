package socket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/FreekingDean/gojellyfin/internal/notify"
	"github.com/FreekingDean/gojellyfin/internal/sessions"
	"github.com/FreekingDean/gojellyfin/internal/store"
)

type fixture struct {
	socket    *Socket
	token     string
	sessionID uuid.UUID
	returned  chan struct{}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	connection, err := store.NewStore()
	if err != nil {
		t.Fatalf("failed to open the database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the database, set DATABASE_URL: %v", err)
	}

	ctx := context.Background()
	client := connection.Client()
	name := t.Name() + "-" + uuid.NewString()

	user, err := client.User.Create().SetName(name).SetUsername(name).SetPasswordHash(name).Save(ctx)
	if err != nil {
		t.Fatalf("failed to create the user: %v", err)
	}

	service := sessions.New(client)
	token := uuid.NewString()
	session, err := service.Create(ctx, user.ID, token, sessions.DeviceInfo{
		ID:         name,
		Name:       name,
		AppName:    "socket-test",
		AppVersion: "1",
	})
	if err != nil {
		t.Fatalf("failed to create the session: %v", err)
	}

	t.Cleanup(func() {
		if err := service.RemoveDevice(ctx, name); err != nil {
			t.Errorf("failed to delete the device: %v", err)
		}
		if err := client.User.DeleteOne(user).Exec(ctx); err != nil {
			t.Errorf("failed to delete the user: %v", err)
		}
		if err := connection.Stop(); err != nil {
			t.Errorf("failed to close the database: %v", err)
		}
	})

	return &fixture{
		socket:    New(service),
		token:     token,
		sessionID: session.ID,
		returned:  make(chan struct{}, 1),
	}
}

func (f *fixture) connect(t *testing.T) *websocket.Conn {
	t.Helper()

	handler := func(w http.ResponseWriter, r *http.Request) {
		f.socket.Handle(w, r)
		f.returned <- struct{}{}
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(server.Close)

	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(server.URL, "http")+"/socket?api_key="+f.token,
		nil,
	)
	if err != nil {
		t.Fatalf("failed to dial the socket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

func read(t *testing.T, conn *websocket.Conn) wsMessage {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("failed to set the read deadline: %v", err)
	}

	var message wsMessage
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatalf("failed to read from the socket: %v", err)
	}

	return message
}

func TestForceKeepAliveOpensTheSocket(t *testing.T) {
	conn := newFixture(t).connect(t)

	message := read(t, conn)
	if message.MessageType != "ForceKeepAlive" {
		t.Errorf("want ForceKeepAlive first, got %q", message.MessageType)
	}
	if message.Data != float64(keepAliveTimeout) {
		t.Errorf("want the timeout advertised as %d, got %v", keepAliveTimeout, message.Data)
	}
	if message.MessageId == "" {
		t.Error("want a message id")
	}
}

func TestKeepAliveIsAnswered(t *testing.T) {
	conn := newFixture(t).connect(t)
	read(t, conn)

	if err := conn.WriteJSON(wsMessage{MessageType: "KeepAlive", MessageId: newGUID()}); err != nil {
		t.Fatalf("failed to write to the socket: %v", err)
	}

	if message := read(t, conn); message.MessageType != "KeepAlive" {
		t.Errorf("want a KeepAlive reply, got %q", message.MessageType)
	}
}

func TestAnUnknownMessageIsIgnored(t *testing.T) {
	conn := newFixture(t).connect(t)
	read(t, conn)

	if err := conn.WriteJSON(wsMessage{MessageType: "SessionsStart", MessageId: newGUID()}); err != nil {
		t.Fatalf("failed to write to the socket: %v", err)
	}
	if err := conn.WriteJSON(wsMessage{MessageType: "KeepAlive", MessageId: newGUID()}); err != nil {
		t.Fatalf("failed to write to the socket: %v", err)
	}

	if message := read(t, conn); message.MessageType != "KeepAlive" {
		t.Errorf("want the unknown message skipped and the KeepAlive answered, got %q", message.MessageType)
	}
}

func TestHandleReturnsWhenTheConnectionCloses(t *testing.T) {
	fixture := newFixture(t)
	conn := fixture.connect(t)
	read(t, conn)

	if err := conn.Close(); err != nil {
		t.Fatalf("failed to close the connection: %v", err)
	}

	select {
	case <-fixture.returned:
	case <-time.After(5 * time.Second):
		t.Fatal("want Handle to return once the connection is gone")
	}
}

func TestEnqueueDropsWhenTheBufferIsFull(t *testing.T) {
	out := make(chan wsMessage, 1)
	enqueue(out, wsMessage{MessageType: "KeepAlive"})

	done := make(chan struct{})
	go func() {
		defer close(done)
		enqueue(out, wsMessage{MessageType: "KeepAlive"})
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("want a full buffer to drop the reply rather than block the reader")
	}
}

func TestDeliverReachesAConnectedSession(t *testing.T) {
	f := newFixture(t)
	conn := f.connect(t)
	read(t, conn)

	f.socket.Deliver(notify.Envelope{
		SessionIDs: []uuid.UUID{f.sessionID},
		Type:       "SyncPlayGroupUpdate",
		Data:       json.RawMessage(`{"Type":"UserJoined"}`),
	})

	message := read(t, conn)
	if message.MessageType != "SyncPlayGroupUpdate" {
		t.Fatalf("want SyncPlayGroupUpdate, got %q", message.MessageType)
	}

	data, ok := message.Data.(map[string]any)
	if !ok {
		t.Fatalf("want the update data, got %T", message.Data)
	}
	if data["Type"] != "UserJoined" {
		t.Fatalf("data is %v", data)
	}
}

// A member on another pod, or on none, has no entry in the registry. Dropping
// it must not hold up the members that are connected here.
func TestDeliverSkipsASessionThatIsNotConnected(t *testing.T) {
	f := newFixture(t)
	conn := f.connect(t)
	read(t, conn)

	f.socket.Deliver(notify.Envelope{
		SessionIDs: []uuid.UUID{uuid.New(), f.sessionID},
		Type:       "SyncPlayGroupUpdate",
		Data:       json.RawMessage(`{"Type":"UserLeft"}`),
	})

	if message := read(t, conn); message.MessageType != "SyncPlayGroupUpdate" {
		t.Fatalf("want SyncPlayGroupUpdate, got %q", message.MessageType)
	}
}

func TestDeliverAfterTheSocketClosesIsDropped(t *testing.T) {
	f := newFixture(t)
	conn := f.connect(t)
	read(t, conn)

	if err := conn.Close(); err != nil {
		t.Fatalf("failed to close the socket: %v", err)
	}
	<-f.returned

	f.socket.Deliver(notify.Envelope{
		SessionIDs: []uuid.UUID{f.sessionID},
		Type:       "SyncPlayGroupUpdate",
		Data:       json.RawMessage(`{"Type":"UserLeft"}`),
	})
}
