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

	"munext/server/internal/model"
	"munext/server/internal/protocol"
	"munext/server/internal/world"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server, chan world.Command, *fakeAccounts) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	inbox := make(chan world.Command, 16)
	fake := newFakeAccounts()
	s := New(log, inbox, fake)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts, inbox, fake
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

func send(t *testing.T, conn *websocket.Conn, mtype string, payload any) {
	t.Helper()
	msg, err := protocol.Encode(mtype, payload)
	if err != nil {
		t.Fatalf("Encode(%s): %v", mtype, err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		t.Fatalf("write %s: %v", mtype, err)
	}
}

func readEnvelope(t *testing.T, conn *websocket.Conn, want string, timeout time.Duration) protocol.Envelope {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		_ = conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("esperando %q: %v", want, err)
		}
		env, err := protocol.Decode(data)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if env.T == want {
			return env
		}
	}
}

func TestPingPong(t *testing.T) {
	_, ts, _, _ := newTestServer(t)
	conn := dial(t, ts)

	send(t, conn, "ping", map[string]string{"client": "test"})

	env := readEnvelope(t, conn, "pong", 2*time.Second)
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
	_, ts, _, _ := newTestServer(t)
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
	s, ts, _, _ := newTestServer(t)
	conn := dial(t, ts)

	send(t, conn, "ping", nil)
	readEnvelope(t, conn, "pong", 2*time.Second)

	s.Close()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("esperaba que Close desconectara al cliente")
	}
}

