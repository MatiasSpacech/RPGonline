// Package world contiene el estado y la simulación autoritativa del mundo.
//
// El mundo es propiedad exclusiva de UNA goroutine (el game loop): el resto
// del servidor le envía comandos por canal y él emite snapshots. Esto evita
// race conditions por diseño (ver docs/03-arquitectura.md).
package world

import (
	"context"
	"log/slog"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"munext/server/internal/combat"
	"munext/server/internal/gamedata"
	"munext/server/internal/model"
	"munext/server/internal/protocol"
)

const (
	// TickRate es la frecuencia de simulación del servidor (20 Hz).
	TickRate = 20
	// TickDuration es el tiempo entre ticks de simulación.
	TickDuration = time.Second / TickRate
	// SnapshotEvery: un snapshot cada 2 ticks => 10 Hz.
	SnapshotEvery = 2
	// PlayerSpeed es la velocidad de movimiento en metros por segundo.
	PlayerSpeed = 6.0
	// MapHalfSize limita el área jugable de los mapas.
	MapHalfSize = 19.0
	// DefaultMapID se usa cuando no hay gamedata cargada (tests).
	DefaultMapID = "llanuras"
	// saveInterval es cada cuánto se persisten los personajes conectados.
	saveInterval = 60 * time.Second

	// mobIDBase separa los ids de mobs de los ids de personajes (BIGSERIAL).
	mobIDBase int64 = 1_000_000
	// mobLeash: distancia máxima desde el spawn antes de resetear el mob.
	mobLeash = 14.0
	// combatCooldownTicks: 5 s fuera de combate antes de regenerar.
	combatCooldownTicks uint64 = 5 * TickRate
	// regenIntervalTicks: regenerar una vez por segundo.
	regenIntervalTicks uint64 = TickRate
	// potionCooldownTicks: 3 s entre pociones.
	potionCooldownTicks uint64 = 3 * TickRate
	// Grilla de inventario 8x5 (GDD §5.4).
	inventoryWidth  = 8
	inventoryHeight = 5
	// Los drops del suelo viven 90 s.
	dropLifetimeTicks = 90 * TickRate
)

// Command es una acción que las conexiones envían al game loop.
// La interfaz tiene un método no exportado: solo este paquete puede crear
// comandos válidos.
type Command interface {
	apply(*World)
}

// Join agrega un jugador (personaje seleccionado) al mundo.
type Join struct {
	SessionID int64
	Character model.Character
	Out       chan<- []byte
}

func (j Join) apply(w *World) { w.addPlayer(j) }

// Move cambia el punto destino de un jugador y corta el auto-ataque.
type Move struct {
	SessionID int64
	X, Z      float64
}

func (m Move) apply(w *World) { w.setTarget(m) }

// Attack fija el mob objetivo del auto-ataque (0 = detener).
type Attack struct {
	SessionID int64
	Target    int64
}

func (a Attack) apply(w *World) { w.setAttack(a) }

// Potion usa una poción de HP o MP ("hp" | "mp").
type Potion struct {
	SessionID int64
	Kind      string
}

func (p Potion) apply(w *World) { w.usePotion(p) }

// AllocStat gasta un punto de stat ("str" | "agi" | "vit" | "ene").
type AllocStat struct {
	SessionID int64
	Stat      string
}

func (a AllocStat) apply(w *World) { w.allocStat(a) }

// Leave quita a un jugador del mundo.
type Leave struct {
	SessionID int64
}

func (l Leave) apply(w *World) { w.removePlayer(l.SessionID) }

type player struct {
	ID        int64 // id del personaje (estable, viene de la BD)
	SessionID int64
	Name      string
	Class     string
	MapID     string
	Level     int
	XP        int64 // progreso dentro del nivel actual
	Str       int
	Agi       int
	Vit       int
	Ene       int
	StatPts   int
	Gold      int64

	HPMax, MPMax int
	HP, MP       int

	Items          []model.Item
	NextPotionTick uint64

	Skills      map[string]uint64 // skill id -> tick listo
	BuffUntil   uint64
	BuffPercent int

	X, Z             float64
	TargetX, TargetZ float64
	HasTarget        bool
	AttackTarget     int64
	NextAttackTick   uint64
	LastCombatTick   uint64

	Rot   float64
	Anim  string
	Out   chan<- []byte
	dirty bool
}

