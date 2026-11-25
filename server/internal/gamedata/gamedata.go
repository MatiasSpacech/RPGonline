// Package gamedata carga el contenido estático del juego (mobs, spawns).
// Vive en shared/gamedata y se valida al arrancar: un dato roto rompe el
// startup, no la partida (ver docs/03-arquitectura.md).
package gamedata

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
)

// Mob es una plantilla de mob (stats base, IA y respawn).
type Mob struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Level            int     `json:"level"`
	HP               int     `json:"hp"`
	DamageMin        int     `json:"damage_min"`
	DamageMax        int     `json:"damage_max"`
	Defense          int     `json:"defense"`
	XP               int64   `json:"xp"`
	Speed            float64 `json:"speed"`
	AggroRadius      float64 `json:"aggro_radius"`
	AttackRange      float64 `json:"attack_range"`
	AttackIntervalMS int     `json:"attack_interval_ms"`
	RespawnSeconds   int     `json:"respawn_seconds"`
	Passive          bool    `json:"passive"`
}

// Spawn es un punto de aparición de un mob en un mapa.
type Spawn struct {
	Mob string  `json:"mob"`
	X   float64 `json:"x"`
	Z   float64 `json:"z"`
}

// MapSpawns es el archivo de un mapa (por ahora solo spawns).
type MapSpawns struct {
	ID     string  `json:"id"`
	Spawns []Spawn `json:"spawns"`
}

// Data es el contenido estático cargado y validado.
type Data struct {
	MapID  string
	Mobs   map[string]Mob
	Spawns []Spawn
}

// Load lee y valida mobs.json y el archivo de spawns del mapa.
func Load(fsys fs.FS, mobsPath, spawnsPath string) (*Data, error) {
	mobsRaw, err := fs.ReadFile(fsys, mobsPath)
	if err != nil {
		return nil, fmt.Errorf("gamedata: leyendo %s: %w", mobsPath, err)
	}
	var mobsFile struct {
		Mobs []Mob `json:"mobs"`
	}
	if err := json.Unmarshal(mobsRaw, &mobsFile); err != nil {
		return nil, fmt.Errorf("gamedata: JSON inválido en %s: %w", mobsPath, err)
	}
	if len(mobsFile.Mobs) == 0 {
		return nil, fmt.Errorf("gamedata: %s no define mobs", mobsPath)
	}

	mobs := make(map[string]Mob, len(mobsFile.Mobs))
	for _, m := range mobsFile.Mobs {
		if err := validateMob(m); err != nil {
			return nil, fmt.Errorf("gamedata: mob %q: %w", m.ID, err)
		}
		if _, exists := mobs[m.ID]; exists {
			return nil, fmt.Errorf("gamedata: mob duplicado %q", m.ID)
		}
		mobs[m.ID] = m
	}

	spawnsRaw, err := fs.ReadFile(fsys, spawnsPath)
	if err != nil {
		return nil, fmt.Errorf("gamedata: leyendo %s: %w", spawnsPath, err)
	}
	var mapFile MapSpawns
	if err := json.Unmarshal(spawnsRaw, &mapFile); err != nil {
		return nil, fmt.Errorf("gamedata: JSON inválido en %s: %w", spawnsPath, err)
	}
	if mapFile.ID == "" {
		return nil, fmt.Errorf("gamedata: %s sin id de mapa", spawnsPath)
	}
	if len(mapFile.Spawns) == 0 {
		return nil, fmt.Errorf("gamedata: %s no define spawns", spawnsPath)
	}
	for i, s := range mapFile.Spawns {
		if _, ok := mobs[s.Mob]; !ok {
			return nil, fmt.Errorf("gamedata: spawn %d referencia un mob inexistente %q", i, s.Mob)
		}
		if !finite(s.X) || !finite(s.Z) {
			return nil, fmt.Errorf("gamedata: spawn %d tiene coordenadas inválidas", i)
		}
	}

	return &Data{MapID: mapFile.ID, Mobs: mobs, Spawns: mapFile.Spawns}, nil
}

func validateMob(m Mob) error {
	switch {
	case m.ID == "":
		return fmt.Errorf("id vacío")
	case m.Name == "":
		return fmt.Errorf("name vacío")
	case m.Level < 1:
		return fmt.Errorf("level debe ser >= 1")
	case m.HP < 1:
		return fmt.Errorf("hp debe ser >= 1")
	case m.DamageMin < 1:
		return fmt.Errorf("damage_min debe ser >= 1")
	case m.DamageMax < m.DamageMin:
		return fmt.Errorf("damage_max debe ser >= damage_min")
	case m.Defense < 0:
		return fmt.Errorf("defense no puede ser negativa")
	case m.XP < 0:
		return fmt.Errorf("xp no puede ser negativo")
	case m.Speed <= 0:
		return fmt.Errorf("speed debe ser > 0")
	case m.AggroRadius < 0:
		return fmt.Errorf("aggro_radius no puede ser negativo")
	case m.AttackRange <= 0:
		return fmt.Errorf("attack_range debe ser > 0")
	case m.AttackIntervalMS <= 0:
		return fmt.Errorf("attack_interval_ms debe ser > 0")
	case m.RespawnSeconds <= 0:
		return fmt.Errorf("respawn_seconds debe ser > 0")
	}
	return nil
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