func TestAuthCreateAndSelectFlow(t *testing.T) {
	_, ts, inbox, fake := newTestServer(t)
	user := fake.addUser("dev", "123456")
	fake.addCharacter(user.ID, "Kaen", model.ClassWarrior)

	conn := dial(t, ts)
	send(t, conn, "login", protocol.Auth{User: "dev", Pass: "123456"})

	env := readEnvelope(t, conn, "login_ok", 2*time.Second)
	var loginOK protocol.LoginOK
	if err := json.Unmarshal(env.D, &loginOK); err != nil {
		t.Fatalf("login_ok: %v", err)
	}
	if loginOK.Token == "" {
		t.Fatal("login_ok debe incluir token")
	}
	if loginOK.User != "dev" || len(loginOK.Chars) != 1 || loginOK.Chars[0].Name != "Kaen" {
		t.Fatalf("login_ok = %+v", loginOK)
	}

	// Crear un segundo personaje.
	send(t, conn, "create_char", protocol.CreateChar{Name: "Lyr", Class: model.ClassMage})
	env = readEnvelope(t, conn, "char_created", 2*time.Second)
	var created protocol.CharacterCreated
	if err := json.Unmarshal(env.D, &created); err != nil {
		t.Fatalf("char_created: %v", err)
	}
	if created.Char.Name != "Lyr" || created.Char.Class != model.ClassMage {
		t.Fatalf("char_created = %+v", created)
	}

	// Seleccionar a Kaen y verificar que llega world.Join.
	send(t, conn, "select_char", protocol.SelectChar{CharID: loginOK.Chars[0].ID})
	select {
	case cmd := <-inbox:
		join, ok := cmd.(world.Join)
		if !ok {
			t.Fatalf("comando = %T, quería world.Join", cmd)
		}
		if join.Character.Name != "Kaen" || join.Character.ID != loginOK.Chars[0].ID {
			t.Fatalf("join = %+v", join.Character)
		}
		if join.Out == nil {
			t.Fatal("join.Out no debería ser nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó world.Join")
	}

	// Al estar en el mundo, move_req se reenvía.
	send(t, conn, "move_req", protocol.MoveReq{X: 4, Z: -2})
	select {
	case cmd := <-inbox:
		move, ok := cmd.(world.Move)
		if !ok {
			t.Fatalf("comando = %T, quería world.Move", cmd)
		}
		if move.X != 4 || move.Z != -2 {
			t.Fatalf("destino = (%v, %v)", move.X, move.Z)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó world.Move")
	}

	// attack / potion / alloc_stat también se reenvían al mundo.
	send(t, conn, "attack", protocol.Attack{Target: 1000001})
	select {
	case cmd := <-inbox:
		attack, ok := cmd.(world.Attack)
		if !ok || attack.Target != 1000001 {
			t.Fatalf("comando = %#v, quería world.Attack{Target: 1000001}", cmd)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó world.Attack")
	}

	send(t, conn, "potion", protocol.Potion{Kind: "hp"})
	select {
	case cmd := <-inbox:
		potion, ok := cmd.(world.Potion)
		if !ok || potion.Kind != "hp" {
			t.Fatalf("comando = %#v, quería world.Potion{Kind: hp}", cmd)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó world.Potion")
	}

	send(t, conn, "alloc_stat", protocol.AllocStat{Stat: "str"})
	select {
	case cmd := <-inbox:
		alloc, ok := cmd.(world.AllocStat)
		if !ok || alloc.Stat != "str" {
			t.Fatalf("comando = %#v, quería world.AllocStat{Stat: str}", cmd)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó world.AllocStat")
	}
}

func TestWrongPasswordKeepsConnectionUsable(t *testing.T) {
	_, ts, _, fake := newTestServer(t)
	fake.addUser("dev", "123456")

	conn := dial(t, ts)
	send(t, conn, "login", protocol.Auth{User: "dev", Pass: "incorrecta"})

	env := readEnvelope(t, conn, "err", 2*time.Second)
	var e protocol.Error
	_ = json.Unmarshal(env.D, &e)
	if e.Code != "auth_failed" {
		t.Fatalf("código = %q, quería auth_failed", e.Code)
	}

	// La conexión sigue viva: un ping debe responder.
	send(t, conn, "ping", nil)
	readEnvelope(t, conn, "pong", 2*time.Second)
}

func TestResumeWithToken(t *testing.T) {
	_, ts, _, fake := newTestServer(t)
	user := fake.addUser("dev", "123456")
	fake.addCharacter(user.ID, "Kaen", model.ClassWarrior)

	conn := dial(t, ts)
	send(t, conn, "login", protocol.Auth{User: "dev", Pass: "123456"})
	env := readEnvelope(t, conn, "login_ok", 2*time.Second)
	var loginOK protocol.LoginOK
	if err := json.Unmarshal(env.D, &loginOK); err != nil {
		t.Fatalf("login_ok: %v", err)
	}
	_ = conn.Close()

	// Nueva conexión: reanuda con el token, sin contraseña.
	conn2 := dial(t, ts)
	send(t, conn2, "resume", protocol.Resume{Token: loginOK.Token})
	env = readEnvelope(t, conn2, "login_ok", 2*time.Second)
	var resumed protocol.LoginOK
	if err := json.Unmarshal(env.D, &resumed); err != nil {
		t.Fatalf("login_ok resume: %v", err)
	}
	if resumed.Token != loginOK.Token || resumed.User != "dev" || len(resumed.Chars) != 1 {
		t.Fatalf("resume = %+v", resumed)
	}
}

func TestResumeWithBadToken(t *testing.T) {
	_, ts, _, _ := newTestServer(t)
	conn := dial(t, ts)

	send(t, conn, "resume", protocol.Resume{Token: "no-existe"})
	env := readEnvelope(t, conn, "err", 2*time.Second)
	var e protocol.Error
	_ = json.Unmarshal(env.D, &e)
	if e.Code != "session_expired" {
		t.Fatalf("código = %q, quería session_expired", e.Code)
	}
}

func TestLogoutSendsLeave(t *testing.T) {
	_, ts, inbox, fake := newTestServer(t)
	user := fake.addUser("dev", "123456")
	char := fake.addCharacter(user.ID, "Kaen", model.ClassWarrior)

	conn := dial(t, ts)
	send(t, conn, "login", protocol.Auth{User: "dev", Pass: "123456"})
	readEnvelope(t, conn, "login_ok", 2*time.Second)
	send(t, conn, "select_char", protocol.SelectChar{CharID: char.ID})

	select {
	case cmd := <-inbox:
		join, ok := cmd.(world.Join)
		if !ok {
			t.Fatalf("comando = %T, quería world.Join", cmd)
		}
		_ = join
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó world.Join")
	}

	_ = conn.Close()

	select {
	case cmd := <-inbox:
		if _, ok := cmd.(world.Leave); !ok {
			t.Fatalf("comando = %T, quería world.Leave", cmd)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó world.Leave al cerrar la conexión")
	}
}

func decodeError(t *testing.T, env protocol.Envelope) protocol.Error {
	t.Helper()
	var e protocol.Error
	if err := json.Unmarshal(env.D, &e); err != nil {
		t.Fatalf("error payload: %v", err)
	}
	return e
}