func (p *player) stats() combat.Stats {
	return combat.Stats{Level: p.Level, Str: p.Str, Agi: p.Agi, Vit: p.Vit, Ene: p.Ene}
}

func (p *player) recalcMax() {
	p.HPMax = combat.MaxHP(p.stats())
	p.MPMax = combat.MaxMP(p.stats())
	if p.HP > p.HPMax {
		p.HP = p.HPMax
	}
	if p.MP > p.MPMax {
		p.MP = p.MPMax
	}
}

func (p *player) entity() protocol.Entity {
	return protocol.Entity{
		ID:   p.ID,
		Kind: "pc",
		Name: p.Name,
		X:    p.X,
		Y:    0,
		Z:    p.Z,
		Rot:  p.Rot,
		Anim: p.Anim,
	}
}

type mob struct {
	ID        int64
	Spec      gamedata.Mob
	MapID     string
	X, Z      float64
	SpawnX    float64
	SpawnZ    float64
	HP        int
	State     string // idle | chase | attack | dead
	Target    int64  // SessionID del jugador objetivo
	Rot       float64
	NextAtk   uint64
	Respawn   uint64
	Temporary bool
	Summoned  bool
	Enraged   bool
}

func (m *mob) entity() protocol.Entity {
	return protocol.Entity{
		ID:    m.ID,
		Kind:  "mob",
		Tmpl:  m.Spec.ID,
		Name:  m.Spec.Name,
		Level: m.Spec.Level,
		X:     m.X,
		Y:     0,
		Z:     m.Z,
		Rot:   m.Rot,
		Anim:  m.State,
		HP:    m.HP,
		HPMax: m.Spec.HP,
	}
}

// World es el estado del mundo y su game loop.
type World struct {
	log          *slog.Logger
	players      map[int64]*player // por SessionID
	mobs         []*mob
	mobsByID     map[int64]*mob
	mobSpecs     map[string]gamedata.Mob
	drops        []*drop
	dropsByID    map[int64]*drop
	items        map[string]gamedata.ItemTemplate
	skills       map[string]gamedata.Skill
	maps         map[string]gamedata.Map
	gameData     protocol.GameData
	defaultMapID string
	townMapID    string
	nextMobID    int64
	inbox        chan Command
	saveCh       chan<- model.CharacterState
	tick         uint64
	rng          *rand.Rand
}

// New crea un mundo vacío. saveCh recibe los estados a persistir (nil = sin
// persistencia, útil en tests). data puede ser nil: el mundo corre sin mapas.
func New(log *slog.Logger, saveCh chan<- model.CharacterState, data *gamedata.Data) *World {
	w := &World{
		log:          log,
		players:      make(map[int64]*player),
		mobsByID:     make(map[int64]*mob),
		dropsByID:    make(map[int64]*drop),
		defaultMapID: DefaultMapID,
		townMapID:    DefaultMapID,
		inbox:        make(chan Command, 256),
		saveCh:       saveCh,
		gameData: protocol.GameData{
			Items:  make([]gamedata.ItemTemplate, 0),
			Skills: make([]gamedata.Skill, 0),
			Maps:   make([]gamedata.Map, 0),
		},
		rng: rand.New(rand.NewPCG(
			uint64(time.Now().UnixNano()),
			0x9E3779B97F4A7C15,
		)),
	}
	if data == nil {
		return w
	}

	w.items = data.Items
	w.skills = data.Skills
	w.mobSpecs = data.Mobs
	w.maps = data.Maps

	itemIDs := make([]string, 0, len(data.Items))
	for id := range data.Items {
		itemIDs = append(itemIDs, id)
	}
	slices.Sort(itemIDs)
	for _, id := range itemIDs {
		w.gameData.Items = append(w.gameData.Items, data.Items[id])
	}

	skillIDs := make([]string, 0, len(data.Skills))
	for id := range data.Skills {
		skillIDs = append(skillIDs, id)
	}
	slices.SortFunc(skillIDs, func(a, b string) int {
		sa, sb := data.Skills[a], data.Skills[b]
		if sa.Class != sb.Class {
			if sa.Class < sb.Class {
				return -1
			}
			return 1
		}
		if sa.LevelReq != sb.LevelReq {
			return sa.LevelReq - sb.LevelReq
		}
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})
	for _, id := range skillIDs {
		w.gameData.Skills = append(w.gameData.Skills, data.Skills[id])
	}

	mapIDs := make([]string, 0, len(data.Maps))
	for id := range data.Maps {
		mapIDs = append(mapIDs, id)
	}
	slices.Sort(mapIDs)
	for _, id := range mapIDs {
		w.gameData.Maps = append(w.gameData.Maps, data.Maps[id])
	}
	if len(mapIDs) > 0 {
		w.defaultMapID = mapIDs[0]
		if _, ok := data.Maps["refugio"]; ok {
			w.defaultMapID = "refugio"
		}
		w.townMapID = ""
		for _, id := range mapIDs {
			if data.Maps[id].Safe {
				w.townMapID = id
				if id == "refugio" {
					break
				}
			}
		}
		if w.townMapID == "" {
			w.townMapID = w.defaultMapID
		}
	}

	for _, mapID := range mapIDs {
		m := data.Maps[mapID]
		for _, spawn := range m.Spawns {
			spec, ok := data.Mobs[spawn.Mob]
			if !ok {
				continue // ya validado al cargar gamedata
			}
			id := mobIDBase + int64(len(w.mobs))
			mo := &mob{
				ID:     id,
				Spec:   spec,
				MapID:  mapID,
				X:      spawn.X,
				Z:      spawn.Z,
				SpawnX: spawn.X,
				SpawnZ: spawn.Z,
				HP:     spec.HP,
				State:  "idle",
			}
			w.mobs = append(w.mobs, mo)
			w.mobsByID[id] = mo
		}
	}
	w.nextMobID = mobIDBase + int64(len(w.mobs)) + 1000
	return w
}

