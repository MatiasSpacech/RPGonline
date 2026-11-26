// Package gamedata carga el contenido estático del juego (ítems, mobs,
// spawns). Vive en shared/gamedata y se valida al arrancar: un dato roto
// rompe el startup, no la partida (ver docs/03-arquitectura.md).
package gamedata

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
)

// ItemTemplate es una plantilla de ítem (equipo o consumible).
type ItemTemplate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"` // weapon | armor | potion | core

	Slot  string `json:"slot,omitempty"`  // weapon | helm | chest | gloves | boots | shield
	Class string `json:"class,omitempty"` // warrior | mage | archer | "" (cualquiera)

	ReqStr int `json:"req_str,omitempty"`
	ReqAgi int `json:"req_agi,omitempty"`
	ReqVit int `json:"req_vit,omitempty"`
	ReqEne int `json:"req_ene,omitempty"`

	MinDamage  int     `json:"min_damage,omitempty"`
	MaxDamage  int     `json:"max_damage,omitempty"`
	IntervalMS float64 `json:"interval_ms,omitempty"`
	Range      float64 `json:"range,omitempty"`
	Defense    int     `json:"defense,omitempty"`

	Value    int64 `json:"value"`
	BuyPrice int64 `json:"buy_price,omitempty"`

	Stackable bool `json:"stackable,omitempty"`
	MaxStack  int  `json:"max_stack,omitempty"`

	HealKind    string `json:"heal_kind,omitempty"` // hp | mp
	HealPercent int    `json:"heal_percent,omitempty"`
}

// LootEntry es una posibilidad de drop de un mob.
type LootEntry struct {
	Item   string  `json:"item"`
	Chance float64 `json:"chance"`
}

// Mob es una plantilla de mob (stats base, IA, respawn, loot).
type Mob struct {
	ID               string      `json:"id"`
	Name             string      `json:"name"`
	Level            int         `json:"level"`
	HP               int         `json:"hp"`
	DamageMin        int         `json:"damage_min"`
	DamageMax        int         `json:"damage_max"`
	Defense          int         `json:"defense"`
	XP               int64       `json:"xp"`
	Speed            float64     `json:"speed"`
	AggroRadius      float64     `json:"aggro_radius"`
	AttackRange      float64     `json:"attack_range"`
	AttackIntervalMS int         `json:"attack_interval_ms"`
	RespawnSeconds   int         `json:"respawn_seconds"`
	Passive          bool        `json:"passive"`
	Loot             []LootEntry `json:"loot,omitempty"`
	GoldMin          int64       `json:"gold_min,omitempty"`
	GoldMax          int64       `json:"gold_max,omitempty"`
	GoldChance       float64     `json:"gold_chance,omitempty"`
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
	Items  map[string]ItemTemplate
	Spawns []Spawn
}

// Load lee y valida items.json, mobs.json y el archivo de spawns del mapa.
func Load(fsys fs.FS, itemsPath, mobsPath, spawnsPath string) (*Data, error) {
	items, err := loadItems(fsys, itemsPath)
	if err != nil {
		return nil, err
	}
	mobs, err := loadMobs(fsys, mobsPath, items)
	if err != nil {
		return nil, err
	}
	mapFile, err := loadSpawns(fsys, spawnsPath, mobs)
	if err != nil {
		return nil, err
	}
	return &Data{MapID: mapFile.ID, Mobs: mobs, Items: items, Spawns: mapFile.Spawns}, nil
}

func loadItems(fsys fs.FS, path string) (map[string]ItemTemplate, error) {
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("gamedata: leyendo %s: %w", path, err)
	}
	var file struct {
		Items []ItemTemplate `json:"items"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("gamedata: JSON inválido en %s: %w", path, err)
	}
	if len(file.Items) == 0 {
		return nil, fmt.Errorf("gamedata: %s no define items", path)
	}

	items := make(map[string]ItemTemplate, len(file.Items))
	for _, it := range file.Items {
		if err := validateItem(it); err != nil {
			return nil, fmt.Errorf("gamedata: item %q: %w", it.ID, err)
		}
		if _, exists := items[it.ID]; exists {
			return nil, fmt.Errorf("gamedata: item duplicado %q", it.ID)
		}
		items[it.ID] = it
	}
	return items, nil
}

func loadMobs(fsys fs.FS, path string, items map[string]ItemTemplate) (map[string]Mob, error) {
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("gamedata: leyendo %s: %w", path, err)
	}
	var file struct {
		Mobs []Mob `json:"mobs"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("gamedata: JSON inválido en %s: %w", path, err)
	}
	if len(file.Mobs) == 0 {
		return nil, fmt.Errorf("gamedata: %s no define mobs", path)
	}

	mobs := make(map[string]Mob, len(file.Mobs))
	for _, m := range file.Mobs {
		if err := validateMob(m); err != nil {
			return nil, fmt.Errorf("gamedata: mob %q: %w", m.ID, err)
		}
		if _, exists := mobs[m.ID]; exists {
			return nil, fmt.Errorf("gamedata: mob duplicado %q", m.ID)
		}
		for i, l := range m.Loot {
			if _, ok := items[l.Item]; !ok {
				return nil, fmt.Errorf("gamedata: mob %q loot %d referencia un item inexistente %q", m.ID, i, l.Item)
			}
		}
		mobs[m.ID] = m
	}
	return mobs, nil
}

