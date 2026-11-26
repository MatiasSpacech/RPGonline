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
     "loot":[{"item":"pot","chance":0.5}]},
    {"id":"jefe","name":"Jefe","level":10,"hp":500,"damage_min":10,"damage_max":20,
     "defense":5,"xp":500,"speed":2,"aggro_radius":10,"attack_range":2,
     "attack_interval_ms":1500,"respawn_seconds":60,
     "summon_at_percent":50,"summon_mob":"babosa","summon_count":2,
     "enrage_at_percent":30,"enrage_speed_mult":1.4,"enrage_damage_mult":1.2}
  ]
}`

const validSkills = `{
  "skills": [
    {"id":"bola","name":"Bola","class":"mage","level_req":1,"mana_cost":6,
     "cooldown_ms":3000,"kind":"single","multiplier":1.8,"range":8.5},
    {"id":"nova","name":"Nova","class":"mage","level_req":8,"mana_cost":20,
     "cooldown_ms":12000,"kind":"aoe","multiplier":1.2,"range":8.5,
     "radius":4,"max_targets":6},
    {"id":"tp","name":"Teleport","class":"mage","level_req":14,"mana_cost":15,
     "cooldown_ms":20000,"kind":"teleport","teleport_distance":6}
  ]
}`

const validMapA = `{
  "id":"town","name":"Town","theme":"town","safe":true,"level_req":1,
  "player_spawn":{"x":0,"z":0},
  "portals":[{"id":"to_field","name":"Field","x":-8,"z":0,
    "target":"field","target_x":0,"target_z":6,"level_req":1}],
  "spawns":[]
}`

const validMapB = `{
  "id":"field","name":"Field","theme":"field","safe":false,"level_req":1,
  "player_spawn":{"x":0,"z":0},
  "portals":[{"id":"to_town","name":"Town","x":0,"z":8,
    "target":"town","target_x":-6,"target_z":2,"level_req":1}],
  "spawns":[{"mob":"babosa","x":5,"z":5}]
}`

func testFS(items, mobs, skills string, maps map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{
		"items.json":  {Data: []byte(items)},
		"mobs.json":   {Data: []byte(mobs)},
		"skills.json": {Data: []byte(skills)},
	}
	for name, content := range maps {
		fsys["maps/"+name] = &fstest.MapFile{Data: []byte(content)}
	}
	return fsys
}

func validFS() fstest.MapFS {
	return testFS(validItems, validMobs, validSkills, map[string]string{
		"town.json":  validMapA,
		"field.json": validMapB,
	})
}

func TestLoadValid(t *testing.T) {
	data, err := Load(validFS(), "items.json", "mobs.json", "skills.json", "maps")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(data.Items) != 2 || len(data.Mobs) != 2 || len(data.Skills) != 3 || len(data.Maps) != 2 {
		t.Fatalf("conteos = items %d, mobs %d, skills %d, maps %d",
			len(data.Items), len(data.Mobs), len(data.Skills), len(data.Maps))
	}
	if !data.Maps["town"].Safe || data.Maps["field"].Safe {
		t.Fatal("safe/unsafe mal configurados")
	}
	if data.Maps["town"].Portals[0].Target != "field" {
		t.Fatal("portal de town mal configurado")
	}
	boss := data.Mobs["jefe"]
	if boss.SummonMob != "babosa" || boss.SummonCount != 2 || boss.EnrageAtPercent != 30 {
		t.Fatalf("mecánicas de boss = %+v", boss)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name   string
		items  string
		mobs   string
		skills string
		maps   map[string]string
	}{
		{
			name:   "items inválidos",
			items:  `{`,
			mobs:   validMobs,
			skills: validSkills,
			maps:   map[string]string{"a.json": validMapA, "b.json": validMapB},
		},
		{
			name: "item duplicado",
			items: `{"items":[
				{"id":"x","name":"X","kind":"core","stackable":true,"max_stack":20,"value":1},
				{"id":"x","name":"Y","kind":"core","stackable":true,"max_stack":20,"value":1}]}`,
			mobs:   validMobs,
			skills: validSkills,
			maps:   map[string]string{"a.json": validMapA, "b.json": validMapB},
		},
		{
			name:   "arma sin daño",
			items:  `{"items":[{"id":"x","name":"X","kind":"weapon","slot":"weapon","value":10}]}`,
			mobs:   validMobs,
			skills: validSkills,
			maps:   map[string]string{"a.json": validMapA, "b.json": validMapB},
		},
		{
			name:   "loot inexistente",
			items:  validItems,
			mobs:   `{"mobs":[{"id":"m","name":"M","level":1,"hp":1,"damage_min":1,"damage_max":1,"speed":1,"attack_range":1,"attack_interval_ms":1000,"respawn_seconds":10,"loot":[{"item":"fantasma","chance":0.5}]}]}`,
			skills: validSkills,
			maps:   map[string]string{"a.json": validMapA, "b.json": validMapB},
		},
		{
			name:   "summon inexistente",
			items:  validItems,
			mobs:   `{"mobs":[{"id":"m","name":"M","level":1,"hp":1,"damage_min":1,"damage_max":1,"speed":1,"attack_range":1,"attack_interval_ms":1000,"respawn_seconds":10,"summon_at_percent":50,"summon_mob":"fantasma","summon_count":1}]}`,
			skills: validSkills,
			maps:   map[string]string{"a.json": validMapA, "b.json": validMapB},
		},
		{
			name:   "skill inválida",
			items:  validItems,
			mobs:   validMobs,
			skills: `{"skills":[{"id":"x","name":"X","class":"paladin","level_req":1,"mana_cost":1,"cooldown_ms":1000,"kind":"single","multiplier":1,"range":1}]}`,
			maps:   map[string]string{"a.json": validMapA, "b.json": validMapB},
		},
		{
			name:   "theme inválido",
			items:  validItems,
			mobs:   validMobs,
			skills: validSkills,
			maps:   map[string]string{"a.json": `{"id":"x","name":"X","theme":"space","level_req":1,"player_spawn":{"x":0,"z":0}}`},
		},
		{
			name:   "spawn mob inexistente",
			items:  validItems,
			mobs:   validMobs,
			skills: validSkills,
			maps: map[string]string{
				"a.json": validMapA,
				"b.json": `{"id":"field","name":"Field","theme":"field","level_req":1,"player_spawn":{"x":0,"z":0},"spawns":[{"mob":"fantasma","x":0,"z":0}]}`,
			},
		},
		{
			name:   "portal a mapa inexistente",
			items:  validItems,
			mobs:   validMobs,
			skills: validSkills,
			maps: map[string]string{
				"a.json": `{"id":"town","name":"Town","theme":"town","level_req":1,"player_spawn":{"x":0,"z":0},"portals":[{"id":"p","name":"P","x":0,"z":0,"target":"nope","target_x":0,"target_z":0,"level_req":1}]}`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(testFS(tc.items, tc.mobs, tc.skills, tc.maps), "items.json", "mobs.json", "skills.json", "maps"); err == nil {
				t.Fatal("esperaba error de validación")
			}
		})
	}
}