// Inbox es el canal por el que las conexiones envían comandos al mundo.
func (w *World) Inbox() chan<- Command { return w.inbox }

// Run ejecuta el game loop hasta que ctx se cancele.
func (w *World) Run(ctx context.Context) {
	ticker := time.NewTicker(TickDuration)
	defer ticker.Stop()
	saveTicker := time.NewTicker(saveInterval)
	defer saveTicker.Stop()

	w.log.Info("mundo iniciado",
		"tick_rate", TickRate,
		"snapshot_rate", TickRate/SnapshotEvery,
		"maps", len(w.maps),
		"mobs", len(w.mobs),
	)

	dt := TickDuration.Seconds()
	for {
		select {
		case <-ctx.Done():
			w.saveAll()
			w.log.Info("mundo detenido")
			return
		case cmd := <-w.inbox:
			cmd.apply(w)
		case <-ticker.C:
			w.tick++
			w.simulate(dt)
			if w.tick%SnapshotEvery == 0 {
				w.broadcast()
			}
		case <-saveTicker.C:
			w.saveDirty()
		}
	}
}

func (w *World) addPlayer(j Join) {
	if _, exists := w.players[j.SessionID]; exists {
		return
	}
	// Un personaje solo puede estar online una vez (evita duplicar entidades).
	for _, p := range w.players {
		if p.ID == j.Character.ID {
			w.sendError(j.Out, "character_online", "Ese personaje ya está en línea")
			return
		}
	}

	c := j.Character
	mapID := c.MapID
	if _, ok := w.maps[mapID]; !ok && len(w.maps) > 0 {
		mapID = w.defaultMapID
	}
	p := &player{
		ID:        c.ID,
		SessionID: j.SessionID,
		Name:      c.Name,
		Class:     c.Class,
		MapID:     mapID,
		Level:     c.Level,
		XP:        c.XP,
		Str:       c.Str,
		Agi:       c.Agi,
		Vit:       c.Vit,
		Ene:       c.Ene,
		StatPts:   c.StatPoints,
		Gold:      c.Gold,
		Items:     append([]model.Item{}, c.Items...),
		Skills:    make(map[string]uint64),
		X:         clamp(c.X, -MapHalfSize, MapHalfSize),
		Z:         clamp(c.Z, -MapHalfSize, MapHalfSize),
		Anim:      "idle",
		Out:       j.Out,
	}
	p.recalcMax()
	p.HP = p.HPMax
	p.MP = p.MPMax
	w.players[p.SessionID] = p

	welcome, err := protocol.Encode("welcome", protocol.Welcome{
		Self:         p.entity(),
		Map:          p.MapID,
		TickRate:     TickRate,
		SnapshotRate: TickRate / SnapshotEvery,
		Character:    model.CharacterSummary{ID: c.ID, Name: c.Name, Class: c.Class, Level: c.Level},
		GameData:     w.gameData,
	})
	if err != nil {
		w.log.Error("no se pudo codificar el welcome", "err", err)
		return
	}
	w.send(p, welcome)
	w.sendInventory(p)
	w.sendSelfState(p)

	w.log.Info("jugador entró al mundo", "id", p.ID, "name", p.Name, "session", p.SessionID,
		"map", p.MapID, "level", p.Level, "items", len(p.Items), "gold", p.Gold)
}

