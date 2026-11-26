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
	"munext/server/internal/gamedata"
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

	// Un mob pasivo cerca del spawn: muere de un golpe y da 60 XP (nivel 1→2).
	testData := &gamedata.Data{
		MapID: "llanuras",
		Mobs: map[string]gamedata.Mob{
			"dummy": {
				ID: "dummy", Name: "Dummy", Level: 1, HP: 20,
				DamageMin: 1, DamageMax: 2, Defense: 0, XP: 60,
				Speed: 1, AggroRadius: 0, AttackRange: 1.5,
				AttackIntervalMS: 1000, RespawnSeconds: 10, Passive: true,
			},
		},
		Items: map[string]gamedata.ItemTemplate{
			"sword_iron": {
				ID: "sword_iron", Name: "Espada de hierro", Kind: "weapon", Slot: "weapon",
				Class: model.ClassWarrior, ReqStr: 25, MinDamage: 8, MaxDamage: 14,
				IntervalMS: 1100, Range: 1.8, Value: 200,
			},
			"pot_hp": {
				ID: "pot_hp", Name: "Poción de vida", Kind: "potion", HealKind: "hp",
				HealPercent: 40, Stackable: true, MaxStack: 20, Value: 10, BuyPrice: 50,
			},
			"pot_mp": {
				ID: "pot_mp", Name: "Poción de maná", Kind: "potion", HealKind: "mp",
				HealPercent: 40, Stackable: true, MaxStack: 20, Value: 10, BuyPrice: 50,
			},
			"core_enhance": {
				ID: "core_enhance", Name: "Núcleo de mejora", Kind: "core",
				Stackable: true, MaxStack: 20, Value: 120, BuyPrice: 600,
			},
		},
		Spawns: []gamedata.Spawn{{Mob: "dummy", X: 1.5, Z: 0}},
	}

	gameWorld := world.New(log, saveCh, testData)
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

	// 3.5) Kit inicial, equipar y mejorar el arma.
	env = readEnvelope(t, conn, "inventory", 3*time.Second)
	var inv protocol.Inventory
	if err := json.Unmarshal(env.D, &inv); err != nil {
		t.Fatalf("inventory: %v", err)
	}
	if len(inv.Items) != 4 || inv.Gold != 500 {
		t.Fatalf("kit inicial = %d ítems, %d oro", len(inv.Items), inv.Gold)
	}
	var weaponID int64
	for _, it := range inv.Items {
		if it.Tmpl == "sword_iron" {
			weaponID = it.ID
		}
	}
	if weaponID == 0 {
		t.Fatal("el kit inicial no trae espada")
	}

	send(t, conn, "equip", protocol.Equip{ItemID: weaponID})
	env = readEnvelope(t, conn, "inventory", 3*time.Second)
	if err := json.Unmarshal(env.D, &inv); err != nil {
		t.Fatalf("inventory tras equipar: %v", err)
	}
	weaponEquipped := false
	for _, it := range inv.Items {
		if it.ID == weaponID && it.EquippedSlot == "weapon" {
			weaponEquipped = true
		}
	}
	if !weaponEquipped {
		t.Fatal("la espada no quedó equipada")
	}

	send(t, conn, "enhance", protocol.Enhance{ItemID: weaponID})
	env = readEnvelope(t, conn, "enhance_result", 3*time.Second)
	var enhanceResult protocol.EnhanceResult
	if err := json.Unmarshal(env.D, &enhanceResult); err != nil {
		t.Fatalf("enhance_result: %v", err)
	}
	env = readEnvelope(t, conn, "inventory", 3*time.Second)
	if err := json.Unmarshal(env.D, &inv); err != nil {
		t.Fatalf("inventory tras mejorar: %v", err)
	}
	if inv.Gold != 450 {
		t.Fatalf("oro tras mejorar = %d, quería 450", inv.Gold)
	}
	coreLeft := false
	for _, it := range inv.Items {
		if it.Tmpl == "core_enhance" {
			coreLeft = true
		}
		if it.ID == weaponID && it.Enhance != enhanceResult.NewEnhance {
			t.Fatalf("enhance del arma = %d, resultado = %d", it.Enhance, enhanceResult.NewEnhance)
		}
	}
	if coreLeft {
		t.Fatal("el núcleo debería haberse consumido")
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

	// 4) Atacar un mob: debe morir, dar XP y subir a nivel 2.
	send(t, conn, "attack", protocol.Attack{Target: 1_000_000})
	deadline = time.Now().Add(6 * time.Second)
	leveledUp := false
	for time.Now().Before(deadline) && !leveledUp {
		_ = conn.SetReadDeadline(deadline)
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("esperando level up: %v", err)
		}
		msg, err := protocol.Decode(data)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if msg.T != "self_state" {
			continue
		}
		var state protocol.SelfState
		if err := json.Unmarshal(msg.D, &state); err != nil {
			t.Fatalf("self_state: %v", err)
		}
		if state.Level == 2 {
			leveledUp = true
			if state.StatPoints != 5 {
				t.Fatalf("stat points tras level up = %d, quería 5", state.StatPoints)
			}
		}
	}
	if !leveledUp {
		t.Fatal("el personaje no subió a nivel 2 tras matar el mob")
	}

	// 5) Desconectar: el mundo debe guardar posición + progreso.
	_ = conn.Close()

	var saved model.Character
	deadline = time.Now().Add(5 * time.Second)
	for {
		saved, err = store.LoadCharacter(ctx, user.ID, charID)
		if err != nil {
			t.Fatalf("LoadCharacter: %v", err)
		}
		if saved.Level == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("el progreso no se guardó a tiempo (level=%d)", saved.Level)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if saved.StatPoints != 5 {
		t.Fatalf("stat points guardados = %d, quería 5", saved.StatPoints)
	}
	if saved.Gold != 450 {
		t.Fatalf("oro guardado = %d, quería 450", saved.Gold)
	}
	var savedWeapon model.Item
	coreSaved := false
	for _, it := range saved.Items {
		if it.ID == weaponID {
			savedWeapon = it
		}
		if it.Tmpl == "core_enhance" {
			coreSaved = true
		}
	}
	if savedWeapon.EquippedSlot != "weapon" || savedWeapon.Enhance != enhanceResult.NewEnhance {
		t.Fatalf("arma guardada = %+v (resultado %+v)", savedWeapon, enhanceResult)
	}
	if coreSaved {
		t.Fatal("el núcleo no debería persistir tras la mejora")
	}

	// 6) Reconectar + login + seleccionar: nivel, posición e inventario se restauran.
	conn2 := dial(t, ts)
	send(t, conn2, "login", protocol.Auth{User: "matias", Pass: "secreto123"})
	readEnvelope(t, conn2, "login_ok", 3*time.Second)
	send(t, conn2, "select_char", protocol.SelectChar{CharID: charID})
	env = readEnvelope(t, conn2, "welcome", 3*time.Second)
	var welcome2 protocol.Welcome
	if err := json.Unmarshal(env.D, &welcome2); err != nil {
		t.Fatalf("welcome2: %v", err)
	}

	if welcome2.Character.Level != 2 {
		t.Fatalf("nivel restaurado = %d, quería 2", welcome2.Character.Level)
	}
	if math.Abs(welcome2.Self.X-saved.X) > 0.001 || math.Abs(welcome2.Self.Z-saved.Z) > 0.001 {
		t.Fatalf("posición al reingresar = (%v, %v), guardada = (%v, %v)",
			welcome2.Self.X, welcome2.Self.Z, saved.X, saved.Z)
	}
	if welcome2.Self.X <= 1.0 {
		t.Fatalf("la posición restaurada no refleja el movimiento: x=%v", welcome2.Self.X)
	}

	env = readEnvelope(t, conn2, "inventory", 3*time.Second)
	var inv2 protocol.Inventory
	if err := json.Unmarshal(env.D, &inv2); err != nil {
		t.Fatalf("inventory2: %v", err)
	}
	weaponRestored := false
	for _, it := range inv2.Items {
		if it.ID == weaponID && it.EquippedSlot == "weapon" && it.Enhance == enhanceResult.NewEnhance {
			weaponRestored = true
		}
	}
	if !weaponRestored || inv2.Gold != 450 {
		t.Fatalf("inventario restaurado = %+v (oro %d)", inv2.Items, inv2.Gold)
	}
}
