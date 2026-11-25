// Package world contiene el estado y la simulación autoritativa del mundo.
//
// El mundo es propiedad exclusiva de UNA goroutine (el game loop): el resto
// del servidor le envía comandos por canal y él emite snapshots. Esto evita
// race conditions por diseño (ver docs/03-arquitectura.md).
package world

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"time"

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
	// MapHalfSize limita el área jugable del mapa plano de F1.
	MapHalfSize = 19.0
	// SpawnRadius es la distancia al centro donde aparecen los jugadores.
	SpawnRadius = 3.0
)

// Command es una acción que las conexiones envían al game loop.
// La interfaz tiene un método no exportado: solo este paquete puede crear
// comandos válidos.
type Command interface {
	apply(*World)
}

// Join agrega un jugador al mundo.
type Join struct {
	SessionID int64
	User      string
	Out       chan<- []byte
}

func (j Join) apply(w *World) { w.addPlayer(j) }

// Move cambia el punto destino de un jugador.
type Move struct {
	SessionID int64
	X, Z      float64
}

func (m Move) apply(w *World) { w.setTarget(m) }

// Leave quita a un jugador del mundo.
type Leave struct {
	SessionID int64
}

func (l Leave) apply(w *World) { delete(w.players, l.SessionID) }

type player struct {
	ID   int64
	Name string
	X, Z float64
	// TargetX/TargetZ es el destino actual (click-to-move por ahora).
	TargetX, TargetZ float64
	HasTarget        bool
	Rot              float64
	Anim             string
	Out              chan<- []byte
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

// World es el estado del mundo y su game loop.
type World struct {
	log     *slog.Logger
	players map[int64]*player
	inbox   chan Command
	tick    uint64
}

// New crea un mundo vacío.
func New(log *slog.Logger) *World {
	return &World{
		log:     log,
		players: make(map[int64]*player),
		inbox:   make(chan Command, 256),
	}
}

// Inbox es el canal por el que las conexiones envían comandos al mundo.
func (w *World) Inbox() chan<- Command { return w.inbox }

// Run ejecuta el game loop hasta que ctx se cancele.
func (w *World) Run(ctx context.Context) {
	ticker := time.NewTicker(TickDuration)
	defer ticker.Stop()

	w.log.Info("mundo iniciado",
		"tick_rate", TickRate,
		"snapshot_rate", TickRate/SnapshotEvery,
		"map", "llanuras",
	)

	dt := TickDuration.Seconds()
	for {
		select {
		case <-ctx.Done():
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
		}
	}
}

func (w *World) addPlayer(j Join) {
	if _, exists := w.players[j.SessionID]; exists {
		return
	}

	// Spawn en espiral áurea alrededor del centro para no superponerse.
	angle := float64(j.SessionID) * 2.399963229728653
	p := &player{
		ID:   j.SessionID,
		Name: w.uniqueName(j.User),
		X:    math.Cos(angle) * SpawnRadius,
		Z:    math.Sin(angle) * SpawnRadius,
		Anim: "idle",
		Out:  j.Out,
	}
	w.players[p.ID] = p

	welcome, err := protocol.Encode("welcome", protocol.Welcome{
		Self:         p.entity(),
		Map:          "llanuras",
		TickRate:     TickRate,
		SnapshotRate: TickRate / SnapshotEvery,
	})
	if err != nil {
		w.log.Error("no se pudo codificar el welcome", "err", err)
		return
	}
	w.send(p, welcome)

	w.log.Info("jugador entró al mundo", "id", p.ID, "name", p.Name)
}

func (w *World) uniqueName(base string) string {
	name := base
	for i := 2; w.nameTaken(name); i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return name
}

func (w *World) nameTaken(name string) bool {
	for _, p := range w.players {
		if p.Name == name {
			return true
		}
	}
	return false
}

func (w *World) setTarget(m Move) {
	p, ok := w.players[m.SessionID]
	if !ok {
		return
	}
	if math.IsNaN(m.X) || math.IsNaN(m.Z) || math.IsInf(m.X, 0) || math.IsInf(m.Z, 0) {
		return
	}
	p.TargetX = clamp(m.X, -MapHalfSize, MapHalfSize)
	p.TargetZ = clamp(m.Z, -MapHalfSize, MapHalfSize)
	p.HasTarget = true
}

// simulate avanza el mundo dt segundos. dt es fijo (1/TickRate).
func (w *World) simulate(dt float64) {
	for _, p := range w.players {
		if !p.HasTarget {
			p.Anim = "idle"
			continue
		}

		dx := p.TargetX - p.X
		dz := p.TargetZ - p.Z
		dist := math.Hypot(dx, dz)
		if dist < 0.001 {
			p.X, p.Z = p.TargetX, p.TargetZ
			p.HasTarget = false
			p.Anim = "idle"
			continue
		}

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
	ents := make([]protocol.Entity, 0, len(w.players))
	for _, p := range w.players {
		ents = append(ents, p.entity())
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

func (w *World) send(p *player, data []byte) {
	select {
	case p.Out <- data:
	default:
		// Cliente lento o desconectándose: descartar evita bloquear el mundo.
		w.log.Warn("buzón lleno: mensaje descartado", "player", p.Name)
	}
}

func clamp(v, min, max float64) float64 {
	return math.Max(min, math.Min(max, v))
}