func (w *World) removePlayer(sessionID int64) {
	p, ok := w.players[sessionID]
	if !ok {
		return
	}
	w.queueSave(p)
	delete(w.players, sessionID)
	w.log.Info("jugador salió del mundo", "id", p.ID, "name", p.Name)
}

func (w *World) setTarget(m Move) {
	p, ok := w.players[m.SessionID]
	if !ok {
		return
	}
	if math.IsNaN(m.X) || math.IsNaN(m.Z) || math.IsInf(m.X, 0) || math.IsInf(m.Z, 0) {
		return
	}
	p.AttackTarget = 0
	p.TargetX = clamp(m.X, -MapHalfSize, MapHalfSize)
	p.TargetZ = clamp(m.Z, -MapHalfSize, MapHalfSize)
	p.HasTarget = true
}

func (w *World) setAttack(a Attack) {
	p, ok := w.players[a.SessionID]
	if !ok || p.HP <= 0 {
		return
	}
	if w.isSafe(p.MapID) {
		w.sendError(p.Out, "safe_zone", "No podés atacar en la ciudad")
		return
	}
	if a.Target == 0 {
		p.AttackTarget = 0
		p.Anim = "idle"
		return
	}
	m, ok := w.mobsByID[a.Target]
	if !ok || m.State == "dead" || m.MapID != p.MapID {
		p.AttackTarget = 0
		p.Anim = "idle"
		return
	}
	p.AttackTarget = m.ID
	p.HasTarget = false
	p.Anim = "chase"
}

func (w *World) allocStat(a AllocStat) {
	p, ok := w.players[a.SessionID]
	if !ok || p.StatPts <= 0 {
		return
	}

	switch a.Stat {
	case "str":
		p.Str++
	case "agi":
		p.Agi++
	case "vit":
		p.Vit++
	case "ene":
		p.Ene++
	default:
		return
	}
	p.StatPts--

	oldHPMax, oldMPMax := p.HPMax, p.MPMax
	p.recalcMax()
	p.HP = min(p.HPMax, p.HP+(p.HPMax-oldHPMax))
	p.MP = min(p.MPMax, p.MP+(p.MPMax-oldMPMax))

	p.dirty = true
	w.sendSelfState(p)
	w.queueSave(p)
}

func (w *World) simulate(dt float64) {
	w.simulatePlayers(dt)
	w.simulateMobs(dt)
	w.simulateDrops()
}

