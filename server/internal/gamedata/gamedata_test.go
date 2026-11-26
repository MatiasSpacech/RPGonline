package gamedata

import (
	"testing"
	"testing/fstest"
)

const validItems = `{
  "items": [
    {"id":"sword","name":"Sword","kind":"weapon","slot":"weapon",
     "min_damage":1,"max_damage":2,"interval_ms":1000,"range":1.8,"value":10},
    {"id":"pot","name":"Poción","kind":"potion","heal_kind":"hp","heal_percent":40,
     "stackable":true,"max_stack":20,"value":1,"buy_price":10}
  ]
}`

const validMobs = `{
  "mobs": [
    {"id":"babosa","name":"Babosa","level":1,"hp":30,"damage_min":3,"damage_max":4,
     "defense":0,"xp":12,"speed":2.5,"aggro_radius":0,"attack_range":1.4,
     "attack_interval_ms":1500,"respawn_seconds":20,"passive":true,
     "gold_min":1,"gold_max":3,"gold_chance":0.6,
     "loot":[{"item":"pot","chance":0.5}]}
  ]
}`

const validSpawns = `{
  "id":"llanuras",
  "spawns":[{"mob":"babosa","x":-5,"z":4},{"mob":"babosa","x":5,"z":4}]
}`

func testFS(items, mobs, spawns string) fstest.MapFS {
	return fstest.MapFS{
		"items.json":     {Data: []byte(items)},
		"mobs.json":      {Data: []byte(mobs)},
		"maps/test.json": {Data: []byte(spawns)},
	}
}

func TestLoadValid(t *testing.T) {
	data, err := Load(testFS(validItems, validMobs, validSpawns), "items.json", "mobs.json", "maps/test.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if data.MapID != "llanuras" || len(data.Spawns) != 2 {
		t.Fatalf("data = %+v", data)
	}
	mob, ok := data.Mobs["babosa"]
	if !ok || mob.HP != 30 || !mob.Passive || len(mob.Loot) != 1 {
		t.Fatalf("mob = %+v", mob)
	}
	item, ok := data.Items["sword"]
	if !ok || item.MinDamage != 1 || item.Slot != "weapon" {
		t.Fatalf("item = %+v", item)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name   string
		items  string
		mobs   string
		spawns string
	}{
		{
			name:   "json de items inválido",
			items:  `{`,
			mobs:   validMobs,
			spawns: validSpawns,
		},
		{
			name:   "sin items",
			items:  `{"items":[]}`,
			mobs:   validMobs,
			spawns: validSpawns,
		},
		{
			name: "item duplicado",
			items: `{"items":[
				{"id":"x","name":"X","kind":"weapon","slot":"weapon","min_damage":1,"max_damage":2,
				 "interval_ms":1000,"range":1.8,"value":10},
				{"id":"x","name":"X2","kind":"weapon","slot":"weapon","min_damage":1,"max_damage":2,
				 "interval_ms":1000,"range":1.8,"value":10}]}`,
			mobs:   validMobs,
			spawns: validSpawns,
		},
		{
			name:   "arma sin daño",
			items:  `{"items":[{"id":"x","name":"X","kind":"weapon","slot":"weapon","value":10}]}`,
			mobs:   `{"mobs":[{"id":"m","name":"M","level":1,"hp":1,"damage_min":1,"damage_max":1,"speed":1,"attack_range":1,"attack_interval_ms":1000,"respawn_seconds":10}]}`,
			spawns: validSpawns,
		},
		{
			name:  "loot referencia item inexistente",
			items: validItems,
			mobs: `{"mobs":[{"id":"m","name":"M","level":1,"hp":1,"damage_min":1,"damage_max":1,
				"speed":1,"attack_range":1,"attack_interval_ms":1000,"respawn_seconds":10,
				"loot":[{"item":"fantasma","chance":0.5}]}]}`,
			spawns: validSpawns,
		},
		{
			name:  "rango de oro inválido",
			items: validItems,
			mobs: `{"mobs":[{"id":"m","name":"M","level":1,"hp":1,"damage_min":1,"damage_max":1,
				"speed":1,"attack_range":1,"attack_interval_ms":1000,"respawn_seconds":10,
				"gold_min":10,"gold_max":5}]}`,
			spawns: validSpawns,
		},
		{
			name:   "spawn referencia mob inexistente",
			items:  validItems,
			mobs:   validMobs,
			spawns: `{"id":"llanuras","spawns":[{"mob":"fantasma","x":0,"z":0}]}`,
		},
		{
			name:   "spawn sin coordenadas válidas",
			items:  validItems,
			mobs:   validMobs,
			spawns: `{"id":"llanuras","spawns":[{"mob":"babosa","x":1e999,"z":0}]}`,
		},
		{
			name:   "mapa sin id",
			items:  validItems,
			mobs:   validMobs,
			spawns: `{"spawns":[{"mob":"babosa","x":0,"z":0}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(testFS(tc.items, tc.mobs, tc.spawns), "items.json", "mobs.json", "maps/test.json"); err == nil {
				t.Fatal("esperaba error de validación")
			}
		})
	}
}
