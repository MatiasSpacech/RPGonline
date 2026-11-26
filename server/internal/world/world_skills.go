package world

// Skills de clase (F5): maná, cooldowns, daño/AoE/heal/buff/teleport.

import (
	"math"

	"munext/server/internal/combat"
	"munext/server/internal/gamedata"
	"munext/server/internal/protocol"
)

// UseSkill lanza una skill. TargetID es opcional (mob); X/Z se usan para
// teletransporte.
type UseSkill struct {
	SessionID int64
	SkillID   string
	TargetID  int64
	X, Z      float64
}

func (c UseSkill) apply(w *World) { w.useSkill(c) }

func (w *World) useSkill(c UseSkill) {
	p, ok := w.players[c.SessionID]
	if !ok || p.HP <= 0 {
		return
	}
	if w.isSafe(p.MapID) {
		w.sendError(p.Out, "safe_zone", "No podés usar skills en la ciudad")
		return
	}

	skill, ok := w.skills[c.SkillID]
	if !ok || skill.Class != p.Class {
		w.sendError(p.Out, "invalid_skill", "Skill inválida")
		return
	}
	if p.Level < skill.LevelReq {
		w.sendError(p.Out, "level_req", "Todavía no desbloqueaste esa skill")
		return
	}
	if p.Skills == nil {
		p.Skills = make(map[string]uint64)
	}
	if w.tick < p.Skills[skill.ID] {
		w.sendError(p.Out, "cooldown", "La skill está en cooldown")
		return
	}
	if p.MP < skill.ManaCost {
		w.sendError(p.Out, "no_mana", "No tenés maná suficiente")
		return
	}

	// Validar objetivo/destino antes de consumir maná.
	var target *mob
	if skill.Kind == "single" || skill.Kind == "aoe" {
		target = w.resolveSkillTarget(p, c.TargetID, skill.Range)
		if target == nil {
			w.sendError(p.Out, "no_target", "No hay un objetivo en rango")
			return
		}
	}

	destX, destZ := c.X, c.Z
	if skill.Kind == "teleport" {
		if destX == 0 && destZ == 0 {
			destX = p.X + math.Sin(p.Rot)*skill.TeleportDistance
			destZ = p.Z + math.Cos(p.Rot)*skill.TeleportDistance
		}
		dx := destX - p.X
		dz := destZ - p.Z
		dist := math.Hypot(dx, dz)
		if dist < 0.2 {
			w.sendError(p.Out, "invalid_destination", "Destino de teletransporte inválido")
			return
		}
		if dist > skill.TeleportDistance {
			scale := skill.TeleportDistance / dist
			destX = p.X + dx*scale
			destZ = p.Z + dz*scale
		}
		destX = clamp(destX, -MapHalfSize, MapHalfSize)
		destZ = clamp(destZ, -MapHalfSize, MapHalfSize)
	}

	// Consumir recursos y arrancar cooldown.
	p.MP -= skill.ManaCost
	cooldownTicks := uint64(skill.CooldownMS * TickRate / 1000)
	if cooldownTicks < 1 {
		cooldownTicks = 1
	}
	p.Skills[skill.ID] = w.tick + cooldownTicks

	if data, err := protocol.Encode("skill_cast", protocol.SkillCast{
		SkillID: skill.ID, CooldownMS: skill.CooldownMS,
	}); err == nil {
		w.send(p, data)
	}

	effectX, effectZ, radius := p.X, p.Z, 1.0
	switch skill.Kind {
	case "single", "aoe":
		effectX, effectZ = target.X, target.Z
		if skill.Kind == "aoe" {
			radius = skill.Radius
		}
	case "heal", "buff":
		radius = 2.0
	case "teleport":
		effectX, effectZ = destX, destZ
	}
	w.broadcastEvent(p.MapID, "skill_effect", protocol.SkillEffect{
		SkillID: skill.ID, Caster: p.ID,
		X: effectX, Z: effectZ, Radius: radius, Kind: skill.Kind,
	})

	switch skill.Kind {
	case "single":
		hits := skill.Hits
		if hits < 1 {
			hits = 1
		}
		for i := 0; i < hits; i++ {
			if target.State == "dead" {
				break
			}
			w.skillHit(p, target, skill)
		}
	case "aoe":
		for _, m := range w.aoeTargets(p, target, skill.Radius, skill.MaxTargets) {
			if m.State == "dead" {
				continue
			}
			w.skillHit(p, m, skill)
		}
	case "heal":
		heal := max(1, p.HPMax*skill.HealPercent/100)
		p.HP = min(p.HPMax, p.HP+heal)
	case "buff":
		p.BuffPercent = skill.BuffPercent
		p.BuffUntil = w.tick + uint64(skill.BuffDurationMS*TickRate/1000)
	case "teleport":
		dx := destX - p.X
		dz := destZ - p.Z
		p.X, p.Z = destX, destZ
		p.Rot = math.Atan2(dx, dz)
		p.Anim = "idle"
		p.dirty = true
		w.queueSave(p)
	}

	w.sendSelfState(p)
	w.log.Debug("skill usada", "player", p.Name, "skill", skill.ID, "map", p.MapID)
}

func (w *World) resolveSkillTarget(p *player, targetID int64, maxRange float64) *mob {
	if maxRange <= 0 {
		maxRange = math.MaxFloat64
	}
	if targetID != 0 {
		m := w.mobsByID[targetID]
		if m != nil && m.State != "dead" && m.MapID == p.MapID &&
			math.Hypot(m.X-p.X, m.Z-p.Z) <= maxRange+0.6 {
			return m
		}
	}

	var best *mob
	bestDist := maxRange + 0.6
	for _, m := range w.mobs {
		if m.MapID != p.MapID || m.State == "dead" {
			continue
		}
		d := math.Hypot(m.X-p.X, m.Z-p.Z)
		if d < bestDist {
			best = m
			bestDist = d
		}
	}
	return best
}

func (w *World) aoeTargets(p *player, center *mob, radius float64, maxTargets int) []*mob {
	targets := []*mob{center}
	for _, m := range w.mobs {
		if m == center || m.MapID != p.MapID || m.State == "dead" {
			continue
		}
		if math.Hypot(m.X-center.X, m.Z-center.Z) <= radius+0.4 {
			targets = append(targets, m)
		}
	}
	if maxTargets < 1 {
		maxTargets = 1
	}
	if len(targets) > maxTargets {
		targets = targets[:maxTargets]
	}
	return targets
}

func (w *World) skillHit(p *player, m *mob, skill gamedata.Skill) {
	minD, maxD := w.damageRange(p)
	mult := skill.Multiplier * w.damageMultiplier(p)
	minD = max(1, int(math.Round(float64(minD)*mult)))
	maxD = max(1, int(math.Round(float64(maxD)*mult)))

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

	w.sendDamage(p.MapID, p.ID, m.ID, damage, crit, "skill")
	w.checkBossMechanics(m)
	if m.HP <= 0 {
		w.killMob(m, p)
	}
}
