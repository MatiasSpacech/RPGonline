package world

import (
	"encoding/json"
	"math"
	"testing"

	"munext/server/internal/gamedata"
	"munext/server/internal/model"
	"munext/server/internal/protocol"
)

func f5Skills() map[string]gamedata.Skill {
	return map[string]gamedata.Skill{
		"bola_fuego": {
			ID: "bola_fuego", Name: "Bola de Fuego", Class: "mage",
			LevelReq: 1, ManaCost: 6, CooldownMS: 3000,
			Kind: "single", Multiplier: 1.8, Range: 8.5,
		},
		"nova": {
			ID: "nova", Name: "Nova", Class: "mage",
			LevelReq: 8, ManaCost: 20, CooldownMS: 12000,
			Kind: "aoe", Multiplier: 1.2, Range: 8.5, Radius: 4.0, MaxTargets: 6,
		},
		"teletransporte": {
			ID: "teletransporte", Name: "Teleport", Class: "mage",
			LevelReq: 14, ManaCost: 15, CooldownMS: 20000,
			Kind: "teleport", TeleportDistance: 6,
		},
		"grito": {
			ID: "grito", Name: "Grito de Guerra", Class: "warrior",
			LevelReq: 10, ManaCost: 20, CooldownMS: 60000,
			Kind: "buff", BuffPercent: 15, BuffDurationMS: 30000,
		},
		"curar": {
			ID: "curar", Name: "Flecha Curativa", Class: "archer",
			LevelReq: 12, ManaCost: 15, CooldownMS: 15000,
			Kind: "heal", HealPercent: 25,
		},
	}
}

func f5MobSpec(id string, hp int) gamedata.Mob {
	return gamedata.Mob{
		ID: id, Name: id, Level: 1, HP: hp,
		DamageMin: 1, DamageMax: 2, Defense: 0, XP: 0,
		Speed: 1, AggroRadius: 0, AttackRange: 1.5,
		AttackIntervalMS: 2000, RespawnSeconds: 10, Passive: true,
	}
}

func f5Data(mobSpots []gamedata.Spawn) *gamedata.Data {
	return &gamedata.Data{
		Mobs:   map[string]gamedata.Mob{"dummy": f5MobSpec("dummy", 500)},
		Items:  testItemTemplates(),
		Skills: f5Skills(),
		Maps: map[string]gamedata.Map{
			"refugio": {
				ID: "refugio", Name: "Refugio", Theme: "town", Safe: true, LevelReq: 1,
				PlayerSpawn: gamedata.Vec2{},
				Portals: []gamedata.Portal{
					{ID: "to_field", Name: "Field", X: 2, Z: 0, Target: "llanuras", TargetX: 0, TargetZ: 6, LevelReq: 1},
					{ID: "to_dungeon", Name: "Dungeon", X: 3, Z: 0, Target: "llanuras", TargetX: 0, TargetZ: 6, LevelReq: 13},
					{ID: "far", Name: "Far", X: 10, Z: 10, Target: "llanuras", TargetX: 0, TargetZ: 6, LevelReq: 1},
				},
			},
			"llanuras": {
				ID: "llanuras", Name: "Llanuras", Theme: "field", Safe: false, LevelReq: 1,
				PlayerSpawn: gamedata.Vec2{}, Spawns: mobSpots,
			},
		},
	}
}

func hasErrorCode(msgs []protocol.Envelope, code string) bool {
	for _, env := range msgs {
		if env.T != "err" {
			continue
		}
		var e protocol.Error
		if json.Unmarshal(env.D, &e) == nil && e.Code == code {
			return true
		}
	}
	return false
}

func TestPortalMovesPlayerAndChecksLevel(t *testing.T) {
	data := f5Data([]gamedata.Spawn{{Mob: "dummy", X: 5, Z: 5}})
	out := make(chan []byte, 128)
	w := testWorld(nil, data)
	ch := testCharacter(101, "Mage", model.ClassMage, 0, 0)
	ch.MapID = "refugio"
	w.addPlayer(Join{SessionID: 1, Character: ch, Out: out})
	p := w.players[1]
	drainMessages(t, out)

	w.usePortal(UsePortal{SessionID: 1, PortalID: "to_field"})
	if p.MapID != "llanuras" {
		t.Fatalf("mapa = %q, quería llanuras", p.MapID)
	}
	if p.X != 0 || p.Z != 6 {
		t.Fatalf("posición tras portal = (%v, %v)", p.X, p.Z)
	}
	msgs := drainMessages(t, out)
	if _, ok := findMessage(msgs, "teleport"); !ok {
		t.Fatal("falta teleport")
	}

	// Nivel insuficiente para la mazmorra.
	p.MapID = "refugio"
	p.X, p.Z = 0, 0
	w.usePortal(UsePortal{SessionID: 1, PortalID: "to_dungeon"})
	if p.MapID != "refugio" {
		t.Fatalf("no debería haber entrado a la mazmorra a nivel 1 (mapa %q)", p.MapID)
	}
	if !hasErrorCode(drainMessages(t, out), "level_req") {
		t.Fatal("falta error level_req")
	}

	// Portal lejano.
	w.usePortal(UsePortal{SessionID: 1, PortalID: "far"})
	if !hasErrorCode(drainMessages(t, out), "too_far") {
		t.Fatal("falta error too_far")
	}
}