func (w *World) simulatePlayers(dt float64) {
	for _, p := range w.players {
		if p.HP <= 0 {
			continue
		}

		if p.AttackTarget != 0 {
			m := w.mobsByID[p.AttackTarget]
			if m == nil || m.State == "dead" || m.MapID != p.MapID {
				p.AttackTarget = 0
				p.Anim = "idle"
			} else {
				dx := m.X - p.X
				dz := m.Z - p.Z
				dist := math.Hypot(dx, dz)
				attackRange := w.attackRange(p) + 0.5

				if dist > attackRange {
					p.Anim = "run"
					if dist > 0.001 {
						p.Rot = math.Atan2(dx, dz)
						step := PlayerSpeed * dt
						if step > dist {
							step = dist
						}
						p.X += dx / dist * step
						p.Z += dz / dist * step
						p.dirty = true
					}
				} else {
					p.Anim = "attack"
					p.Rot = math.Atan2(dx, dz)
					if w.tick >= p.NextAttackTick {
						w.playerAttack(p, m)
						interval := w.attackInterval(p)
						ticks := uint64(interval / TickDuration)
						if ticks < 1 {
							ticks = 1
						}
						p.NextAttackTick = w.tick + ticks
					}
				}
			}
		} else if p.HasTarget {
			dx := p.TargetX - p.X
			dz := p.TargetZ - p.Z
			dist := math.Hypot(dx, dz)
			if dist < 0.001 {
				p.X, p.Z = p.TargetX, p.TargetZ
				p.HasTarget = false
				p.Anim = "idle"
			} else {
				p.Rot = math.Atan2(dx, dz)
				step := PlayerSpeed * dt
				if step >= dist {
					p.X, p.Z = p.TargetX, p.TargetZ
					p.HasTarget = false
					p.Anim = "idle"
				} else {
					p.X += dx / dist * step
					p.Z += dz / dist * step
					p.Anim = "run"
				}
				p.dirty = true
			}
		} else {
			p.Anim = "idle"
		}

		if w.tick-p.LastCombatTick >= combatCooldownTicks && w.tick%regenIntervalTicks == 0 {
			changed := false
			if p.HP < p.HPMax {
				p.HP = min(p.HPMax, p.HP+max(1, p.HPMax/100))
				changed = true
			}
			if p.MP < p.MPMax {
				p.MP = min(p.MPMax, p.MP+max(1, p.MPMax/50))
				changed = true
			}
			if changed {
				w.sendSelfState(p)
			}
		}
	}
}

func (w *World) simulateMobs(dt float64) {
	for _, m := range w.mobs {
		if m.State == "dead" {
			if m.Temporary {
				continue
			}
			if w.tick >= m.Respawn {
				m.HP = m.Spec.HP
				m.X, m.Z = m.SpawnX, m.SpawnZ
				m.State = "idle"
				m.Target = 0
				m.Rot = 0
				m.Summoned = false
				m.Enraged = false
			}
			continue
		}

		target := w.players[m.Target]
		if target == nil || target.HP <= 0 || target.MapID != m.MapID {
			target = nil
			m.Target = 0
		}

		if m.Target == 0 && !m.Spec.Passive {
			if id, ok := w.nearestPlayer(m); ok {
				m.Target = id
				target = w.players[id]
			}
		}

		if target == nil {
			if math.Hypot(m.X-m.SpawnX, m.Z-m.SpawnZ) > 0.05 {
				m.X, m.Z = m.SpawnX, m.SpawnZ
				m.HP = m.Spec.HP
				m.Rot = 0
			}
			m.State = "idle"
			continue
		}

		dx := target.X - m.X
		dz := target.Z - m.Z
		dist := math.Hypot(dx, dz)

		if math.Hypot(m.X-m.SpawnX, m.Z-m.SpawnZ) > mobLeash || dist > mobLeash*1.5 {
			m.X, m.Z = m.SpawnX, m.SpawnZ
			m.HP = m.Spec.HP
			m.Target = 0
			m.Rot = 0
			m.State = "idle"
			continue
		}

		if dist <= m.Spec.AttackRange+0.4 {
			m.State = "attack"
			m.Rot = math.Atan2(dx, dz)
			if w.tick >= m.NextAtk {
				w.mobAttack(m, target)
				ticks := uint64(m.Spec.AttackIntervalMS * TickRate / 1000)
				if ticks < 1 {
					ticks = 1
				}
				m.NextAtk = w.tick + ticks
			}
		} else {
			m.State = "chase"
			m.Rot = math.Atan2(dx, dz)
			if dist > 0.001 {
				speed := m.Spec.Speed
				if m.Enraged && m.Spec.EnrageSpeedMult > 0 {
					speed *= m.Spec.EnrageSpeedMult
				}
				step := speed * dt
				if step > dist {
					step = dist
				}
				m.X += dx / dist * step
				m.Z += dz / dist * step
			}
		}
	}
}

func (w *World) nearestPlayer(m *mob) (int64, bool) {
	best := m.Spec.AggroRadius
	if best <= 0 {
		return 0, false
	}
	var bestID int64
	for _, p := range w.players {
		if p.HP <= 0 || p.MapID != m.MapID {
			continue
		}
		d := math.Hypot(p.X-m.X, p.Z-m.Z)
		if d <= best {
			best = d
			bestID = p.SessionID
		}
	}
	return bestID, bestID != 0
}

