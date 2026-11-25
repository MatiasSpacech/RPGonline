package netio

// Test de integración F2: dos clientes reales contra el game loop real y una
// capa de cuentas en memoria. Verifica login, selección de personaje y que el
// movimiento de uno se replica al otro por snapshot.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"munext/server/internal/model"
	"munext/server/internal/protocol"
	"munext/server/internal/world"
)

type authedClient struct {
	conn  *websocket.Conn
	token string
	chars []model.CharacterSummary
}

func loginClient(t *testing.T, ts *httptest.Server, username, password string) authedClient {
	t.Helper()
	conn := dial(t, ts)
	send(t, conn, "login", protocol.Auth{User: username, Pass: password})

	env := readEnvelope(t, conn, "login_ok", 2*time.Second)
	var ok protocol.LoginOK
	if err := json.Unmarshal(env.D, &ok); err != nil {
		t.Fatalf("login_ok: %v", err)
	}
	return authedClient{conn: conn, token: ok.Token, chars: ok.Chars}
}

func (c authedClient) selectCharacter(t *testing.T, characterID int64) protocol.Welcome {
	t.Helper()
	send(t, c.conn, "select_char", protocol.SelectChar{CharID: characterID})

	env := readEnvelope(t, c.conn, "welcome", 2*time.Second)
	var w protocol.Welcome
	if err := json.Unmarshal(env.D, &w); err != nil {
		t.Fatalf("welcome: %v", err)
	}
	return w
}

func TestTwoClientsSeeEachOther(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	gameWorld := world.New(log, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go gameWorld.Run(ctx)

	fake := newFakeAccounts()
	user1 := fake.addUser("matias", "123456")
	char1 := fake.addCharacter(user1.ID, "Kaen", model.ClassWarrior)
	user2 := fake.addUser("otro", "123456")
	char2 := fake.addCharacter(user2.ID, "Lyr", model.ClassMage)

	s := New(log, gameWorld.Inbox(), fake)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	client1 := loginClient(t, ts, "matias", "123456")
	welcome1 := client1.selectCharacter(t, char1.ID)
	client2 := loginClient(t, ts, "otro", "123456")
	welcome2 := client2.selectCharacter(t, char2.ID)

	if welcome1.Character.ID == welcome2.Character.ID {
		t.Fatalf("los dos clientes seleccionaron el mismo personaje: %d", welcome1.Character.ID)
	}
	if welcome1.Character.Name != "Kaen" || welcome2.Character.Name != "Lyr" {
		t.Fatalf("personajes = %q / %q", welcome1.Character.Name, welcome2.Character.Name)
	}

	// El jugador 1 se mueve; el jugador 2 debe verlo acercarse a x=8.
	send(t, client1.conn, "move_req", protocol.MoveReq{X: 8, Z: 0})

	deadline := time.Now().Add(5 * time.Second)
	sawBoth := false
	var lastX float64
	for time.Now().Before(deadline) {
		_ = client2.conn.SetReadDeadline(deadline)
		_, data, err := client2.conn.ReadMessage()
		if err != nil {
			t.Fatalf("leyendo snapshots: %v", err)
		}
		env, err := protocol.Decode(data)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if env.T != "snapshot" {
			continue
		}
		var snap protocol.Snapshot
		if err := json.Unmarshal(env.D, &snap); err != nil {
			t.Fatalf("snapshot: %v", err)
		}

		ids := make(map[int64]bool, len(snap.Ents))
		for _, ent := range snap.Ents {
			ids[ent.ID] = true
			if ent.ID == char1.ID {
				lastX = ent.X
			}
		}
		sawBoth = ids[char1.ID] && ids[char2.ID]
		if sawBoth && lastX > 1.0 {
			return // éxito
		}
	}

	t.Fatalf("timeout: sawBoth=%v, última x del jugador 1=%v", sawBoth, lastX)
}
