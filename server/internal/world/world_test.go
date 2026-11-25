package world

import (
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"testing"

	"munext/server/internal/gamedata"
	"munext/server/internal/model"
	"munext/server/internal/protocol"
)

func testWorld(saveCh chan model.CharacterState, data *gamedata.Data) *World {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(log, saveCh, data)
}

func testCharacter(id int64, name string, class string, x, z float64) model.Character {
	def := model.Classes[class]
	return model.Character{
		ID:    id,
		Name:  name,
		Class: class,
		Level: 1,
		Str:   def.Str,
		Agi:   def.Agi,
		Vit:   def.Vit,
		Ene:   def.Ene,
		MapID: DefaultMapID,
		X:     x,
		Z:     z,
	}
}

func testMobData(spec gamedata.Mob, x, z float64) *gamedata.Data {
	return &gamedata.Data{
		MapID: DefaultMapID,
		Mobs:  map[string]gamedata.Mob{spec.ID: spec},
		Spawns: []gamedata.Spawn{
			{Mob: spec.ID, X: x, Z: z},
		},
	}
}

func testMobSpec(id string, hp int, xp int64) gamedata.Mob {
	return gamedata.Mob{
		ID:               id,
		Name:             "Mob de prueba",
		Level:            1,
		HP:               hp,
		DamageMin:        1,
		DamageMax:        2,
		Defense:          0,
		XP:               xp,
		Speed:            2,
		AggroRadius:      0,
		AttackRange:      1.5,
		AttackIntervalMS: 1000,
		RespawnSeconds:   1,
		Passive:          true,
	}
}

func drainMessages(t *testing.T, out chan []byte) []protocol.Envelope {
	t.Helper()
	var msgs []protocol.Envelope
	for {
		select {
		case data := <-out:
			env, err := protocol.Decode(data)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			msgs = append(msgs, env)
		default:
			return msgs
		}
	}
}

func findMessage(msgs []protocol.Envelope, mtype string) (protocol.Envelope, bool) {
	for _, m := range msgs {
		if m.T == mtype {
			return m, true
		}
	}
	return protocol.Envelope{}, false
}

func TestJoinWelcomeSelfStateAndDuplicateCharacter(t *testing.T) {
	w := testWorld(nil, nil)
	out1 := make(chan []byte, 16)
	out2 := make(chan []byte, 16)
	out3 := make(chan []byte, 16)

	w.addPlayer(Join{SessionID: 1, Character: testCharacter(101, "Kaen", model.ClassWarrior, 1, 2), Out: out1})
	w.addPlayer(Join{SessionID: 2, Character: testCharacter(102, "Lyr", model.ClassMage, -3, 4), Out: out2})
	// El mismo personaje en otra sesión debe rechazarse.
	w.addPlayer(Join{SessionID: 3, Character: testCharacter(101, "Kaen", model.ClassWarrior, 0, 0), Out: out3})

	if len(w.players) != 2 {
		t.Fatalf("jugadores = %d, quería 2", len(w.players))
	}

	msgs := drainMessages(t, out1)
	welcomeEnv, ok := findMessage(msgs, "welcome")
	if !ok {
		t.Fatal("el jugador 1 no recibió welcome")
	}
	var welcome protocol.Welcome
	if err := json.Unmarshal(welcomeEnv.D, &welcome); err != nil {
		t.Fatalf("welcome: %v", err)
	}
	if welcome.Self.ID != 101 || welcome.Character.Name != "Kaen" {
		t.Fatalf("welcome = %+v", welcome)
	}
	if welcome.Self.X != 1 || welcome.Self.Z != 2 {
		t.Fatalf("posición inicial = (%v, %v)", welcome.Self.X, welcome.Self.Z)
	}

	stateEnv, ok := findMessage(msgs, "self_state")
	if !ok {
		t.Fatal("el jugador 1 no recibió self_state")
	}
	var state protocol.SelfState
	if err := json.Unmarshal(stateEnv.D, &state); err != nil {
		t.Fatalf("self_state: %v", err)
	}
	if state.HP != state.HPMax || state.HP != 73 {
		t.Fatalf("HP inicial = %d/%d, quería 73/73", state.HP, state.HPMax)
	}
	if state.HPPotions != startingPotions || state.Level != 1 {
		t.Fatalf("estado inicial = %+v", state)
	}

	errEnv, ok := findMessage(drainMessages(t, out3), "err")
	if !ok {
		t.Fatal("el tercer cliente no recibió error")
	}
	var e protocol.Error
	_ = json.Unmarshal(errEnv.D, &e)
	if e.Code != "character_online" {
		t.Fatalf("código = %q, quería character_online", e.Code)
	}
}

