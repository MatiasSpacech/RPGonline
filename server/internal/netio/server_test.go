package netio

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"munext/server/internal/protocol"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := New(log)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts
}

func dial(t *testing.T, ts *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v (respuesta: %v)", err, resp)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestPingPong(t *testing.T) {
	_, ts := newTestServer(t)
	conn := dial(t, ts)

	ping, err := protocol.Encode("ping", map[string]string{"client": "test"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, ping); err != nil {
		t.Fatalf("write: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	env, err := protocol.Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if env.T != "pong" {
		t.Fatalf("tipo = %q, quería %q", env.T, "pong")
	}

	var payload struct {
		ServerTimeMS int64 `json:"server_time_ms"`
	}
	if err := json.Unmarshal(env.D, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload.ServerTimeMS == 0 {
		t.Fatal("server_time_ms no debe ser cero")
	}
}

func TestInvalidMessageClosesConnection(t *testing.T) {
	_, ts := newTestServer(t)
	conn := dial(t, ts)

	if err := conn.WriteMessage(websocket.TextMessage, []byte("esto no es json")); err != nil {
		t.Fatalf("write: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("esperaba que el servidor cerrara la conexión")
	}
}

func TestCloseDisconnectsClients(t *testing.T) {
	s, ts := newTestServer(t)
	conn := dial(t, ts)

	// Un ping/pong primero garantiza que el handler ya registró la conexión.
	ping, err := protocol.Encode("ping", nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, ping); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read pong: %v", err)
	}

	s.Close()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("esperaba que Close desconectara al cliente")
	}
}
