// Package gamedata carga el contenido estático del juego (ítems, mobs, mapas,
// skills). Vive en shared/gamedata y se valida al arrancar: un dato roto
// rompe el startup, no la partida (ver docs/03-arquitectura.md).
package gamedata

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"path"
	"sort"
	"strings"
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

// Mob es una plantilla de mob (stats base, IA, respawn, loot y mecánicas de boss).
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

	// Mecánicas de boss (opcionales).
	SummonAtPercent int     `json:"summon_at_percent,omitempty"` // 0 = sin summon
	SummonMob       string  `json:"summon_mob,omitempty"`
	SummonCount     int     `json:"summon_count,omitempty"`
	EnrageAtPercent int     `json:"enrage_at_percent,omitempty"` // 0 = sin enrage
	EnrageSpeedMult float64 `json:"enrage_speed_mult,omitempty"`
	EnrageDmgMult   float64 `json:"enrage_damage_mult,omitempty"`
}

// Spawn es un punto de aparición de un mob en un mapa.
type Spawn struct {
	Mob string  `json:"mob"`
	X   float64 `json:"x"`
	Z   float64 `json:"z"`
}

// Vec2 es una posición X/Z en el plano.
type Vec2 struct {
	X float64 `json:"x"`
	Z float64 `json:"z"`
}

// Portal conecta dos mapas con un requisito de nivel.
type Portal struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	X        float64 `json:"x"`
	Z        float64 `json:"z"`
	Target   string  `json:"target"`
	TargetX  float64 `json:"target_x"`
	TargetZ  float64 `json:"target_z"`
	LevelReq int     `json:"level_req"`
}

// Map es un mapa jugable con sus spawns y portales.
type Map struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Theme       string   `json:"theme"` // town | field | dungeon
	Safe        bool     `json:"safe"`
	LevelReq    int      `json:"level_req"`
	PlayerSpawn Vec2     `json:"player_spawn"`
	Portals     []Portal `json:"portals,omitempty"`
	Spawns      []Spawn  `json:"spawns,omitempty"`
}

// Skill es una habilidad de clase.
type Skill struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Class    string `json:"class"`
	LevelReq int    `json:"level_req"`

	ManaCost   int `json:"mana_cost"`
	CooldownMS int `json:"cooldown_ms"`

	Kind       string  `json:"kind"` // single | aoe | heal | buff | teleport
	Multiplier float64 `json:"multiplier,omitempty"`
	Hits       int     `json:"hits,omitempty"`
	Range      float64 `json:"range,omitempty"`
	Radius     float64 `json:"radius,omitempty"`
	MaxTargets int     `json:"max_targets,omitempty"`

	HealPercent      int     `json:"heal_percent,omitempty"`
	BuffPercent      int     `json:"buff_percent,omitempty"`
	BuffDurationMS   int     `json:"buff_duration_ms,omitempty"`
	TeleportDistance float64 `json:"teleport_distance,omitempty"`
}

// Data es el contenido estático cargado y validado.
type Data struct {
	Mobs   map[string]Mob
	Items  map[string]ItemTemplate
	Skills map[string]Skill
	Maps   map[string]Map
}

// Load lee y valida los archivos de contenido.
func Load(fsys fs.FS, itemsPath, mobsPath, skillsPath, mapsDir string) (*Data, error) {
	items, err := loadItems(fsys, itemsPath)
	if err != nil {
		return nil, err
	}
	mobs, err := loadMobs(fsys, mobsPath, items)
	if err != nil {
		return nil, err
	}
	skills, err := loadSkills(fsys, skillsPath)
	if err != nil {
		return nil, err
	}
	maps, err := loadMaps(fsys, mapsDir, mobs)
	if err != nil {
		return nil, err
	}
	return &Data{Mobs: mobs, Items: items, Skills: skills, Maps: maps}, nil
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
	for _, m := range mobs {
		if m.SummonMob != "" {
			if _, ok := mobs[m.SummonMob]; !ok {
				return nil, fmt.Errorf("gamedata: mob %q invoca un mob inexistente %q", m.ID, m.SummonMob)
			}
		}
	}
	return mobs, nil
}

