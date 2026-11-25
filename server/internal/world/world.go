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
	// MapHalfSize limita el área jugable del mapa plano.
	MapHalfSize = 19.0
	// DefaultMapID identifica el mapa cuando no hay gamedata cargada.
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
	// startingPotions es la cantidad inicial (y al revivir) de cada poción.
	startingPotions = 5
	// potionHealPercent cura el 40% del máximo.
	potionHealPercent = 40
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

	HPPotions, MPPotions int
	NextPotionTick       uint64

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
	ID      int64
	Spec    gamedata.Mob
	X, Z    float64
	SpawnX  float64
	SpawnZ  float64
	HP      int
	State   string // idle | chase | attack | dead
	Target  int64  // SessionID del jugador objetivo
	Rot     float64
	NextAtk uint64
	Respawn uint64
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
	log      *slog.Logger
	players  map[int64]*player // por SessionID
	mobs     []*mob
	mobsByID map[int64]*mob
	mapID    string
	inbox    chan Command
	saveCh   chan<- model.CharacterState
	tick     uint64
	rng      *rand.Rand
}

// New crea un mundo vacío. saveCh recibe los estados a persistir (nil = sin
// persistencia, útil en tests). data puede ser nil: el mundo corre sin mobs.
func New(log *slog.Logger, saveCh chan<- model.CharacterState, data *gamedata.Data) *World {
	w := &World{
		log:      log,
		players:  make(map[int64]*player),
		mobsByID: make(map[int64]*mob),
		mapID:    DefaultMapID,
		inbox:    make(chan Command, 256),
		saveCh:   saveCh,
		rng: rand.New(rand.NewPCG(
			uint64(time.Now().UnixNano()),
			0x9E3779B97F4A7C15,
		)),
	}
	if data != nil {
		if data.MapID != "" {
			w.mapID = data.MapID
		}
		for i, spawn := range data.Spawns {
			spec, ok := data.Mobs[spawn.Mob]
			if !ok {
				continue // ya validado al cargar gamedata
			}
			m := &mob{
				ID:     mobIDBase + int64(i),
				Spec:   spec,
				X:      spawn.X,
				Z:      spawn.Z,
				SpawnX: spawn.X,
				SpawnZ: spawn.Z,
				HP:     spec.HP,
				State:  "idle",
			}
			w.mobs = append(w.mobs, m)
			w.mobsByID[m.ID] = m
		}
	}
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
		"map", w.mapID,
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
	p := &player{
		ID:             c.ID,
		SessionID:      j.SessionID,
		Name:           c.Name,
		Class:          c.Class,
		Level:          c.Level,
		XP:             c.XP,
		Str:            c.Str,
		Agi:            c.Agi,
		Vit:            c.Vit,
		Ene:            c.Ene,
		StatPts:        c.StatPoints,
		Gold:           c.Gold,
		HPPotions:      startingPotions,
		MPPotions:      startingPotions,
		X:              c.X,
		Z:              c.Z,
		Anim:           "idle",
		Out:            j.Out,
		LastCombatTick: 0,
	}
	p.recalcMax()
	p.HP = p.HPMax
	p.MP = p.MPMax
	w.players[p.SessionID] = p

	welcome, err := protocol.Encode("welcome", protocol.Welcome{
		Self:         p.entity(),
		Map:          w.mapID,
		TickRate:     TickRate,
		SnapshotRate: TickRate / SnapshotEvery,
		Character:    model.CharacterSummary{ID: c.ID, Name: c.Name, Class: c.Class, Level: c.Level},
	})
	if err != nil {
		w.log.Error("no se pudo codificar el welcome", "err", err)
		return
	}
	w.send(p, welcome)
	w.sendSelfState(p)

	w.log.Info("jugador entró al mundo", "id", p.ID, "name", p.Name, "session", p.SessionID, "level", p.Level)
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
	if a.Target == 0 {
		p.AttackTarget = 0
		p.Anim = "idle"
		return
	}
	m, ok := w.mobsByID[a.Target]
	if !ok || m.State == "dead" {
		p.AttackTarget = 0
		p.Anim = "idle"
		return
	}
	p.AttackTarget = m.ID
	p.HasTarget = false
	p.Anim = "chase"
}

func (w *World) usePotion(c Potion) {
	p, ok := w.players[c.SessionID]
	if !ok || p.HP <= 0 {
		return
	}
	if w.tick < p.NextPotionTick {
		return
	}

	switch c.Kind {
	case "hp":
		if p.HPPotions <= 0 {
			return
		}
		heal := max(1, p.HPMax*potionHealPercent/100)
		p.HP = min(p.HPMax, p.HP+heal)
		p.HPPotions--
	case "mp":
		if p.MPPotions <= 0 {
			return
		}
		heal := max(1, p.MPMax*potionHealPercent/100)
		p.MP = min(p.MPMax, p.MP+heal)
		p.MPPotions--
	default:
		return
	}

	p.NextPotionTick = w.tick + potionCooldownTicks
	w.sendSelfState(p)
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
	// Los puntos de VIT/ENE aumentan el máximo y también la vida/maná actual.
	p.HP = min(p.HPMax, p.HP+(p.HPMax-oldHPMax))
	p.MP = min(p.MPMax, p.MP+(p.MPMax-oldMPMax))

	p.dirty = true
	w.sendSelfState(p)
	w.queueSave(p) // el progreso no debe depender del próximo guardado periódico
}

func (w *World) simulate(dt float64) {
	w.simulatePlayers(dt)
	w.simulateMobs(dt)
}

