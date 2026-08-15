//go:build e2e

package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/FreekingDean/gojellyfin/internal/env"
	"github.com/FreekingDean/gojellyfin/internal/libraries"
	"github.com/FreekingDean/gojellyfin/internal/store"
	itemmodal "github.com/FreekingDean/gojellyfin/internal/store/item"
)

const (
	username = "smoke"
	password = "smoke-password"
	library  = "Smoke Movies"
)

var movies = []string{"Fixture Alpha", "Fixture Beta"}

func TestSmoke(t *testing.T) {
	isolate(t, scratchDatabase(t))

	run(t, migrateCommand())
	addUser(t)
	libraryID := seed(t)

	c := &client{t: t, base: start(t)}

	t.Run("public system info", func(t *testing.T) {
		var info struct {
			Id, Version, ServerName string
		}
		c.get(t, "/System/Info/Public", http.StatusOK, &info)

		if info.Version == "" || info.Id == "" {
			t.Fatalf("the login page has no server to show: %+v", info)
		}
	})

	t.Run("anonymous access is refused", func(t *testing.T) {
		c.get(t, "/UserViews", http.StatusUnauthorized, nil)
	})

	t.Run("the wrong password is refused", func(t *testing.T) {
		c.post(t, "/Users/AuthenticateByName", authenticate(username, "not-the-password"), http.StatusUnauthorized, nil)
	})

	var session struct {
		AccessToken string
		User        struct{ Id, Name string }
	}
	c.post(t, "/Users/AuthenticateByName", authenticate(username, password), http.StatusOK, &session)
	if session.AccessToken == "" || session.User.Id == "" {
		t.Fatalf("logging in returned no session: %+v", session)
	}
	c.token = session.AccessToken

	t.Run("the token identifies the user", func(t *testing.T) {
		var me struct{ Id, Name string }
		c.get(t, "/Users/Me", http.StatusOK, &me)

		if me.Id != session.User.Id || me.Name != username {
			t.Fatalf("Users/Me = %+v, want %s (%s)", me, username, session.User.Id)
		}
	})

	t.Run("the library is a view", func(t *testing.T) {
		var views result
		c.get(t, "/UserViews", http.StatusOK, &views)

		view, ok := views.find(library)
		if !ok {
			t.Fatalf("%q is missing from the views: %v", library, views.names())
		}
		if view.Id != libraryID.String() {
			t.Errorf("view id = %s, want %s", view.Id, libraryID)
		}
		if view.CollectionType != "movies" {
			t.Errorf("collection type = %q, want movies", view.CollectionType)
		}
	})

	t.Run("the library lists its items through the alias the web client asks for", func(t *testing.T) {
		var items result
		c.get(t, "/Users/"+session.User.Id+"/Items?parentId="+libraryID.String()+"&sortBy=SortName", http.StatusOK, &items)

		for _, want := range movies {
			if _, ok := items.find(want); !ok {
				t.Errorf("%q is missing from the library: %v", want, items.names())
			}
		}
	})

	t.Run("an item opens and favouriting it sticks", func(t *testing.T) {
		var listing result
		c.get(t, "/Items?parentId="+libraryID.String(), http.StatusOK, &listing)

		item, ok := listing.find(movies[0])
		if !ok {
			t.Fatalf("%q is missing: %v", movies[0], listing.names())
		}

		c.post(t, "/UserFavoriteItems/"+item.Id, nil, http.StatusOK, nil)

		var reopened struct {
			Name     string
			UserData struct{ IsFavorite bool }
		}
		c.get(t, "/Items/"+item.Id, http.StatusOK, &reopened)

		if reopened.Name != movies[0] {
			t.Errorf("item name = %q, want %q", reopened.Name, movies[0])
		}
		if !reopened.UserData.IsFavorite {
			t.Error("the favourite did not survive a round trip to the database")
		}
	})

	t.Run("the websocket greets the client", func(t *testing.T) {
		c.socket(t)
	})
}

type client struct {
	t     *testing.T
	base  string
	token string
}

func (c *client) get(t *testing.T, path string, status int, into any) {
	t.Helper()
	c.do(t, http.MethodGet, path, nil, status, into)
}

func (c *client) post(t *testing.T, path string, body any, status int, into any) {
	t.Helper()
	c.do(t, http.MethodPost, path, body, status, into)
}

func (c *client) do(t *testing.T, method, path string, body any, status int, into any) {
	t.Helper()

	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to encode the body: %v", err)
		}
		payload = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(t.Context(), method, c.base+path, payload)
	if err != nil {
		t.Fatalf("failed to build %s %s: %v", method, path, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", c.authorization())

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("%s %s never answered: %v", method, path, err)
	}
	defer func() { _ = response.Body.Close() }()

	answer, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("%s %s answered a body that could not be read: %v", method, path, err)
	}
	if response.StatusCode != status {
		t.Fatalf("%s %s = %d, want %d: %s", method, path, response.StatusCode, status, excerpt(answer))
	}
	if into == nil {
		return
	}
	if err := json.Unmarshal(answer, into); err != nil {
		t.Fatalf("%s %s answered something that is not the documented shape: %v: %s", method, path, err, excerpt(answer))
	}
}

func excerpt(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 400 {
		return text[:400] + "…"
	}

	return text
}

func (c *client) authorization() string {
	return fmt.Sprintf(
		`MediaBrowser Client="smoke test", Device="go test", DeviceId="%s", Version="0.0.0", Token="%s"`,
		"smoke-e2e-device", c.token,
	)
}