func loadSpawns(fsys fs.FS, path string, mobs map[string]Mob) (MapSpawns, error) {
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return MapSpawns{}, fmt.Errorf("gamedata: leyendo %s: %w", path, err)
	}
	var mapFile MapSpawns
	if err := json.Unmarshal(raw, &mapFile); err != nil {
		return MapSpawns{}, fmt.Errorf("gamedata: JSON inválido en %s: %w", path, err)
	}
	if mapFile.ID == "" {
		return MapSpawns{}, fmt.Errorf("gamedata: %s sin id de mapa", path)
	}
	if len(mapFile.Spawns) == 0 {
		return MapSpawns{}, fmt.Errorf("gamedata: %s no define spawns", path)
	}
	for i, s := range mapFile.Spawns {
		if _, ok := mobs[s.Mob]; !ok {
			return MapSpawns{}, fmt.Errorf("gamedata: spawn %d referencia un mob inexistente %q", i, s.Mob)
		}
		if !finite(s.X) || !finite(s.Z) {
			return MapSpawns{}, fmt.Errorf("gamedata: spawn %d tiene coordenadas inválidas", i)
		}
	}
	return mapFile, nil
}

func validateItem(it ItemTemplate) error {
	if it.ID == "" {
		return fmt.Errorf("id vacío")
	}
	if it.Name == "" {
		return fmt.Errorf("name vacío")
	}
	if it.Value < 0 {
		return fmt.Errorf("value no puede ser negativo")
	}
	if it.BuyPrice < 0 {
		return fmt.Errorf("buy_price no puede ser negativo")
	}
	if it.Class != "" && it.Class != "warrior" && it.Class != "mage" && it.Class != "archer" {
		return fmt.Errorf("class inválida")
	}

	switch it.Kind {
	case "weapon":
		if it.Slot != "weapon" {
			return fmt.Errorf("weapon debe tener slot=weapon")
		}
		if it.MinDamage < 1 || it.MaxDamage < it.MinDamage || it.IntervalMS <= 0 || it.Range <= 0 {
			return fmt.Errorf("stats de arma inválidos")
		}
	case "armor":
		switch it.Slot {
		case "helm", "chest", "gloves", "boots", "shield":
		default:
			return fmt.Errorf("slot de armadura inválido")
		}
		if it.Defense < 1 {
			return fmt.Errorf("defense debe ser >= 1")
		}
	case "potion":
		if it.HealKind != "hp" && it.HealKind != "mp" {
			return fmt.Errorf("heal_kind inválido")
		}
		if it.HealPercent < 1 || it.HealPercent > 100 {
			return fmt.Errorf("heal_percent debe estar entre 1 y 100")
		}
		if !it.Stackable || it.MaxStack < 1 {
			return fmt.Errorf("las pociones deben ser stackables")
		}
	case "core":
		if !it.Stackable || it.MaxStack < 1 {
			return fmt.Errorf("los núcleos deben ser stackables")
		}
	default:
		return fmt.Errorf("kind inválido %q", it.Kind)
	}
	return nil
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
	case m.GoldMin < 0 || m.GoldMax < m.GoldMin:
		return fmt.Errorf("rango de oro inválido")
	case m.GoldChance < 0 || m.GoldChance > 1:
		return fmt.Errorf("gold_chance debe estar entre 0 y 1")
	}
	for i, l := range m.Loot {
		if l.Item == "" {
			return fmt.Errorf("loot %d sin item", i)
		}
		if l.Chance <= 0 || l.Chance > 1 {
			return fmt.Errorf("loot %d con chance inválida", i)
		}
	}
	return nil
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
