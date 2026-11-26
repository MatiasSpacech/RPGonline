package world

// Portales entre mapas (F5).

import (
	"math"

	"munext/server/internal/gamedata"
	"munext/server/internal/protocol"
)

// UsePortal viaja a otro mapa usando un portal del mapa actual.
type UsePortal struct {
	SessionID int64
	PortalID  string
}

func (c UsePortal) apply(w *World) { w.usePortal(c) }

func (w *World) usePortal(c UsePortal) {
	p, ok := w.players[c.SessionID]
	if !ok || p.HP <= 0 {
		return
	}
	current, ok := w.maps[p.MapID]
	if !ok {
		return
	}

	var portal *gamedata.Portal
	for i := range current.Portals {
		if current.Portals[i].ID == c.PortalID {
			portal = &current.Portals[i]
			break
		}
	}
	if portal == nil {
		w.sendError(p.Out, "no_portal", "Ese portal no existe")
		return
	}
	if math.Hypot(p.X-portal.X, p.Z-portal.Z) > 3.0 {
		w.sendError(p.Out, "too_far", "Acercate al portal")
		return
	}
	if p.Level < portal.LevelReq {
		w.sendError(p.Out, "level_req", "Necesitás nivel mayor para entrar")
		return
	}
	target, ok := w.maps[portal.Target]
	if !ok {
		return
	}

	p.MapID = target.ID
	p.X = clamp(portal.TargetX, -MapHalfSize, MapHalfSize)
	p.Z = clamp(portal.TargetZ, -MapHalfSize, MapHalfSize)
	p.HasTarget = false
	p.AttackTarget = 0
	p.Anim = "idle"
	p.Rot = 0
	p.dirty = true

	if data, err := protocol.Encode("teleport", protocol.Teleport{
		Map: p.MapID, X: p.X, Y: 0, Z: p.Z,
	}); err == nil {
		w.send(p, data)
	}
	w.sendSelfState(p)
	w.queueSave(p)

	w.log.Info("jugador cambió de mapa", "name", p.Name, "map", p.MapID, "portal", portal.ID)
}