func (w *World) playerAttack(p *player, m *mob) {
	minD, maxD := w.damageRange(p)
	if mult := w.damageMultiplier(p); mult > 1 {
		minD = max(1, int(math.Round(float64(minD)*mult)))
		maxD = max(1, int(math.Round(float64(maxD)*mult)))
	}
	damage := combat.RollDamage(minD, maxD, w.rng.Float64())
	damage = combat.ApplyDefense(damage, m.Spec.Defense)
	crit := combat.IsCrit(w.rng.Float64())
	if crit {
		damage = combat.CritDamage(damage)
	}

	m.HP -= damage
	p.LastCombatTick = w.tick
	p.dirty = true

	if m.Target == 0 {
		m.Target = p.SessionID
		m.State = "chase"
	}

	w.sendDamage(p.MapID, p.ID, m.ID, damage, crit, "out")
	w.checkBossMechanics(m)
	if m.HP <= 0 {
		w.killMob(m, p)
	}
}

func (w *World) mobAttack(m *mob, p *player) {
	if combat.Dodged(p.stats(), w.rng.Float64()*100.0) {
		w.sendDamage(m.MapID, m.ID, p.ID, 0, false, "dodge")
		return
	}

	damage := combat.RollDamage(m.Spec.DamageMin, m.Spec.DamageMax, w.rng.Float64())
	if m.Enraged && m.Spec.EnrageDmgMult > 0 {
		damage = max(1, int(math.Round(float64(damage)*m.Spec.EnrageDmgMult)))
	}
	damage = combat.ApplyDefense(damage, w.totalDefense(p))
	crit := combat.IsCrit(w.rng.Float64())
	if crit {
		damage = combat.CritDamage(damage)
	}

	p.HP -= damage
	p.LastCombatTick = w.tick
	w.sendDamage(m.MapID, m.ID, p.ID, damage, crit, "in")

	if p.HP <= 0 {
		w.killPlayer(p)
		return
	}
	w.sendSelfState(p)
}

func (w *World) killMob(m *mob, killer *player) {
	m.HP = 0
	m.State = "dead"
	m.Target = 0
	respawnTicks := uint64(m.Spec.RespawnSeconds * TickRate)
	if respawnTicks < 1 {
		respawnTicks = 1
	}
	m.Respawn = w.tick + respawnTicks

	gained := combat.XPForKill(m.Spec.XP, m.Spec.Level, killer.Level)
	killer.XP += gained
	killer.dirty = true

	gold := w.rollGold(killer, m)
	w.spawnLoot(m)

	leveled := false
	for {
		need := combat.XPToNext(killer.Level)
		if killer.XP < need {
			break
		}
		killer.XP -= need
		killer.Level++
		killer.StatPts += 5
		leveled = true
	}
	if leveled {
		killer.recalcMax()
		killer.HP = killer.HPMax
		killer.MP = killer.MPMax
		if data, err := protocol.Encode("levelup", protocol.LevelUp{
			Level:      killer.Level,
			StatPoints: killer.StatPts,
		}); err == nil {
			w.send(killer, data)
		}
	}

	w.broadcastEvent(m.MapID, "mob_dead", protocol.MobDead{
		ID:     m.ID,
		XP:     gained,
		Gold:   gold,
		Killer: killer.ID,
	})
	w.sendSelfState(killer)
	w.sendInventory(killer)
	w.queueSave(killer)

	w.log.Info("mob eliminado",
		"mob", m.Spec.ID, "mob_id", m.ID, "map", m.MapID,
		"killer", killer.Name, "xp", gained, "gold", gold, "level", killer.Level)
}