func loadSkills(fsys fs.FS, path string) (map[string]Skill, error) {
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("gamedata: leyendo %s: %w", path, err)
	}
	var file struct {
		Skills []Skill `json:"skills"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("gamedata: JSON inválido en %s: %w", path, err)
	}
	if len(file.Skills) == 0 {
		return nil, fmt.Errorf("gamedata: %s no define skills", path)
	}

	skills := make(map[string]Skill, len(file.Skills))
	for _, s := range file.Skills {
		if err := validateSkill(s); err != nil {
			return nil, fmt.Errorf("gamedata: skill %q: %w", s.ID, err)
		}
		if _, exists := skills[s.ID]; exists {
			return nil, fmt.Errorf("gamedata: skill duplicada %q", s.ID)
		}
		skills[s.ID] = s
	}
	return skills, nil
}

func loadMaps(fsys fs.FS, dir string, mobs map[string]Mob) (map[string]Map, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("gamedata: leyendo %s: %w", dir, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	maps := make(map[string]Map)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		filePath := path.Join(dir, entry.Name())
		raw, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return nil, fmt.Errorf("gamedata: leyendo %s: %w", filePath, err)
		}
		var m Map
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("gamedata: JSON inválido en %s: %w", filePath, err)
		}
		if err := validateMap(m, mobs); err != nil {
			return nil, fmt.Errorf("gamedata: mapa %q: %w", m.ID, err)
		}
		if _, exists := maps[m.ID]; exists {
			return nil, fmt.Errorf("gamedata: mapa duplicado %q", m.ID)
		}
		maps[m.ID] = m
	}
	if len(maps) == 0 {
		return nil, fmt.Errorf("gamedata: %s no define mapas", dir)
	}
	for _, m := range maps {
		for _, portal := range m.Portals {
			if _, ok := maps[portal.Target]; !ok {
				return nil, fmt.Errorf("gamedata: portal %q de %q apunta a un mapa inexistente %q", portal.ID, m.ID, portal.Target)
			}
		}
	}
	return maps, nil
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
	case m.SummonAtPercent < 0 || m.SummonAtPercent > 100:
		return fmt.Errorf("summon_at_percent inválido")
	case m.SummonAtPercent > 0 && (m.SummonMob == "" || m.SummonCount < 1):
		return fmt.Errorf("summon incompleto")
	case m.EnrageAtPercent < 0 || m.EnrageAtPercent > 100:
		return fmt.Errorf("enrage_at_percent inválido")
	case m.EnrageAtPercent > 0 && (m.EnrageSpeedMult < 1 || m.EnrageDmgMult < 1):
		return fmt.Errorf("enrage incompleto")
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

func validateSkill(s Skill) error {
	if s.ID == "" || s.Name == "" {
		return fmt.Errorf("id/name vacío")
	}
	switch s.Class {
	case "warrior", "mage", "archer":
	default:
		return fmt.Errorf("class inválida")
	}
	if s.LevelReq < 1 {
		return fmt.Errorf("level_req debe ser >= 1")
	}
	if s.ManaCost < 0 {
		return fmt.Errorf("mana_cost no puede ser negativo")
	}
	if s.CooldownMS <= 0 {
		return fmt.Errorf("cooldown_ms debe ser > 0")
	}
	switch s.Kind {
	case "single", "aoe":
		if s.Multiplier <= 0 || s.Range <= 0 {
			return fmt.Errorf("skill de daño sin multiplier/range")
		}
		if s.Hits < 1 {
			s.Hits = 1
		}
		if s.Kind == "aoe" && (s.Radius <= 0 || s.MaxTargets < 1) {
			return fmt.Errorf("aoe sin radius/max_targets")
		}
	case "heal":
		if s.HealPercent < 1 || s.HealPercent > 100 {
			return fmt.Errorf("heal_percent inválido")
		}
	case "buff":
		if s.BuffPercent < 1 || s.BuffDurationMS <= 0 {
			return fmt.Errorf("buff inválido")
		}
	case "teleport":
		if s.TeleportDistance <= 0 {
			return fmt.Errorf("teleport_distance inválido")
		}
	default:
		return fmt.Errorf("kind inválido %q", s.Kind)
	}
	return nil
}

func validateMap(m Map, mobs map[string]Mob) error {
	if m.ID == "" || m.Name == "" {
		return fmt.Errorf("id/name vacío")
	}
	switch m.Theme {
	case "town", "field", "dungeon":
	default:
		return fmt.Errorf("theme inválido %q", m.Theme)
	}
	if m.LevelReq < 1 {
		return fmt.Errorf("level_req debe ser >= 1")
	}
	if !finite(m.PlayerSpawn.X) || !finite(m.PlayerSpawn.Z) {
		return fmt.Errorf("player_spawn inválido")
	}
	portalIDs := make(map[string]bool)
	for i, s := range m.Spawns {
		if _, ok := mobs[s.Mob]; !ok {
			return fmt.Errorf("spawn %d referencia un mob inexistente %q", i, s.Mob)
		}
		if !finite(s.X) || !finite(s.Z) {
			return fmt.Errorf("spawn %d tiene coordenadas inválidas", i)
		}
	}
	for _, p := range m.Portals {
		if p.ID == "" || p.Name == "" || p.Target == "" {
			return fmt.Errorf("portal incompleto")
		}
		if portalIDs[p.ID] {
			return fmt.Errorf("portal duplicado %q", p.ID)
		}
		portalIDs[p.ID] = true
		if !finite(p.X) || !finite(p.Z) || !finite(p.TargetX) || !finite(p.TargetZ) {
			return fmt.Errorf("portal %q con coordenadas inválidas", p.ID)
		}
		if p.LevelReq < 1 {
			return fmt.Errorf("portal %q con level_req inválido", p.ID)
		}
	}
	return nil
}

func finite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
