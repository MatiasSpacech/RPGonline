// Package combat contiene las fórmulas de combate del GDD (docs/02-diseno-de-juego.md).
// Son funciones puras: entran números, salen números. Eso las hace triviales
// de testear y de rebalancear sin tocar la simulación.
package combat

import (
	"math"
	"time"
)

// Stats son los atributos de un personaje.
type Stats struct {
	Level int
	Str   int
	Agi   int
	Vit   int
	Ene   int
}

// ClassProfile es el "arma base" provisional por clase hasta que existan
// ítems equipables (F4).
type ClassProfile struct {
	MinDamage  int
	MaxDamage  int
	IntervalMS float64
	Range      float64
}

var profiles = map[string]ClassProfile{
	"warrior": {MinDamage: 4, MaxDamage: 7, IntervalMS: 1100, Range: 1.8},
	"mage":    {MinDamage: 5, MaxDamage: 8, IntervalMS: 1400, Range: 8.0},
	"archer":  {MinDamage: 3, MaxDamage: 6, IntervalMS: 900, Range: 9.0},
}

// Profile devuelve el perfil de la clase (o uno conservador si es desconocida).
func Profile(class string) ClassProfile {
	if p, ok := profiles[class]; ok {
		return p
	}
	return ClassProfile{MinDamage: 1, MaxDamage: 1, IntervalMS: 1200, Range: 1.5}
}

// MaxHP aplica la fórmula del GDD: 20 + VIT*2 + nivel*3.
func MaxHP(s Stats) int {
	level := s.Level
	if level < 1 {
		level = 1
	}
	return 20 + s.Vit*2 + level*3
}

// MaxMP aplica: 10 + ENE*1.5 + nivel.
func MaxMP(s Stats) int {
	level := s.Level
	if level < 1 {
		level = 1
	}
	return 10 + int(math.Round(float64(s.Ene)*1.5)) + level
}

// Defense aplica: VIT * 0.3.
func Defense(s Stats) int {
	return int(math.Round(float64(s.Vit) * 0.3))
}

// Evasion aplica: 5 + AGI*0.1, con tope de 30%.
func Evasion(s Stats) float64 {
	evasion := 5 + float64(s.Agi)*0.1
	if evasion > 30 {
		return 30
	}
	return evasion
}

// AttackRange es el alcance del auto-ataque según la clase.
func AttackRange(class string) float64 {
	return Profile(class).Range
}

// AttackInterval baja con AGI: base_ms / (1 + AGI*0.005), mínimo 250 ms.
func AttackInterval(s Stats, class string) time.Duration {
	return AttackIntervalFromBase(s, Profile(class).IntervalMS)
}

// AttackIntervalFromBase aplica la reducción por AGI a un arma concreta.
func AttackIntervalFromBase(s Stats, baseMS float64) time.Duration {
	ms := baseMS / (1 + float64(s.Agi)*0.005)
	if ms < 250 {
		ms = 250
	}
	return time.Duration(math.Round(ms)) * time.Millisecond
}

// DamageRange devuelve (min, max) de daño según clase y stats con el arma
// base de la clase. Guerrero: +STR*0.6 · Mago: +ENE*0.8 · Arquera: +AGI*0.5.
func DamageRange(s Stats, class string) (int, int) {
	p := Profile(class)
	return DamageRangeFromBase(s, class, p.MinDamage, p.MaxDamage)
}

// DamageRangeFromBase aplica el bonus de stat de la clase al daño de un arma.
func DamageRangeFromBase(s Stats, class string, baseMin, baseMax int) (int, int) {
	bonus := 0.0
	switch class {
	case "warrior":
		bonus = float64(s.Str) * 0.6
	case "mage":
		bonus = float64(s.Ene) * 0.8
	case "archer":
		bonus = float64(s.Agi) * 0.5
	}

	minDamage := int(math.Round(float64(baseMin) + bonus))
	maxDamage := int(math.Round(float64(baseMax) + bonus))
	if minDamage < 1 {
		minDamage = 1
	}
	if maxDamage < minDamage {
		maxDamage = minDamage
	}
	return minDamage, maxDamage
}

// RollDamage elige un valor uniforme en [min, max]. roll debe ser [0, 1).
func RollDamage(minDamage, maxDamage int, roll float64) int {
	if maxDamage <= minDamage {
		return maxDamage
	}
	if roll < 0 {
		roll = 0
	}
	if roll >= 1 {
		roll = 0.999999
	}
	return minDamage + int(roll*float64(maxDamage-minDamage+1))
}

// ApplyDefense reduce el daño con la fórmula 100/(100+defensa), mínimo 1.
func ApplyDefense(damage, defense int) int {
	if damage < 1 {
		damage = 1
	}
	if defense < 0 {
		defense = 0
	}
	result := int(math.Round(float64(damage) * 100.0 / (100.0 + float64(defense))))
	if result < 1 {
		return 1
	}
	return result
}

// Dodged indica si un ataque falló. roll100 debe ser [0, 100).
func Dodged(s Stats, roll100 float64) bool {
	return roll100 < Evasion(s)
}

// CritChance es el 5% base del GDD.
const CritChance = 5.0

// CritMultiplier es el x1.5 del GDD.
const CritMultiplier = 1.5

// IsCrit decide un crítico. roll debe ser [0, 1).
func IsCrit(roll float64) bool {
	return roll*100.0 < CritChance
}

// CritDamage aplica el multiplicador de crítico (siempre sube al menos 1).
func CritDamage(damage int) int {
	result := int(math.Round(float64(damage) * CritMultiplier))
	if result <= damage {
		return damage + 1
	}
	return result
}

// XPToNext devuelve el XP necesario para pasar del nivel dado al siguiente.
// xp(n) = round(60 * n^1.7). El XP del personaje es progreso DENTRO del nivel.
func XPToNext(level int) int64 {
	if level < 1 {
		level = 1
	}
	xp := int64(math.Round(60.0 * math.Pow(float64(level), 1.7)))
	if xp < 1 {
		return 1
	}
	return xp
}

// XPForKill aplica la atenuación por diferencia de nivel del GDD:
// matar mobs 5+ niveles por debajo reduce el XP, con piso del 10%.
func XPForKill(baseXP int64, mobLevel, playerLevel int) int64 {
	if baseXP <= 0 {
		return 0
	}
	diff := playerLevel - mobLevel - 5
	if diff <= 0 {
		return baseXP
	}
	factor := math.Pow(0.3, float64(diff)/5.0)
	if factor < 0.1 {
		factor = 0.1
	}
	xp := int64(math.Round(float64(baseXP) * factor))
	if xp < 1 {
		xp = 1
	}
	return xp
}

// DeathPenalty pierde 3% del XP necesario para el nivel actual, sin bajar
// el XP por debajo de 0 (nunca se pierde nivel).
func DeathPenalty(xp int64, level int) (remaining, lost int64) {
	penalty := int64(math.Round(float64(XPToNext(level)) * 0.03))
	if penalty < 1 {
		penalty = 1
	}
	if penalty > xp {
		penalty = xp
	}
	return xp - penalty, penalty
}
