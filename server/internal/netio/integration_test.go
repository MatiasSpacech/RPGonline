package netio

// Test de integración F1: dos clientes WebSocket reales contra el game loop
// real. Verifica login, nombres únicos y que el movimiento de uno se replica
// al otro por snapshot.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"munext/server/internal/protocol"
	"munext/server/internal/world"
)

func loginAs(t *testing.T, conn *websocket.Conn, user string) {
	t.Helper()
	msg, err := protocol.Encode("login", protocol.Login{User: user})
	if err != nil {
		t.Fatalf("Encode login: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		t.Fatalf("write login: %v", err)
	}
}

func readWelcome(t *testing.T, conn *websocket.Conn) protocol.Welcome {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		_ = conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("leyendo welcome: %v", err)
		}
		env, err := protocol.Decode(data)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if env.T != "welcome" {
			continue
		}
		var welcome protocol.Welcome
		if err := json.Unmarshal(env.D, &welcome); err != nil {
			t.Fatalf("payload welcome: %v", err)
		}
		return welcome
	}
}

func TestTwoClientsSeeEachOther(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	gameWorld := world.New(log)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go gameWorld.Run(ctx)

	s := New(log, gameWorld.Inbox())
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	conn1 := dial(t, ts)
	conn2 := dial(t, ts)

	loginAs(t, conn1, "dev")
	welcome1 := readWelcome(t, conn1)
	loginAs(t, conn2, "dev")
	welcome2 := readWelcome(t, conn2)

	if welcome1.Self.ID == welcome2.Self.ID {
		t.Fatalf("los dos clientes recibieron el mismo id: %d", welcome1.Self.ID)
	}
	if welcome1.Self.Name == welcome2.Self.Name {
		t.Fatalf("nombres duplicados: %q", welcome1.Self.Name)
	}

	// El jugador 1 se mueve; el jugador 2 debe verlo acercarse a x=8.
	move, err := protocol.Encode("move_req", protocol.MoveReq{X: 8, Z: 0})
	if err != nil {
		t.Fatalf("Encode move: %v", err)
	}
	if err := conn1.WriteMessage(websocket.TextMessage, move); err != nil {
		t.Fatalf("write move: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	sawBoth := false
	var lastX float64
	for time.Now().Before(deadline) {
		_ = conn2.SetReadDeadline(deadline)
		_, data, err := conn2.ReadMessage()
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
			t.Fatalf("payload snapshot: %v", err)
		}

		ids := make(map[int64]bool, len(snap.Ents))
		for _, ent := range snap.Ents {
			ids[ent.ID] = true
			if ent.ID == welcome1.Self.ID {
				lastX = ent.X
			}
		}
		sawBoth = ids[welcome1.Self.ID] && ids[welcome2.Self.ID]
		if sawBoth && lastX > 1.0 {
			return // éxito: ve a ambos y al jugador 1 moviéndose
		}
	}

	t.Fatalf("timeout: sawBoth=%v, última x del jugador 1=%v", sawBoth, lastX)
}
