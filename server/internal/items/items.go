// Package items contiene las reglas puras de ítems: rareza, mejora +N,
// requisitos de equipamiento y precios. Testeable sin red ni BD.
package items

import (
	"math"

	"munext/server/internal/combat"
	"munext/server/internal/gamedata"
)

// Rarezas del GDD (docs/02-diseno-de-juego.md §5.2).
const (
	RarityCommon    = "common"
	RarityGood      = "good"
	RarityRare      = "rare"
	RarityEpic      = "epic"
	RarityLegendary = "legendary"
)

// MaxEnhance es el tope de mejora del MVP.
const MaxEnhance = 12

// EnhanceBonusPerLevel es el +8% de stats por nivel de mejora.
const EnhanceBonusPerLevel = 0.08

// RarityMultiplier aplica el multiplicador de stats de la rareza.
func RarityMultiplier(rarity string) float64 {
	switch rarity {
	case RarityGood:
		return 1.15
	case RarityRare:
		return 1.3
	case RarityEpic:
		return 1.5
	case RarityLegendary:
		return 1.8
	default:
		return 1.0
	}
}

// EnhanceMultiplier aplica +8% por nivel de mejora.
func EnhanceMultiplier(enhance int) float64 {
	if enhance < 0 {
		enhance = 0
	}
	if enhance > MaxEnhance {
		enhance = MaxEnhance
	}
	return 1.0 + EnhanceBonusPerLevel*float64(enhance)
}

// RollRarity elige una rareza con los pesos del GDD. roll debe ser [0, 1).
func RollRarity(roll float64) string {
	if roll < 0 {
		roll = 0
	}
	if roll >= 1 {
		roll = 0.999999
	}
	switch pct := roll * 100; {
	case pct < 60:
		return RarityCommon
	case pct < 85:
		return RarityGood
	case pct < 95:
		return RarityRare
	case pct < 99:
		return RarityEpic
	default:
		return RarityLegendary
	}
}

// EffectiveDamage devuelve el daño de un arma con rareza y mejora aplicadas.
func EffectiveDamage(t gamedata.ItemTemplate, rarity string, enhance int) (int, int) {
	mult := RarityMultiplier(rarity) * EnhanceMultiplier(enhance)
	minDamage := int(math.Round(float64(t.MinDamage) * mult))
	maxDamage := int(math.Round(float64(t.MaxDamage) * mult))
	if minDamage < 1 {
		minDamage = 1
	}
	if maxDamage < minDamage {
		maxDamage = minDamage
	}
	return minDamage, maxDamage
}

// EffectiveDefense devuelve la defensa de una armadura con rareza y mejora.
func EffectiveDefense(t gamedata.ItemTemplate, rarity string, enhance int) int {
	mult := RarityMultiplier(rarity) * EnhanceMultiplier(enhance)
	defense := int(math.Round(float64(t.Defense) * mult))
	if defense < 0 {
		return 0
	}
	return defense
}

// CanEquip valida clase y requisitos de stats base.
func CanEquip(t gamedata.ItemTemplate, class string, s combat.Stats) bool {
	if t.Slot == "" {
		return false
	}
	if t.Class != "" && t.Class != class {
		return false
	}
	return s.Str >= t.ReqStr && s.Agi >= t.ReqAgi && s.Vit >= t.ReqVit && s.Ene >= t.ReqEne
}

// EnhanceSuccessRate devuelve la probabilidad de éxito del intento from→from+1.
// La tabla sigue el GDD §5.3.
func EnhanceSuccessRate(from int) (float64, bool) {
	rates := [...]float64{
		0.95, 0.90, 0.85, 0.78, 0.70, 0.70,
		0.55, 0.45, 0.38, 0.30, 0.20, 0.12,
	}
	if from < 0 || from >= len(rates) {
		return 0, false
	}
	return rates[from], true
}

// EnhanceCost es el oro que cuesta el intento desde el nivel from.
func EnhanceCost(from int) int64 {
	if from < 0 {
		from = 0
	}
	return 50 + 50*int64(from)*int64(from)
}

// ResolveEnhance decide el resultado de un intento. successRoll y destroyRoll
// deben ser [0, 1). A partir de +7 un fallo baja 1 nivel; a partir de +10 hay
// 5% de destrucción (el GDD permite ajustar esa decisión en balance).
func ResolveEnhance(from int, successRoll, destroyRoll float64) (newLevel int, success bool, destroyed bool) {
	rate, ok := EnhanceSuccessRate(from)
	if !ok {
		return from, false, false
	}
	if successRoll < rate {
		return from + 1, true, false
	}
	if from >= 10 && destroyRoll < 0.05 {
		return 0, false, true
	}
	if from >= 7 {
		return from - 1, false, false
	}
	return from, false, false
}

// SellPrice devuelve lo que paga el vendedor: 30% del valor, ajustado por
// rareza y mejora.
func SellPrice(t gamedata.ItemTemplate, rarity string, enhance int) int64 {
	value := float64(t.Value) * 0.3 * RarityMultiplier(rarity) * EnhanceMultiplier(enhance)
	price := int64(math.Round(value))
	if price < 1 {
		return 1
	}
	return price
}
