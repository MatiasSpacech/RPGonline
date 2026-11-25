package gamedata

import (
	"testing"
	"testing/fstest"
)

const validMobs = `{
  "mobs": [
    {"id":"babosa","name":"Babosa","level":1,"hp":30,"damage_min":3,"damage_max":4,
     "defense":0,"xp":12,"speed":2.5,"aggro_radius":0,"attack_range":1.4,
     "attack_interval_ms":1500,"respawn_seconds":20,"passive":true}
  ]
}`

const validSpawns = `{
  "id":"llanuras",
  "spawns":[{"mob":"babosa","x":-5,"z":4},{"mob":"babosa","x":5,"z":4}]
}`

func testFS(mobs, spawns string) fstest.MapFS {
	return fstest.MapFS{
		"mobs.json":      {Data: []byte(mobs)},
		"maps/test.json": {Data: []byte(spawns)},
	}
}

func TestLoadValid(t *testing.T) {
	data, err := Load(testFS(validMobs, validSpawns), "mobs.json", "maps/test.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if data.MapID != "llanuras" || len(data.Spawns) != 2 {
		t.Fatalf("data = %+v", data)
	}
	mob, ok := data.Mobs["babosa"]
	if !ok || mob.HP != 30 || !mob.Passive {
		t.Fatalf("mob = %+v", mob)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name   string
		mobs   string
		spawns string
	}{
		{
			name:   "json de mobs inválido",
			mobs:   `{`,
			spawns: validSpawns,
		},
		{
			name:   "sin mobs",
			mobs:   `{"mobs":[]}`,
			spawns: validSpawns,
		},
		{
			name: "mob id duplicado",
			mobs: `{"mobs":[
				{"id":"x","name":"X","level":1,"hp":1,"damage_min":1,"damage_max":1,
				 "speed":1,"attack_range":1,"attack_interval_ms":1000,"respawn_seconds":10},
				{"id":"x","name":"X2","level":1,"hp":1,"damage_min":1,"damage_max":1,
				 "speed":1,"attack_range":1,"attack_interval_ms":1000,"respawn_seconds":10}]}`,
			spawns: validSpawns,
		},
		{
			name:   "spawn referencia mob inexistente",
			mobs:   validMobs,
			spawns: `{"id":"llanuras","spawns":[{"mob":"fantasma","x":0,"z":0}]}`,
		},
		{
			name:   "spawn sin coordenadas válidas",
			mobs:   validMobs,
			spawns: `{"id":"llanuras","spawns":[{"mob":"babosa","x":1e999,"z":0}]}`,
		},
		{
			name:   "mapa sin id",
			mobs:   validMobs,
			spawns: `{"spawns":[{"mob":"babosa","x":0,"z":0}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Load(testFS(tc.mobs, tc.spawns), "mobs.json", "maps/test.json"); err == nil {
				t.Fatal("esperaba error de validación")
			}
		})
	}
}