func TestSafeZoneBlocksAttackAndSkills(t *testing.T) {
	data := f5Data(nil)
	out := make(chan []byte, 64)
	w := testWorld(nil, data)
	ch := testCharacter(101, "Mage", model.ClassMage, 0, 0)
	ch.MapID = "refugio"
	w.addPlayer(Join{SessionID: 1, Character: ch, Out: out})
	drainMessages(t, out)

	w.setAttack(Attack{SessionID: 1, Target: 12345})
	w.useSkill(UseSkill{SessionID: 1, SkillID: "bola_fuego"})
	msgs := drainMessages(t, out)
	if !hasErrorCode(msgs, "safe_zone") {
		t.Fatal("faltan errores de zona segura")
	}
}

func TestSingleSkillDamageManaAndCooldown(t *testing.T) {
	data := f5Data([]gamedata.Spawn{{Mob: "dummy", X: 5, Z: 0}})
	out := make(chan []byte, 256)
	w := testWorld(nil, data)
	ch := testCharacter(101, "Mage", model.ClassMage, 0, 0)
	w.addPlayer(Join{SessionID: 1, Character: ch, Out: out})
	p := w.players[1]
	m := w.mobsByID[mobIDBase]
	drainMessages(t, out)
	startHP := m.HP
	startMP := p.MP

	w.useSkill(UseSkill{SessionID: 1, SkillID: "bola_fuego"})
	if m.HP >= startHP {
		t.Fatalf("el mob no recibió daño: %d -> %d", startHP, m.HP)
	}
	if p.MP != startMP-6 {
		t.Fatalf("maná = %d, quería %d", p.MP, startMP-6)
	}
	if p.Skills["bola_fuego"] <= w.tick {
		t.Fatal("el cooldown no se registró")
	}

	msgs := drainMessages(t, out)
	if _, ok := findMessage(msgs, "skill_cast"); !ok {
		t.Fatal("falta skill_cast")
	}
	if _, ok := findMessage(msgs, "skill_effect"); !ok {
		t.Fatal("falta skill_effect")
	}
	if _, ok := findMessage(msgs, "dmg"); !ok {
		t.Fatal("falta dmg")
	}

	// Cooldown: segundo uso rechazado y sin consumir maná.
	beforeMP := p.MP
	w.useSkill(UseSkill{SessionID: 1, SkillID: "bola_fuego"})
	if p.MP != beforeMP {
		t.Fatal("el segundo uso no debería consumir maná")
	}
	if !hasErrorCode(drainMessages(t, out), "cooldown") {
		t.Fatal("falta error cooldown")
	}
}

func TestAoeSkillHitsMultipleTargets(t *testing.T) {
	data := f5Data([]gamedata.Spawn{
		{Mob: "dummy", X: 4, Z: 0},
		{Mob: "dummy", X: 5, Z: 1},
		{Mob: "dummy", X: 6, Z: 0},
	})
	out := make(chan []byte, 256)
	w := testWorld(nil, data)
	ch := testCharacter(101, "Mage", model.ClassMage, 0, 0)
	w.addPlayer(Join{SessionID: 1, Character: ch, Out: out})
	p := w.players[1]
	p.Level = 8
	p.recalcMax()
	p.MP = p.MPMax
	drainMessages(t, out)

	var ids []int64
	for _, m := range w.mobs {
		ids = append(ids, m.ID)
	}
	w.useSkill(UseSkill{SessionID: 1, SkillID: "nova", TargetID: ids[0]})

	damaged := 0
	for _, m := range w.mobs {
		if m.HP < m.Spec.HP {
			damaged++
		}
	}
	if damaged != 3 {
		t.Fatalf("mobs dañados = %d, quería 3", damaged)
	}
	if p.MP != p.MPMax-20 {
		t.Fatalf("maná = %d, quería %d", p.MP, p.MPMax-20)
	}
}