func (w *World) simulatePlayers(dt float64) {
	for _, p := range w.players {
		if p.HP <= 0 {
			continue
		}

		if p.AttackTarget != 0 {
			m := w.mobsByID[p.AttackTarget]
			if m == nil || m.State == "dead" {
				p.AttackTarget = 0
				p.Anim = "idle"
			} else {
				dx := m.X - p.X
				dz := m.Z - p.Z
				dist := math.Hypot(dx, dz)
				attackRange := combat.AttackRange(p.Class) + 0.5

				if dist > attackRange {
					// Auto-approach: el servidor acerca al jugador a su objetivo.
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
						interval := combat.AttackInterval(p.stats(), p.Class)
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

		// Regeneración fuera de combate (5 s sin pegar ni recibir).
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
			if w.tick >= m.Respawn {
				m.HP = m.Spec.HP
				m.X, m.Z = m.SpawnX, m.SpawnZ
				m.State = "idle"
				m.Target = 0
				m.Rot = 0
			}
			continue
		}

		target := w.players[m.Target]
		if target == nil || target.HP <= 0 {
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
			// Sin objetivo: volver al spawn (p. ej. el jugador se desconectó).
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

		// Leash: si se alejó demasiado del spawn o del objetivo, resetea.
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
				step := m.Spec.Speed * dt
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
		if p.HP <= 0 {
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
	minD, maxD := combat.DamageRange(p.stats(), p.Class)
	damage := combat.RollDamage(minD, maxD, w.rng.Float64())
	damage = combat.ApplyDefense(damage, m.Spec.Defense)
	crit := combat.IsCrit(w.rng.Float64())
	if crit {
		damage = combat.CritDamage(damage)
	}

	m.HP -= damage
	p.LastCombatTick = w.tick
	p.dirty = true

	// Los pasivos también retalián al ser atacados.
	if m.Target == 0 {
		m.Target = p.SessionID
		m.State = "chase"
	}

	w.sendDamage(p.ID, m.ID, damage, crit, "out")
	if m.HP <= 0 {
		w.killMob(m, p)
	}
}

func (w *World) mobAttack(m *mob, p *player) {
	if combat.Dodged(p.stats(), w.rng.Float64()*100.0) {
		w.sendDamage(m.ID, p.ID, 0, false, "dodge")
		return
	}

	damage := combat.RollDamage(m.Spec.DamageMin, m.Spec.DamageMax, w.rng.Float64())
	damage = combat.ApplyDefense(damage, combat.Defense(p.stats()))
	crit := combat.IsCrit(w.rng.Float64())
	if crit {
		damage = combat.CritDamage(damage)
	}

	p.HP -= damage
	p.LastCombatTick = w.tick
	w.sendDamage(m.ID, p.ID, damage, crit, "in")

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

	w.broadcastEvent("mob_dead", protocol.MobDead{
		ID:     m.ID,
		XP:     gained,
		Killer: killer.ID,
	})
	w.sendSelfState(killer)
	w.queueSave(killer)

	w.log.Info("mob eliminado",
		"mob", m.Spec.ID, "mob_id", m.ID,
		"killer", killer.Name, "xp", gained, "level", killer.Level)
}

func (w *World) killPlayer(p *player) {
	p.HP = 0
	remainingXP, lost := combat.DeathPenalty(p.XP, p.Level)
	p.XP = remainingXP

	p.X, p.Z = 0, 0
	p.HasTarget = false
	p.AttackTarget = 0
	p.Anim = "idle"
	p.Rot = 0
	p.HP = max(1, p.HPMax/2)
	p.MP = p.MPMax / 2
	p.HPPotions = startingPotions
	p.MPPotions = startingPotions
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
		Map: w.mapID, X: p.X, Y: 0, Z: p.Z,
	}); err == nil {
		w.send(p, data)
	}
	w.sendSelfState(p)
	w.queueSave(p)

	w.log.Info("jugador murió", "name", p.Name, "xp_lost", lost, "level", p.Level)
}

func (w *World) selfState(p *player) protocol.SelfState {
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
		HPPotions:  p.HPPotions,
		MPPotions:  p.MPPotions,
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

func (w *World) sendDamage(src, dst int64, amount int, crit bool, kind string) {
	w.broadcastEvent("dmg", protocol.Damage{
		Src: src, Dst: dst, Amount: amount, Crit: crit, Kind: kind,
	})
}

func (w *World) broadcastEvent(mtype string, payload any) {
	data, err := protocol.Encode(mtype, payload)
	if err != nil {
		w.log.Error("no se pudo codificar evento", "tipo", mtype, "err", err)
		return
	}
	for _, p := range w.players {
		w.send(p, data)
	}
}

func (w *World) broadcast() {
	data, err := protocol.Encode("snapshot", protocol.Snapshot{
		Tick: w.tick,
		Ents: w.entities(),
	})
	if err != nil {
		w.log.Error("no se pudo codificar el snapshot", "err", err)
		return
	}
	for _, p := range w.players {
		w.send(p, data)
	}
}

// entities devuelve una copia ordenada de las entidades (para tests y snapshots).
func (w *World) entities() []protocol.Entity {
	ents := make([]protocol.Entity, 0, len(w.players)+len(w.mobs))
	for _, p := range w.players {
		ents = append(ents, p.entity())
	}
	for _, m := range w.mobs {
		if m.State == "dead" {
			continue
		}
		ents = append(ents, m.entity())
	}
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
	return ents
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
	state := model.CharacterState{
		CharacterID: p.ID,
		MapID:       w.mapID,
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
		// Cliente lento o desconectándose: descartar evita bloquear el mundo.
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

func clamp(v, min, max float64) float64 {
	return math.Max(min, math.Min(max, v))
}
