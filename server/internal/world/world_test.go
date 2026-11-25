package world

import (
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"testing"

	"munext/server/internal/protocol"
)

func testWorld() *World {
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestJoinNamesUniqueAndWelcome(t *testing.T) {
	w := testWorld()
	out1 := make(chan []byte, 4)
	out2 := make(chan []byte, 4)

	w.addPlayer(Join{SessionID: 1, User: "dev", Out: out1})
	w.addPlayer(Join{SessionID: 2, User: "dev", Out: out2})

	if got := w.players[1].Name; got != "dev" {
		t.Fatalf("nombre del jugador 1 = %q, quería %q", got, "dev")
	}
	if got := w.players[2].Name; got != "dev-2" {
		t.Fatalf("nombre del jugador 2 = %q, quería %q", got, "dev-2")
	}

	select {
	case data := <-out1:
		env, err := protocol.Decode(data)
		if err != nil {
			t.Fatalf("Decode welcome: %v", err)
		}
		if env.T != "welcome" {
			t.Fatalf("tipo = %q, quería welcome", env.T)
		}
		var welcome protocol.Welcome
		if err := json.Unmarshal(env.D, &welcome); err != nil {
			t.Fatalf("payload welcome: %v", err)
		}
		if welcome.Self.ID != 1 {
			t.Fatalf("self.id = %d, quería 1", welcome.Self.ID)
		}
		if welcome.Self.Name != "dev" {
			t.Fatalf("self.name = %q, quería dev", welcome.Self.Name)
		}
	default:
		t.Fatal("el jugador 1 no recibió welcome")
	}
}

func TestMoveClampsAndReachesTarget(t *testing.T) {
	w := testWorld()
	w.addPlayer(Join{SessionID: 1, User: "dev", Out: make(chan []byte, 8)})
	p := w.players[1]
	p.X, p.Z = 0, 0

	// Un destino fuera del mapa se recorta al borde.
	w.setTarget(Move{SessionID: 1, X: 100, Z: 0})
	if p.TargetX != MapHalfSize {
		t.Fatalf("target.x = %v, quería %v", p.TargetX, MapHalfSize)
	}
	if !p.HasTarget {
		t.Fatal("el jugador debería tener destino")
	}

	// Simular 4 segundos: 19 m a 6 m/s tardan ~3.2 s.
	for i := 0; i < TickRate*4; i++ {
		w.simulate(1.0 / TickRate)
	}
	if math.Abs(p.X-MapHalfSize) > 0.001 {
		t.Fatalf("x final = %v, quería %v", p.X, MapHalfSize)
	}
	if p.HasTarget {
		t.Fatal("el destino debería haberse alcanzado")
	}
	if p.Anim != "idle" {
		t.Fatalf("anim = %q, quería idle", p.Anim)
	}
}

func TestEntitiesSortedAndLeave(t *testing.T) {
	w := testWorld()
	w.addPlayer(Join{SessionID: 2, User: "b", Out: make(chan []byte, 4)})
	w.addPlayer(Join{SessionID: 1, User: "a", Out: make(chan []byte, 4)})

	ents := w.entities()
	if len(ents) != 2 {
		t.Fatalf("entidades = %d, quería 2", len(ents))
	}
	if ents[0].ID != 1 || ents[1].ID != 2 {
		t.Fatalf("orden = [%d, %d], quería [1, 2]", ents[0].ID, ents[1].ID)
	}

	Leave{SessionID: 1}.apply(w)
	if len(w.players) != 1 {
		t.Fatalf("jugadores = %d después de Leave, quería 1", len(w.players))
	}
}

func TestMoveForUnknownPlayerIsIgnored(t *testing.T) {
	w := testWorld()
	// No debe entrar en pánico ni crear jugadores.
	w.setTarget(Move{SessionID: 99, X: 1, Z: 1})
	if len(w.players) != 0 {
		t.Fatalf("jugadores = %d, quería 0", len(w.players))
	}
}