func (w *World) killPlayer(p *player) {
	p.HP = 0
	remainingXP, lost := combat.DeathPenalty(p.XP, p.Level)
	p.XP = remainingXP

	respawnMap := w.townMapID
	if _, ok := w.maps[respawnMap]; !ok {
		respawnMap = p.MapID
	}
	sx, sz := w.mapSpawn(respawnMap)
	p.MapID = respawnMap
	p.X, p.Z = sx, sz
	p.HasTarget = false
	p.AttackTarget = 0
	p.Anim = "idle"
	p.Rot = 0
	p.HP = max(1, p.HPMax/2)
	p.MP = p.MPMax / 2
	p.NextPotionTick = w.tick
	p.LastCombatTick = w.tick
	p.dirty = true

	// Los mobs que estaban peleando con el jugador vuelven a su spawn.
	for _, m := range w.mobs {
		if m.Target == p.SessionID {
			m.Target = 0
			m.State = "idle"
			m.HP = m.Spec.HP
			m.X, m.Z = m.SpawnX, m.SpawnZ
			m.Rot = 0
		}
	}

	if data, err := protocol.Encode("death", protocol.Death{XPLost: lost}); err == nil {
		w.send(p, data)
	}
	if data, err := protocol.Encode("teleport", protocol.Teleport{
		Map: p.MapID, X: p.X, Y: 0, Z: p.Z,
	}); err == nil {
		w.send(p, data)
	}
	w.sendSelfState(p)
	w.queueSave(p)

	w.log.Info("jugador murió", "name", p.Name, "xp_lost", lost, "level", p.Level, "map", p.MapID)
}

func (w *World) selfState(p *player) protocol.SelfState {
	hpPotions, mpPotions := w.potionCounts(p)
	minD, maxD := w.damageRange(p)
	return protocol.SelfState{
		HP:         p.HP,
		HPMax:      p.HPMax,
		MP:         p.MP,
		MPMax:      p.MPMax,
		XP:         p.XP,
		XPNext:     combat.XPToNext(p.Level),
		Level:      p.Level,
		Str:        p.Str,
		Agi:        p.Agi,
		Vit:        p.Vit,
		Ene:        p.Ene,
		StatPoints: p.StatPts,
		HPPotions:  hpPotions,
		MPPotions:  mpPotions,
		AttackMin:  minD,
		AttackMax:  maxD,
		Defense:    w.totalDefense(p),
		IntervalMS: int(w.attackInterval(p).Milliseconds()),
	}
}

func (w *World) sendSelfState(p *player) {
	data, err := protocol.Encode("self_state", w.selfState(p))
	if err != nil {
		w.log.Error("no se pudo codificar self_state", "err", err)
		return
	}
	w.send(p, data)
}

func (w *World) sendDamage(mapID string, src, dst int64, amount int, crit bool, kind string) {
	w.broadcastEvent(mapID, "dmg", protocol.Damage{
		Src: src, Dst: dst, Amount: amount, Crit: crit, Kind: kind,
	})
}

func (w *World) broadcastEvent(mapID, mtype string, payload any) {
	data, err := protocol.Encode(mtype, payload)
	if err != nil {
		w.log.Error("no se pudo codificar evento", "tipo", mtype, "err", err)
		return
	}
	for _, p := range w.players {
		if p.MapID == mapID {
			w.send(p, data)
		}
	}
}

func (w *World) broadcast() {
	byMap := make(map[string][]*player)
	for _, p := range w.players {
		byMap[p.MapID] = append(byMap[p.MapID], p)
	}
	for mapID, players := range byMap {
		data, err := protocol.Encode("snapshot", protocol.Snapshot{
			Tick: w.tick,
			Ents: w.entitiesInMap(mapID),
		})
		if err != nil {
			w.log.Error("no se pudo codificar el snapshot", "err", err)
			continue
		}
		for _, p := range players {
			w.send(p, data)
		}
	}
}

// entities devuelve todas las entidades del mundo (tests).
func (w *World) entities() []protocol.Entity {
	ents := make([]protocol.Entity, 0, len(w.players)+len(w.mobs)+len(w.drops))
	for _, p := range w.players {
		ents = append(ents, p.entity())
	}
	for _, m := range w.mobs {
		if m.State == "dead" {
			continue
		}
		ents = append(ents, m.entity())
	}
	for _, d := range w.drops {
		ents = append(ents, d.entity(w.items))
	}
	sortEntities(ents)
	return ents
}

func (w *World) entitiesInMap(mapID string) []protocol.Entity {
	ents := make([]protocol.Entity, 0, len(w.players)+len(w.mobs)+len(w.drops))
	for _, p := range w.players {
		if p.MapID == mapID {
			ents = append(ents, p.entity())
		}
	}
	for _, m := range w.mobs {
		if m.MapID != mapID || m.State == "dead" {
			continue
		}
		ents = append(ents, m.entity())
	}
	for _, d := range w.drops {
		if d.MapID == mapID {
			ents = append(ents, d.entity(w.items))
		}
	}
	sortEntities(ents)
	return ents
}