func TestMoveClampsAndReachesTarget(t *testing.T) {
	w := testWorld(nil, nil)
	w.addPlayer(Join{SessionID: 1, Character: testCharacter(101, "Kaen", model.ClassWarrior, 0, 0), Out: make(chan []byte, 8)})
	p := w.players[1]

	w.setTarget(Move{SessionID: 1, X: 100, Z: 0})
	if p.TargetX != MapHalfSize {
		t.Fatalf("target.x = %v, quería %v", p.TargetX, MapHalfSize)
	}
	if !p.HasTarget {
		t.Fatal("el jugador debería tener destino")
	}

	for i := 0; i < TickRate*4; i++ {
		w.tick++
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

func TestAttackKillsMobLevelsUpAndSaves(t *testing.T) {
	saves := make(chan model.CharacterState, 32)
	spec := testMobSpec("dummy", 20, 60) // muere de un golpe y da el nivel 1→2
	w := testWorld(saves, testMobData(spec, 1.5, 0))

	out := make(chan []byte, 256)
	w.addPlayer(Join{SessionID: 1, Character: testCharacter(101, "Kaen", model.ClassWarrior, 0, 0), Out: out})
	p := w.players[1]
	m := w.mobsByID[mobIDBase]

	w.setAttack(Attack{SessionID: 1, Target: m.ID})
	if p.AttackTarget != m.ID {
		t.Fatalf("attack target = %d, quería %d", p.AttackTarget, m.ID)
	}

	for i := 0; i < 20 && m.State != "dead"; i++ {
		w.tick++
		w.simulate(1.0 / TickRate)
	}

	if m.State != "dead" {
		t.Fatalf("mob state = %q, quería dead (hp=%d)", m.State, m.HP)
	}
	if p.Level != 2 {
		t.Fatalf("nivel = %d, quería 2 (xp=%d)", p.Level, p.XP)
	}
	if p.XP != 0 {
		t.Fatalf("xp = %d, quería 0 tras level up", p.XP)
	}
	if p.StatPts != 5 {
		t.Fatalf("stat points = %d, quería 5", p.StatPts)
	}
	if p.HP != p.HPMax || p.MP != p.MPMax {
		t.Fatalf("level up debería curar: HP=%d/%d MP=%d/%d", p.HP, p.HPMax, p.MP, p.MPMax)
	}

	msgs := drainMessages(t, out)
	if _, ok := findMessage(msgs, "mob_dead"); !ok {
		t.Fatal("falta el evento mob_dead")
	}
	if _, ok := findMessage(msgs, "levelup"); !ok {
		t.Fatal("falta el evento levelup")
	}

	select {
	case state := <-saves:
		if state.Level != 2 || state.StatPoints != 5 || state.CharacterID != 101 {
			t.Fatalf("estado guardado = %+v", state)
		}
	default:
		t.Fatal("no se encoló el guardado tras subir de nivel")
	}
}

func TestMobRespawnsAfterTimer(t *testing.T) {
	spec := testMobSpec("dummy", 20, 10)
	w := testWorld(nil, testMobData(spec, 1.5, 0))
	out := make(chan []byte, 64)
	w.addPlayer(Join{SessionID: 1, Character: testCharacter(101, "Kaen", model.ClassWarrior, 0, 0), Out: out})
	m := w.mobsByID[mobIDBase]

	m.HP = 1
	w.playerAttack(w.players[1], m)
	if m.State != "dead" {
		t.Fatalf("state = %q, quería dead", m.State)
	}

	// Dead no aparece en snapshots.
	for _, ent := range w.entities() {
		if ent.ID == m.ID {
			t.Fatal("un mob muerto no debe aparecer en entities()")
		}
	}

	for i := 0; i < TickRate*spec.RespawnSeconds+2; i++ {
		w.tick++
		w.simulate(1.0 / TickRate)
	}

	if m.State != "idle" || m.HP != spec.HP {
		t.Fatalf("mob tras respawn = state %q, hp %d", m.State, m.HP)
	}
	if m.X != m.SpawnX || m.Z != m.SpawnZ {
		t.Fatalf("mob respawneó en (%v, %v), quería (%v, %v)", m.X, m.Z, m.SpawnX, m.SpawnZ)
	}
}

func TestAggressiveMobKillsPlayerAndRespawnsWithPenalty(t *testing.T) {
	saves := make(chan model.CharacterState, 256)
	spec := gamedata.Mob{
		ID: "asesino", Name: "Asesino", Level: 1, HP: 1000,
		DamageMin: 1000, DamageMax: 1000, Defense: 0, XP: 0,
		Speed: 2, AggroRadius: 10, AttackRange: 2,
		AttackIntervalMS: 20, RespawnSeconds: 10, Passive: false,
	}
	w := testWorld(saves, testMobData(spec, 1, 0))
	out := make(chan []byte, 512)
	w.addPlayer(Join{SessionID: 1, Character: testCharacter(101, "Kaen", model.ClassWarrior, 0, 0), Out: out})
	p := w.players[1]
	p.XP = 30 // para verificar la penalidad del 3%

	// Simular hasta la PRIMERA muerte (el XP baja de 30 a 28 y cortamos).
	for i := 0; i < 200; i++ {
		w.tick++
		w.simulate(1.0 / TickRate)
		if p.XP != 30 {
			break
		}
	}

	if p.Level != 1 {
		t.Fatalf("no debe perder nivel: %d", p.Level)
	}
	if p.XP != 28 { // 30 - round(60*0.03)=2
		t.Fatalf("xp tras morir = %d, quería 28", p.XP)
	}
	if p.HP != p.HPMax/2 {
		t.Fatalf("hp tras respawn = %d, quería %d", p.HP, p.HPMax/2)
	}
	if p.HPPotions != startingPotions {
		t.Fatalf("pociones tras respawn = %d", p.HPPotions)
	}

	msgs := drainMessages(t, out)
	if _, ok := findMessage(msgs, "death"); !ok {
		t.Fatal("falta el evento death")
	}
	if _, ok := findMessage(msgs, "teleport"); !ok {
		t.Fatal("falta el evento teleport")
	}

	select {
	case state := <-saves:
		if state.CharacterID != 101 || state.XP != 28 {
			t.Fatalf("guardado tras muerte = %+v", state)
		}
	default:
		t.Fatal("no se encoló el guardado tras morir")
	}
}

func TestPotionAndAllocStat(t *testing.T) {
	saves := make(chan model.CharacterState, 16)
	w := testWorld(saves, nil)
	out := make(chan []byte, 128)
	w.addPlayer(Join{SessionID: 1, Character: testCharacter(101, "Kaen", model.ClassWarrior, 0, 0), Out: out})
	p := w.players[1]

	p.HP = 10
	w.tick = 100
	w.usePotion(Potion{SessionID: 1, Kind: "hp"})
	if p.HP != 39 { // 10 + 40% de 73 (29)
		t.Fatalf("hp tras poción = %d, quería 39", p.HP)
	}
	if p.HPPotions != startingPotions-1 {
		t.Fatalf("pociones = %d, quería %d", p.HPPotions, startingPotions-1)
	}

	// Cooldown: el segundo uso inmediato se ignora.
	w.usePotion(Potion{SessionID: 1, Kind: "hp"})
	if p.HPPotions != startingPotions-1 {
		t.Fatalf("el cooldown no se respetó: %d", p.HPPotions)
	}

	oldHPMax := p.HPMax
	p.StatPts = 1
	w.allocStat(AllocStat{SessionID: 1, Stat: "vit"})
	if p.Vit != 26 || p.StatPts != 0 {
		t.Fatalf("alloc vit = Vit %d, puntos %d", p.Vit, p.StatPts)
	}
	if p.HPMax != oldHPMax+2 {
		t.Fatalf("HPMax = %d, quería %d", p.HPMax, oldHPMax+2)
	}

	select {
	case state := <-saves:
		if state.Vit != 26 || state.StatPoints != 0 {
			t.Fatalf("guardado tras alloc = %+v", state)
		}
	default:
		t.Fatal("no se encoló el guardado tras asignar stat")
	}
}

func TestEntitiesSortedAndLeaveSaves(t *testing.T) {
	saves := make(chan model.CharacterState, 8)
	w := testWorld(saves, nil)
	w.addPlayer(Join{SessionID: 2, Character: testCharacter(102, "Lyr", model.ClassMage, 4, 5), Out: make(chan []byte, 16)})
	w.addPlayer(Join{SessionID: 1, Character: testCharacter(101, "Kaen", model.ClassWarrior, 1, 2), Out: make(chan []byte, 16)})

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
		if state.CharacterID != 101 || state.X != 1 || state.Z != 2 || state.MapID != DefaultMapID {
			t.Fatalf("estado guardado = %+v", state)
		}
	default:
		t.Fatal("no se encoló el guardado al salir")
	}
}

func TestMoveForUnknownPlayerIsIgnored(t *testing.T) {
	w := testWorld(nil, nil)
	w.setTarget(Move{SessionID: 99, X: 1, Z: 1})
	if len(w.players) != 0 {
		t.Fatalf("jugadores = %d, quería 0", len(w.players))
	}
}