func TestHealBuffAndTeleport(t *testing.T) {
	// Heal (arquera nivel 12).
	data := f5Data([]gamedata.Spawn{{Mob: "dummy", X: 5, Z: 0}})
	w := testWorld(nil, data)
	out := make(chan []byte, 128)
	ch := testCharacter(101, "Archer", model.ClassArcher, 0, 0)
	w.addPlayer(Join{SessionID: 1, Character: ch, Out: out})
	p := w.players[1]
	p.Level = 12
	p.recalcMax()
	p.HP = 10
	p.MP = p.MPMax
	drainMessages(t, out)
	w.useSkill(UseSkill{SessionID: 1, SkillID: "curar"})
	if p.HP <= 10 {
		t.Fatalf("el heal no curó: %d", p.HP)
	}

	// Buff (guerrero nivel 10).
	w2 := testWorld(nil, f5Data([]gamedata.Spawn{{Mob: "dummy", X: 5, Z: 0}}))
	out2 := make(chan []byte, 128)
	ch2 := testCharacter(102, "Warrior", model.ClassWarrior, 0, 0)
	w2.addPlayer(Join{SessionID: 1, Character: ch2, Out: out2})
	p2 := w2.players[1]
	p2.Level = 10
	p2.recalcMax()
	p2.MP = p2.MPMax
	drainMessages(t, out2)
	w2.useSkill(UseSkill{SessionID: 1, SkillID: "grito"})
	if p2.BuffPercent != 15 || math.Abs(w2.damageMultiplier(p2)-1.15) > 0.001 {
		t.Fatalf("buff = %d%% / mult %v", p2.BuffPercent, w2.damageMultiplier(p2))
	}

	// Teleport (mago nivel 14).
	w3 := testWorld(nil, f5Data(nil))
	out3 := make(chan []byte, 128)
	ch3 := testCharacter(103, "Mage2", model.ClassMage, 0, 0)
	w3.addPlayer(Join{SessionID: 1, Character: ch3, Out: out3})
	p3 := w3.players[1]
	p3.Level = 14
	p3.recalcMax()
	p3.MP = p3.MPMax
	drainMessages(t, out3)
	w3.useSkill(UseSkill{SessionID: 1, SkillID: "teletransporte", X: 100, Z: 0})
	if math.Abs(p3.X-6) > 0.001 || math.Abs(p3.Z) > 0.001 {
		t.Fatalf("posición tras teleport = (%v, %v), quería (6, 0)", p3.X, p3.Z)
	}
}

func TestBossSummonAndEnrage(t *testing.T) {
	boss := f5MobSpec("boss", 1000)
	boss.XP = 10
	boss.SummonAtPercent = 50
	boss.SummonMob = "add"
	boss.SummonCount = 2
	boss.EnrageAtPercent = 30
	boss.EnrageSpeedMult = 1.4
	boss.EnrageDmgMult = 1.3
	add := f5MobSpec("add", 50)

	data := &gamedata.Data{
		Mobs: map[string]gamedata.Mob{"boss": boss, "add": add},
		Maps: map[string]gamedata.Map{
			"llanuras": {
				ID: "llanuras", Name: "Field", Theme: "field", LevelReq: 1,
				Spawns: []gamedata.Spawn{{Mob: "boss", X: 2, Z: 0}},
			},
		},
	}
	out := make(chan []byte, 128)
	w := testWorld(nil, data)
	ch := testCharacter(101, "Warrior", model.ClassWarrior, 0, 0)
	w.addPlayer(Join{SessionID: 1, Character: ch, Out: out})
	p := w.players[1]
	m := w.mobsByID[mobIDBase]
	drainMessages(t, out)

	m.HP = 501
	w.playerAttack(p, m)
	if !m.Summoned || len(w.mobs) != 3 {
		t.Fatalf("summon: summoned=%v mobs=%d", m.Summoned, len(w.mobs))
	}
	for _, addMob := range w.mobs {
		if addMob.ID != m.ID && (!addMob.Temporary || addMob.MapID != "llanuras") {
			t.Fatalf("esbirro inválido = %+v", addMob)
		}
	}

	m.HP = 301
	w.playerAttack(p, m)
	if !m.Enraged {
		t.Fatal("el boss no se enfureció al 30%")
	}
}

func TestSnapshotsAreFilteredByMap(t *testing.T) {
	data := f5Data([]gamedata.Spawn{{Mob: "dummy", X: 5, Z: 0}})
	w := testWorld(nil, data)
	ch1 := testCharacter(101, "A", model.ClassWarrior, 0, 0) // llanuras
	ch2 := testCharacter(102, "B", model.ClassMage, 0, 0)
	ch2.MapID = "refugio"
	out1 := make(chan []byte, 64)
	w.addPlayer(Join{SessionID: 1, Character: ch1, Out: out1})
	w.addPlayer(Join{SessionID: 2, Character: ch2, Out: make(chan []byte, 64)})

	field := w.entitiesInMap("llanuras")
	town := w.entitiesInMap("refugio")
	if len(field) != 2 { // jugador A + dummy
		t.Fatalf("entidades en field = %d, quería 2", len(field))
	}
	if len(town) != 1 { // solo jugador B
		t.Fatalf("entidades en town = %d, quería 1", len(town))
	}
}