func sortEntities(ents []protocol.Entity) {
	slices.SortFunc(ents, func(a, b protocol.Entity) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
}

func (w *World) saveDirty() {
	for _, p := range w.players {
		if p.dirty {
			w.queueSave(p)
		}
	}
}

func (w *World) saveAll() {
	for _, p := range w.players {
		w.queueSave(p)
	}
}

// queueSave encola el estado del personaje sin bloquear el game loop.
func (w *World) queueSave(p *player) {
	if w.saveCh == nil {
		return
	}
	items := make([]model.Item, len(p.Items))
	copy(items, p.Items)

	state := model.CharacterState{
		CharacterID: p.ID,
		MapID:       p.MapID,
		X:           p.X,
		Y:           0,
		Z:           p.Z,
		Level:       p.Level,
		XP:          p.XP,
		Str:         p.Str,
		Agi:         p.Agi,
		Vit:         p.Vit,
		Ene:         p.Ene,
		StatPoints:  p.StatPts,
		Gold:        p.Gold,
		Items:       items,
	}
	select {
	case w.saveCh <- state:
		p.dirty = false
	default:
		w.log.Warn("cola de guardado llena: se reintentará en el próximo ciclo", "id", p.ID)
	}
}

func (w *World) send(p *player, data []byte) {
	select {
	case p.Out <- data:
	default:
		w.log.Warn("buzón lleno: mensaje descartado", "player", p.Name)
	}
}

func (w *World) sendError(out chan<- []byte, code, msg string) {
	data, err := protocol.Encode("err", protocol.Error{Code: code, Msg: msg})
	if err != nil {
		return
	}
	select {
	case out <- data:
	default:
	}
}

// damageMultiplier aplica el buff de Grito de Guerra (F5).
func (w *World) damageMultiplier(p *player) float64 {
	if p.BuffPercent > 0 && w.tick < p.BuffUntil {
		return 1.0 + float64(p.BuffPercent)/100.0
	}
	return 1.0
}

func (w *World) checkBossMechanics(m *mob) {
	if m.State == "dead" {
		return
	}
	if m.Spec.EnrageAtPercent > 0 && !m.Enraged &&
		m.HP <= m.Spec.HP*m.Spec.EnrageAtPercent/100 {
		m.Enraged = true
		w.log.Info("mob enfurecido", "mob", m.Spec.ID, "mob_id", m.ID)
	}
	if m.Spec.SummonAtPercent > 0 && !m.Summoned &&
		m.HP <= m.Spec.HP*m.Spec.SummonAtPercent/100 {
		m.Summoned = true
		w.summonAdds(m)
	}
}

func (w *World) summonAdds(boss *mob) {
	spec, ok := w.mobSpecs[boss.Spec.SummonMob]
	if !ok {
		return
	}
	for i := 0; i < boss.Spec.SummonCount; i++ {
		id := w.nextMobID
		w.nextMobID++
		x := clamp(boss.X+(w.rng.Float64()*2-1)*2.0, -MapHalfSize, MapHalfSize)
		z := clamp(boss.Z+(w.rng.Float64()*2-1)*2.0, -MapHalfSize, MapHalfSize)
		add := &mob{
			ID:        id,
			Spec:      spec,
			MapID:     boss.MapID,
			X:         x,
			Z:         z,
			SpawnX:    x,
			SpawnZ:    z,
			HP:        spec.HP,
			State:     "chase",
			Target:    boss.Target,
			Temporary: true,
		}
		if add.Target == 0 {
			add.State = "idle"
		}
		w.mobs = append(w.mobs, add)
		w.mobsByID[id] = add
	}
	w.log.Info("boss invocó esbirros", "boss", boss.Spec.ID, "count", boss.Spec.SummonCount)
}

func (w *World) isSafe(mapID string) bool {
	m, ok := w.maps[mapID]
	return ok && m.Safe
}

func (w *World) mapSpawn(mapID string) (float64, float64) {
	if m, ok := w.maps[mapID]; ok {
		return m.PlayerSpawn.X, m.PlayerSpawn.Z
	}
	return 0, 0
}

func clamp(v, min, max float64) float64 {
	return math.Max(min, math.Min(max, v))
}
