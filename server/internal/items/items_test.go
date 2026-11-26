package items

import (
	"testing"

	"munext/server/internal/combat"
	"munext/server/internal/gamedata"
)

func sword() gamedata.ItemTemplate {
	return gamedata.ItemTemplate{
		ID: "sword_iron", Name: "Espada de hierro", Kind: "weapon", Slot: "weapon",
		Class: "warrior", ReqStr: 25, MinDamage: 8, MaxDamage: 14,
		IntervalMS: 1100, Range: 1.8, Value: 200,
	}
}

func TestRarity(t *testing.T) {
	cases := map[float64]string{
		0.0: RarityCommon, 0.59: RarityCommon,
		0.60: RarityGood, 0.84: RarityGood,
		0.85: RarityRare, 0.94: RarityRare,
		0.95: RarityEpic, 0.98: RarityEpic,
		0.99: RarityLegendary, 0.9999: RarityLegendary,
	}
	for roll, want := range cases {
		if got := RollRarity(roll); got != want {
			t.Fatalf("RollRarity(%v) = %s, quería %s", roll, got, want)
		}
	}
	if RarityMultiplier(RarityLegendary) != 1.8 {
		t.Fatal("multiplicador legendario incorrecto")
	}
	if EnhanceMultiplier(12) != 1.96 {
		t.Fatalf("mejora +12 = %v, quería 1.96", EnhanceMultiplier(12))
	}
}

func TestEffectiveStats(t *testing.T) {
	minD, maxD := EffectiveDamage(sword(), RarityCommon, 0)
	if minD != 8 || maxD != 14 {
		t.Fatalf("daño base = (%d, %d)", minD, maxD)
	}
	// Raro (x1.3) +6 (+48%): 8*1.3*1.48=15.392→15 ; 14*1.3*1.48=26.936→27
	minD, maxD = EffectiveDamage(sword(), RarityRare, 6)
	if minD != 15 || maxD != 27 {
		t.Fatalf("daño raro +6 = (%d, %d), quería (15, 27)", minD, maxD)
	}

	armor := gamedata.ItemTemplate{ID: "cap", Kind: "armor", Slot: "helm", Defense: 4, Value: 30}
	if got := EffectiveDefense(armor, RarityGood, 3); got != 6 { // 4*1.15*1.24=5.704→6
		t.Fatalf("defensa = %d, quería 6", got)
	}
}

func TestCanEquip(t *testing.T) {
	stats := combat.Stats{Level: 1, Str: 28, Agi: 20, Vit: 25, Ene: 10}
	if !CanEquip(sword(), "warrior", stats) {
		t.Fatal("el guerrero debería poder equipar la espada")
	}
	if CanEquip(sword(), "mage", stats) {
		t.Fatal("el mago no puede equipar una espada de guerrero")
	}
	weak := combat.Stats{Str: 10}
	if CanEquip(sword(), "warrior", weak) {
		t.Fatal("no debería cumplir el requisito de STR")
	}
}

func TestEnhanceTableAndResolution(t *testing.T) {
	rates := []float64{0.95, 0.90, 0.85, 0.78, 0.70, 0.70, 0.55, 0.45, 0.38, 0.30, 0.20, 0.12}
	for from, want := range rates {
		got, ok := EnhanceSuccessRate(from)
		if !ok || got != want {
			t.Fatalf("rate(%d) = %v, quería %v", from, got, want)
		}
	}
	if _, ok := EnhanceSuccessRate(12); ok {
		t.Fatal("+12 no debería tener intento")
	}

	if level, success, destroyed := ResolveEnhance(0, 0.0, 0.0); !success || level != 1 || destroyed {
		t.Fatalf("+0 éxito = (%d, %v, %v)", level, success, destroyed)
	}
	if level, success, destroyed := ResolveEnhance(7, 0.99, 0.99); success || level != 6 || destroyed {
		t.Fatalf("+7 fallo = (%d, %v, %v)", level, success, destroyed)
	}
	if level, success, destroyed := ResolveEnhance(10, 0.99, 0.01); success || !destroyed || level != 0 {
		t.Fatalf("+10 fallo con destrucción = (%d, %v, %v)", level, success, destroyed)
	}
	if level, success, destroyed := ResolveEnhance(11, 0.99, 0.50); success || destroyed || level != 10 {
		t.Fatalf("+11 fallo = (%d, %v, %v)", level, success, destroyed)
	}
}

func TestCostsAndSell(t *testing.T) {
	if EnhanceCost(0) != 50 || EnhanceCost(6) != 1850 {
		t.Fatalf("costos = %d / %d", EnhanceCost(0), EnhanceCost(6))
	}
	// 200 * 0.3 = 60 común; raro +6: 60*1.3*1.48=115.44→115
	if got := SellPrice(sword(), RarityCommon, 0); got != 60 {
		t.Fatalf("venta común = %d, quería 60", got)
	}
	if got := SellPrice(sword(), RarityRare, 6); got != 115 {
		t.Fatalf("venta rara +6 = %d, quería 115", got)
	}
}
