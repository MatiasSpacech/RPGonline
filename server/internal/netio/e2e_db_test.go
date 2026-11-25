package netio

// Test de aceptación de F2 con base de datos real: registro, creación de
// personaje, movimiento, desconexión (guardado) y reingreso en la posición
// guardada. Requiere MUNEXT_TEST_DATABASE_URL (ver README / Makefile).

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"munext/server/internal/accounts"
	"munext/server/internal/db"
	"munext/server/internal/model"
	"munext/server/internal/protocol"
	"munext/server/internal/world"
)

func TestPersistenceAcrossReconnect(t *testing.T) {
	dsn := os.Getenv("MUNEXT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MUNEXT_TEST_DATABASE_URL no definida: se omite el test de persistencia")
	}

	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := db.Migrate(ctx, dsn, log); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("limpiando BD: %v", err)
	}

	store := db.NewStore(pool)
	accountsSvc := accounts.New(store)

	saveCh := make(chan model.CharacterState, 64)
	go db.RunSaver(saveCh, store, log)

	gameWorld := world.New(log, saveCh)
	worldCtx, cancelWorld := context.WithCancel(context.Background())
	t.Cleanup(cancelWorld)
	go gameWorld.Run(worldCtx)

	s := New(log, gameWorld.Inbox(), accountsSvc)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	// 1) Registrar cuenta (auto-login).
	conn := dial(t, ts)
	send(t, conn, "register", protocol.Auth{User: "matias", Pass: "secreto123"})
	env := readEnvelope(t, conn, "login_ok", 3*time.Second)
	var loginOK protocol.LoginOK
	if err := json.Unmarshal(env.D, &loginOK); err != nil {
		t.Fatalf("login_ok: %v", err)
	}
	if len(loginOK.Chars) != 0 {
		t.Fatalf("cuenta nueva con %d personajes, quería 0", len(loginOK.Chars))
	}

	user, _, err := store.GetUserByUsername(ctx, "matias")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}

	// 2) Crear personaje.
	send(t, conn, "create_char", protocol.CreateChar{Name: "Kaen", Class: model.ClassWarrior})
	env = readEnvelope(t, conn, "char_created", 3*time.Second)
	var created protocol.CharacterCreated
	if err := json.Unmarshal(env.D, &created); err != nil {
		t.Fatalf("char_created: %v", err)
	}
	charID := created.Char.ID

	// 3) Entrar al mundo y moverse.
	send(t, conn, "select_char", protocol.SelectChar{CharID: charID})
	env = readEnvelope(t, conn, "welcome", 3*time.Second)
	var welcome protocol.Welcome
	if err := json.Unmarshal(env.D, &welcome); err != nil {
		t.Fatalf("welcome: %v", err)
	}
	if welcome.Character.Name != "Kaen" {
		t.Fatalf("welcome.character = %+v", welcome.Character)
	}
	if welcome.Self.X != 0 || welcome.Self.Z != 0 {
		t.Fatalf("posición inicial = (%v, %v), quería (0, 0)", welcome.Self.X, welcome.Self.Z)
	}

	send(t, conn, "move_req", protocol.MoveReq{X: 8, Z: -3})

	// Esperar un snapshot que confirme el movimiento.
	deadline := time.Now().Add(4 * time.Second)
	moved := false
	for time.Now().Before(deadline) && !moved {
		_ = conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("esperando snapshot: %v", err)
		}
		msg, err := protocol.Decode(data)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if msg.T != "snapshot" {
			continue
		}
		var snap protocol.Snapshot
		if err := json.Unmarshal(msg.D, &snap); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		for _, ent := range snap.Ents {
			if ent.ID == charID && ent.X > 1.0 {
				moved = true
				break
			}
		}
	}
	if !moved {
		t.Fatal("el personaje no se movió en el servidor")
	}

	// 4) Desconectar: el mundo debe guardar la posición final.
	_ = conn.Close()

	var saved model.Character
	deadline = time.Now().Add(5 * time.Second)
	for {
		saved, err = store.LoadCharacter(ctx, user.ID, charID)
		if err != nil {
			t.Fatalf("LoadCharacter: %v", err)
		}
		if saved.X > 1.0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("la posición no se guardó a tiempo (x=%v)", saved.X)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 5) Reconectar + login + seleccionar: debe reaparecer donde quedó.
	conn2 := dial(t, ts)
	send(t, conn2, "login", protocol.Auth{User: "matias", Pass: "secreto123"})
	readEnvelope(t, conn2, "login_ok", 3*time.Second)
	send(t, conn2, "select_char", protocol.SelectChar{CharID: charID})
	env = readEnvelope(t, conn2, "welcome", 3*time.Second)
	var welcome2 protocol.Welcome
	if err := json.Unmarshal(env.D, &welcome2); err != nil {
		t.Fatalf("welcome2: %v", err)
	}

	if math.Abs(welcome2.Self.X-saved.X) > 0.001 || math.Abs(welcome2.Self.Z-saved.Z) > 0.001 {
		t.Fatalf("posición al reingresar = (%v, %v), guardada = (%v, %v)",
			welcome2.Self.X, welcome2.Self.Z, saved.X, saved.Z)
	}
	if welcome2.Self.X <= 1.0 {
		t.Fatalf("la posición restaurada no refleja el movimiento: x=%v", welcome2.Self.X)
	}
}
