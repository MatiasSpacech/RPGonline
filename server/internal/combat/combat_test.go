package combat

import (
	"math"
	"testing"
	"time"
)

func TestDerivedStats(t *testing.T) {
	warrior := Stats{Level: 1, Str: 28, Agi: 20, Vit: 25, Ene: 10}
	if got := MaxHP(warrior); got != 73 {
		t.Fatalf("MaxHP = %d, quería 73", got)
	}
	if got := MaxMP(warrior); got != 26 {
		t.Fatalf("MaxMP = %d, quería 26", got)
	}
	if got := Defense(warrior); got != 8 { // round(7.5)
		t.Fatalf("Defense = %d, quería 8", got)
	}
	if got := Evasion(warrior); math.Abs(got-7.0) > 0.001 {
		t.Fatalf("Evasion = %v, quería 7", got)
	}

	mage := Stats{Level: 1, Str: 18, Agi: 18, Vit: 15, Ene: 30}
	if got := MaxMP(mage); got != 56 {
		t.Fatalf("MaxMP mago = %d, quería 56", got)
	}

	// Tope de evasión al 30%.
	if got := Evasion(Stats{Agi: 500}); got != 30 {
		t.Fatalf("Evasion cap = %v, quería 30", got)
	}
}

func TestDamageRanges(t *testing.T) {
	warrior := Stats{Str: 28, Agi: 20, Ene: 10}
	minD, maxD := DamageRange(warrior, "warrior")
	if minD != 21 || maxD != 24 { // 4+16.8=20.8→21; 7+16.8=23.8→24
		t.Fatalf("daño guerrero = (%d, %d), quería (21, 24)", minD, maxD)
	}

	mage := Stats{Str: 18, Agi: 18, Ene: 30}
	minD, maxD = DamageRange(mage, "mage")
	if minD != 29 || maxD != 32 { // 5+24 ; 7+24? 8+24=32
		t.Fatalf("daño mago = (%d, %d), quería (29, 32)", minD, maxD)
	}

	archer := Stats{Str: 22, Agi: 25, Ene: 22}
	minD, maxD = DamageRange(archer, "archer")
	if minD != 16 || maxD != 19 { // 3+12.5=15.5→16 ; 6+12.5=18.5→19
		t.Fatalf("daño arquera = (%d, %d), quería (16, 19)", minD, maxD)
	}
}

func TestAttackInterval(t *testing.T) {
	if got := AttackInterval(Stats{Agi: 20}, "warrior"); got != 1000*time.Millisecond {
		t.Fatalf("intervalo guerrero = %v, quería 1s", got)
	}
	if got := AttackInterval(Stats{Agi: 25}, "archer"); got != 800*time.Millisecond {
		t.Fatalf("intervalo arquera = %v, quería 800ms", got)
	}
	if got := AttackInterval(Stats{Agi: 100000}, "warrior"); got != 250*time.Millisecond {
		t.Fatalf("intervalo mínimo = %v, quería 250ms", got)
	}
}

func TestRollAndDefense(t *testing.T) {
	if got := RollDamage(10, 20, 0); got != 10 {
		t.Fatalf("roll 0 = %d", got)
	}
	if got := RollDamage(10, 20, 0.999999); got != 20 {
		t.Fatalf("roll máximo = %d", got)
	}
	for i := 0; i < 1000; i++ {
		roll := float64(i) / 1000.0
		if got := RollDamage(10, 20, roll); got < 10 || got > 20 {
			t.Fatalf("roll %v fuera de rango: %d", roll, got)
		}
	}

	if got := ApplyDefense(20, 8); got != 19 { // 20*100/108 = 18.5→19
		t.Fatalf("ApplyDefense = %d, quería 19", got)
	}
	if got := ApplyDefense(1, 9999); got != 1 {
		t.Fatalf("ApplyDefense mínimo = %d, quería 1", got)
	}
}

func TestDodgeAndCrit(t *testing.T) {
	if !Dodged(Stats{Agi: 20}, 0) {
		t.Fatal("roll 0 debería esquivar")
	}
	if Dodged(Stats{Agi: 20}, 99) {
		t.Fatal("roll 99 no debería esquivar")
	}
	if !IsCrit(0.01) || IsCrit(0.99) {
		t.Fatal("IsCrit fuera de rango")
	}
	if got := CritDamage(20); got != 30 {
		t.Fatalf("CritDamage = %d, quería 30", got)
	}
}

func TestXPToNext(t *testing.T) {
	cases := map[int]int64{1: 60, 2: 195, 5: 926, 10: 3007, 30: 19465}
	for level, want := range cases {
		if got := XPToNext(level); got != want {
			t.Fatalf("XPToNext(%d) = %d, quería %d", level, got, want)
		}
	}
}

func TestXPForKillAttenuation(t *testing.T) {
	if got := XPForKill(100, 5, 5); got != 100 {
		t.Fatalf("mismo nivel = %d, quería 100", got)
	}
	if got := XPForKill(100, 1, 6); got != 100 {
		t.Fatalf("+5 niveles = %d, quería 100 (sin atenuar)", got)
	}
	if got := XPForKill(100, 1, 11); got >= 100 || got <= 0 {
		t.Fatalf("+10 niveles = %d, quería atenuado", got)
	}
	if got := XPForKill(100, 1, 100); got != 10 {
		t.Fatalf("atenuación piso = %d, quería 10", got)
	}
}

func TestDeathPenalty(t *testing.T) {
	// Nivel 1: XPNext=60, penalidad round(1.8)=2.
	if remaining, lost := DeathPenalty(30, 1); remaining != 28 || lost != 2 {
		t.Fatalf("DeathPenalty = (%d, %d), quería (28, 2)", remaining, lost)
	}
	// Nunca deja XP negativo.
	if remaining, lost := DeathPenalty(0, 5); remaining != 0 || lost != 0 {
		t.Fatalf("DeathPenalty con xp 0 = (%d, %d)", remaining, lost)
	}
}
