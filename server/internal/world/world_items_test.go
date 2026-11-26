package world

import (
	"encoding/json"
	"testing"

	"munext/server/internal/gamedata"
	"munext/server/internal/model"
	"munext/server/internal/protocol"
)

func testItemTemplates() map[string]gamedata.ItemTemplate {
	return map[string]gamedata.ItemTemplate{
		"sword_iron": {
			ID: "sword_iron", Name: "Espada de hierro", Kind: "weapon", Slot: "weapon",
			Class: model.ClassWarrior, ReqStr: 25, MinDamage: 8, MaxDamage: 14,
			IntervalMS: 1100, Range: 1.8, Value: 200,
		},
		"helm": {
			ID: "helm", Name: "Casco", Kind: "armor", Slot: "helm", Defense: 5, Value: 100,
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
	}
}

func testItemData() *gamedata.Data {
	return &gamedata.Data{
		MapID: DefaultMapID,
		Mobs:  map[string]gamedata.Mob{},
		Items: testItemTemplates(),
	}
}

func characterWithItems(items []model.Item, gold int64) model.Character {
	c := testCharacter(101, "Kaen", model.ClassWarrior, 0, 0)
	c.Items = items
	c.Gold = gold
	return c
}

func TestInventoryWelcomeAndDerivedStats(t *testing.T) {
	w := testWorld(nil, testItemData())
	out := make(chan []byte, 64)
	w.addPlayer(Join{
		SessionID: 1,
		Character: characterWithItems([]model.Item{
			{ID: 1, CharacterID: 101, Tmpl: "sword_iron", Rarity: "good", Enhance: 2, EquippedSlot: "weapon", InvX: -1, InvY: -1, Qty: 1},
			{ID: 2, CharacterID: 101, Tmpl: "pot_hp", Rarity: "common", InvX: 0, InvY: 0, Qty: 5},
		}, 0),
		Out: out,
	})

	msgs := drainMessages(t, out)
	welcomeEnv, ok := findMessage(msgs, "welcome")
	if !ok {
		t.Fatal("falta welcome")
	}
	var welcome protocol.Welcome
	if err := json.Unmarshal(welcomeEnv.D, &welcome); err != nil {
		t.Fatalf("welcome: %v", err)
	}
	if len(welcome.GameData.Items) != len(testItemTemplates()) {
		t.Fatalf("game_data.items = %d", len(welcome.GameData.Items))
	}

	invEnv, ok := findMessage(msgs, "inventory")
	if !ok {
		t.Fatal("falta inventory")
	}
	var inv protocol.Inventory
	if err := json.Unmarshal(invEnv.D, &inv); err != nil {
		t.Fatalf("inventory: %v", err)
	}
	if len(inv.Items) != 2 {
		t.Fatalf("inventario = %d ítems", len(inv.Items))
	}

	stateEnv, ok := findMessage(msgs, "self_state")
	if !ok {
		t.Fatal("falta self_state")
	}
	var state protocol.SelfState
	if err := json.Unmarshal(stateEnv.D, &state); err != nil {
		t.Fatalf("self_state: %v", err)
	}
	// Espada buena +2: min 8*1.15*1.16=10.672→11 + 16.8 = 27.8→28
	// max 14*1.15*1.16=18.676→19 + 16.8 = 35.8→36
	if state.AttackMin != 28 || state.AttackMax != 36 {
		t.Fatalf("ataque derivado = %d-%d, quería 28-36", state.AttackMin, state.AttackMax)
	}
	if state.HPPotions != 5 {
		t.Fatalf("pociones = %d, quería 5", state.HPPotions)
	}
	if state.Defense != 8 {
		t.Fatalf("defensa = %d, quería 8", state.Defense)
	}
}

func TestEquipUnequipChangesDamage(t *testing.T) {
	w := testWorld(nil, testItemData())
	out := make(chan []byte, 128)
	w.addPlayer(Join{
		SessionID: 1,
		Character: characterWithItems([]model.Item{
			{ID: 1, CharacterID: 101, Tmpl: "sword_iron", Rarity: "common", InvX: 0, InvY: 0, Qty: 1},
		}, 0),
		Out: out,
	})
	p := w.players[1]
	drainMessages(t, out)

	if minD, _ := w.damageRange(p); minD != 21 {
		t.Fatalf("daño sin arma = %d, quería 21", minD)
	}

	w.equipItem(Equip{SessionID: 1, ItemID: 1})
	if p.Items[0].EquippedSlot != "weapon" {
		t.Fatalf("el arma no se equipó: %+v", p.Items[0])
	}
	if minD, maxD := w.damageRange(p); minD != 25 || maxD != 31 {
		t.Fatalf("daño con espada = %d-%d, quería 25-31", minD, maxD)
	}

	w.unequipItem(Unequip{SessionID: 1, Slot: "weapon"})
	if p.Items[0].EquippedSlot != "" {
		t.Fatalf("el arma sigue equipada: %+v", p.Items[0])
	}
	if p.Items[0].InvX < 0 || p.Items[0].InvX >= inventoryWidth {
		t.Fatalf("posición inválida al desequipar: %+v", p.Items[0])
	}
}

func TestEnhanceConsumesCoreAndGold(t *testing.T) {
	saves := make(chan model.CharacterState, 32)
	w := testWorld(saves, testItemData())
	out := make(chan []byte, 256)
	w.addPlayer(Join{
		SessionID: 1,
		Character: characterWithItems([]model.Item{
			{ID: 10, CharacterID: 101, Tmpl: "sword_iron", Rarity: "common", InvX: 0, InvY: 0, Qty: 1},
			{ID: 11, CharacterID: 101, Tmpl: "core_enhance", Rarity: "common", InvX: 1, InvY: 0, Qty: 1},
		}, 500),
		Out: out,
	})
	p := w.players[1]
	drainMessages(t, out)

	w.enhanceItem(Enhance{SessionID: 1, ItemID: 10})

	msgs := drainMessages(t, out)
	env, ok := findMessage(msgs, "enhance_result")
	if !ok {
		t.Fatal("falta enhance_result")
	}
	var result protocol.EnhanceResult
	if err := json.Unmarshal(env.D, &result); err != nil {
		t.Fatalf("enhance_result: %v", err)
	}
	if result.Destroyed {
		t.Fatal("un ítem +0 no puede destruirse")
	}
	if p.Gold != 450 {
		t.Fatalf("oro = %d, quería 450", p.Gold)
	}
	coreFound := false
	weapon := model.Item{}
	for _, it := range p.Items {
		if it.Tmpl == "core_enhance" {
			coreFound = true
		}
		if it.ID == 10 {
			weapon = it
		}
	}
	if coreFound {
		t.Fatal("el núcleo debería haberse consumido")
	}
	if weapon.Enhance != result.NewEnhance {
		t.Fatalf("mejora del arma = %d, resultado = %d", weapon.Enhance, result.NewEnhance)
	}
	if result.Success && weapon.Enhance != 1 {
		t.Fatalf("éxito pero enhance=%d", weapon.Enhance)
	}
}

func TestShopBuyAndSell(t *testing.T) {
	w := testWorld(nil, testItemData())
	out := make(chan []byte, 128)
	w.addPlayer(Join{SessionID: 1, Character: characterWithItems(nil, 500), Out: out})
	p := w.players[1]
	drainMessages(t, out)

	w.shopBuy(ShopBuy{SessionID: 1, Tmpl: "pot_hp", Qty: 2})
	if p.Gold != 400 {
		t.Fatalf("oro tras comprar = %d, quería 400", p.Gold)
	}
	if len(p.Items) != 1 || p.Items[0].Qty != 2 {
		t.Fatalf("inventario tras comprar = %+v", p.Items)
	}

	potionID := p.Items[0].ID
	w.shopSell(ShopSell{SessionID: 1, ItemID: potionID})
	if p.Gold != 406 { // 2 pociones × 3 de venta
		t.Fatalf("oro tras vender = %d, quería 406", p.Gold)
	}
	if len(p.Items) != 0 {
		t.Fatalf("inventario tras vender = %+v", p.Items)
	}
}

func TestLootDropGoldAndPickup(t *testing.T) {
	spec := testMobSpec("dummy", 1, 10)
	spec.Loot = []gamedata.LootEntry{{Item: "pot_hp", Chance: 1}}
	spec.GoldMin, spec.GoldMax, spec.GoldChance = 5, 5, 1

	data := testItemData()
	data.Mobs = map[string]gamedata.Mob{"dummy": spec}
	data.Spawns = []gamedata.Spawn{{Mob: "dummy", X: 1.5, Z: 0}}

	out := make(chan []byte, 256)
	w := testWorld(nil, data)
	w.addPlayer(Join{SessionID: 1, Character: characterWithItems(nil, 0), Out: out})
	p := w.players[1]
	m := w.mobsByID[mobIDBase]
	drainMessages(t, out)

	w.playerAttack(p, m)
	if p.Gold != 5 {
		t.Fatalf("oro tras matar = %d, quería 5", p.Gold)
	}
	if len(w.drops) != 1 {
		t.Fatalf("drops = %d, quería 1", len(w.drops))
	}
	d := w.drops[0]
	p.X, p.Z = d.X, d.Z // acercarse para el pickup

	w.pickup(Pickup{SessionID: 1, DropID: d.ID})
	if len(w.drops) != 0 {
		t.Fatalf("el drop debería haberse recogido")
	}
	if len(p.Items) != 1 || p.Items[0].Tmpl != "pot_hp" {
		t.Fatalf("inventario tras pickup = %+v", p.Items)
	}

	msgs := drainMessages(t, out)
	env, ok := findMessage(msgs, "mob_dead")
	if !ok {
		t.Fatal("falta mob_dead")
	}
	var dead protocol.MobDead
	if err := json.Unmarshal(env.D, &dead); err != nil {
		t.Fatalf("mob_dead: %v", err)
	}
	if dead.Gold != 5 {
		t.Fatalf("oro del evento = %d, quería 5", dead.Gold)
	}
}
