package world

import (
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"testing"

	"munext/server/internal/model"
	"munext/server/internal/protocol"
)

func testWorld(saveCh chan model.CharacterState) *World {
	return New(slog.New(slog.NewTextHandler(io.Discard, nil)), saveCh)
}

func testCharacter(id int64, name string, x, z float64) model.Character {
	return model.Character{
		ID:    id,
		Name:  name,
		Class: model.ClassWarrior,
		Level: 1,
		X:     x,
		Z:     z,
	}
}

func TestJoinWelcomeAndDuplicateCharacter(t *testing.T) {
	w := testWorld(nil)
	out1 := make(chan []byte, 8)
	out2 := make(chan []byte, 8)
	out3 := make(chan []byte, 8)

	w.addPlayer(Join{SessionID: 1, Character: testCharacter(101, "Kaen", 1, 2), Out: out1})
	w.addPlayer(Join{SessionID: 2, Character: testCharacter(102, "Lyr", -3, 4), Out: out2})
	// El mismo personaje en otra sesión debe rechazarse.
	w.addPlayer(Join{SessionID: 3, Character: testCharacter(101, "Kaen", 0, 0), Out: out3})

	if len(w.players) != 2 {
		t.Fatalf("jugadores = %d, quería 2", len(w.players))
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
		if welcome.Self.ID != 101 || welcome.Character.Name != "Kaen" {
			t.Fatalf("welcome = %+v", welcome)
		}
		if welcome.Self.X != 1 || welcome.Self.Z != 2 {
			t.Fatalf("posición inicial en welcome = (%v, %v)", welcome.Self.X, welcome.Self.Z)
		}
	default:
		t.Fatal("el jugador 1 no recibió welcome")
	}

	select {
	case data := <-out3:
		env, err := protocol.Decode(data)
		if err != nil {
			t.Fatalf("Decode error: %v", err)
		}
		var e protocol.Error
		_ = json.Unmarshal(env.D, &e)
		if env.T != "err" || e.Code != "character_online" {
			t.Fatalf("esperaba err character_online, llegó %s %+v", env.T, e)
		}
	default:
		t.Fatal("el tercer cliente no recibió el error de personaje online")
	}
}

func TestMoveClampsAndReachesTarget(t *testing.T) {
	w := testWorld(nil)
	w.addPlayer(Join{SessionID: 1, Character: testCharacter(101, "Kaen", 0, 0), Out: make(chan []byte, 8)})
	p := w.players[1]

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
	if !p.dirty {
		t.Fatal("el personaje debería marcarse para guardar tras moverse")
	}
}

func TestEntitiesSortedAndLeaveSaves(t *testing.T) {
	saves := make(chan model.CharacterState, 8)
	w := testWorld(saves)
	w.addPlayer(Join{SessionID: 2, Character: testCharacter(102, "Lyr", 4, 5), Out: make(chan []byte, 4)})
	w.addPlayer(Join{SessionID: 1, Character: testCharacter(101, "Kaen", 1, 2), Out: make(chan []byte, 4)})

	ents := w.entities()
	if len(ents) != 2 {
		t.Fatalf("entidades = %d, quería 2", len(ents))
	}
	if ents[0].ID != 101 || ents[1].ID != 102 {
		t.Fatalf("orden = [%d, %d], quería [101, 102]", ents[0].ID, ents[1].ID)
	}

	Leave{SessionID: 1}.apply(w)
	if len(w.players) != 1 {
		t.Fatalf("jugadores = %d después de Leave, quería 1", len(w.players))
	}

	select {
	case state := <-saves:
		if state.CharacterID != 101 {
			t.Fatalf("personaje guardado = %d, quería 101", state.CharacterID)
		}
		if state.X != 1 || state.Z != 2 || state.MapID != MapID {
			t.Fatalf("estado guardado = %+v", state)
		}
	default:
		t.Fatal("no se encoló el guardado al salir")
	}
}

func TestMoveForUnknownPlayerIsIgnored(t *testing.T) {
	w := testWorld(nil)
	w.setTarget(Move{SessionID: 99, X: 1, Z: 1})
	if len(w.players) != 0 {
		t.Fatalf("jugadores = %d, quería 0", len(w.players))
	}
}