func (c *client) socket(t *testing.T) {
	t.Helper()

	address := "ws" + strings.TrimPrefix(c.base, "http") + "/socket?api_key=" + c.token
	conn, _, err := websocket.DefaultDialer.DialContext(t.Context(), address, nil)
	if err != nil {
		t.Fatalf("the websocket refused an authenticated client: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("failed to set a read deadline: %v", err)
	}

	var greeting struct{ MessageType string }
	if err := conn.ReadJSON(&greeting); err != nil {
		t.Fatalf("the websocket said nothing: %v", err)
	}
	if greeting.MessageType != "ForceKeepAlive" {
		t.Errorf("greeting = %q, want ForceKeepAlive", greeting.MessageType)
	}
}

type entry struct {
	Id             string
	Name           string
	CollectionType string
}

type result struct {
	Items []entry
}

func (r result) find(name string) (entry, bool) {
	for _, item := range r.Items {
		if item.Name == name {
			return item, true
		}
	}

	return entry{}, false
}

func (r result) names() []string {
	found := make([]string, 0, len(r.Items))
	for _, item := range r.Items {
		found = append(found, item.Name)
	}

	return found
}

func authenticate(name, pw string) map[string]string {
	return map[string]string{"Username": name, "Pw": pw}
}

func scratchDatabase(t *testing.T) string {
	t.Helper()

	admin := os.Getenv("DATABASE_URL")
	if admin == "" {
		t.Fatal("DATABASE_URL is not set, so there is no server to create a scratch database on")
	}

	name := "gojellyfin_e2e_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	execute(t, admin, "CREATE DATABASE "+name)
	t.Cleanup(func() { execute(t, admin, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)") })

	parsed, err := url.Parse(admin)
	if err != nil {
		t.Fatalf("DATABASE_URL is not a URL: %v", err)
	}
	parsed.Path = "/" + name

	return parsed.String()
}

// Postgres does not parameterise a database name, so the statement is
// interpolated; the identifier is generated here rather than sent by anyone.
func execute(t *testing.T, dsn, statement string) {
	t.Helper()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("failed to open %s: %v", statement, err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), statement); err != nil {
		t.Fatalf("%q failed: %v", statement, err)
	}
}

func isolate(t *testing.T, dsn string) {
	t.Helper()

	for _, name := range []string{
		"PUBLISHED_SERVER_URL", "CORS_ORIGINS",
		"TRANSCODER_JOBS", "TRANSCODER_STALL_TIMEOUT",
		"TEMPORAL_HOSTPORT", "TEMPORAL_NAMESPACE",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("HTTP_PORT", strconv.Itoa(freePort(t)))
}

// Binding :0 hands back a port from the ephemeral range, so this can never
// take :8081 from a running `make dev`.
func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find a free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	if err := listener.Close(); err != nil {
		t.Fatalf("failed to release the port: %v", err)
	}

	return port
}

func run(t *testing.T, command *cobra.Command, args ...string) {
	t.Helper()

	command.SetArgs(args)
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("%s failed: %v", command.Name(), err)
	}
}

// The password is read from os.Stdin rather than a flag, so the only way to
// drive the command that bootstraps the first user is to hand it one.
func addUser(t *testing.T) {
	t.Helper()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to open a pipe: %v", err)
	}
	if _, err := write.WriteString(password + "\n"); err != nil {
		t.Fatalf("failed to write the password: %v", err)
	}
	if err := write.Close(); err != nil {
		t.Fatalf("failed to close the pipe: %v", err)
	}

	stdin := os.Stdin
	os.Stdin = read
	defer func() {
		os.Stdin = stdin
		_ = read.Close()
	}()

	run(t, addUserCommand(), username)
}

func seed(t *testing.T) uuid.UUID {
	t.Helper()

	client := open(t)

	record, err := libraries.New(client).CreateLibrary(t.Context(), library, libraries.CollectionTypeMovies, []string{"/fixtures"})
	if err != nil {
		t.Fatalf("failed to create the library: %v", err)
	}

	for _, name := range movies {
		_, err := client.Item.Create().
			SetLibraryID(record.ID).
			SetKind(itemmodal.KindMovie).
			SetMediaType(itemmodal.MediaTypeVideo).
			SetName(name).
			SetSortName(strings.ToLower(name)).
			SetPath("/fixtures/" + name + ".mkv").
			Save(t.Context())
		if err != nil {
			t.Fatalf("failed to create %q: %v", name, err)
		}
	}

	return record.ID
}

func open(t *testing.T) *store.Client {
	t.Helper()

	config, err := env.Load()
	if err != nil {
		t.Fatalf("failed to read the environment: %v", err)
	}

	connection, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("failed to open the scratch database: %v", err)
	}
	if err := connection.Start(); err != nil {
		t.Fatalf("failed to reach the scratch database: %v", err)
	}
	t.Cleanup(func() { _ = connection.Stop() })

	return connection.Client()
}

func start(t *testing.T) string {
	t.Helper()

	app := fx.New(serverModules, fx.NopLogger)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("the server did not start: %v", err)
	}
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := app.Stop(stop); err != nil {
			t.Errorf("the server did not stop cleanly: %v", err)
		}
	})

	base := "http://127.0.0.1:" + os.Getenv("HTTP_PORT")
	await(t, base)

	return base
}

// ListenAndServe runs in a goroutine and logs its failures rather than
// returning them, so a port that could not be bound only shows up as nothing
// ever answering.
func await(t *testing.T, base string) {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(base + "/System/Info/Public")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("nothing answered on %s within 20s", base)
}
