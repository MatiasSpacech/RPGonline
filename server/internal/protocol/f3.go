package protocol

// Mensajes de F3: combate, progresión y muerte (ver docs/04-red.md).

// Attack fija el mob objetivo del auto-ataque (0 = detener).
type Attack struct {
	Target int64 `json:"target"`
}

// Potion usa una poción de HP o MP.
type Potion struct {
	Kind string `json:"kind"` // "hp" | "mp"
}

// AllocStat gasta un punto de stat.
type AllocStat struct {
	Stat string `json:"stat"` // "str" | "agi" | "vit" | "ene"
}

// SelfState es el estado privado del jugador (HUD).
type SelfState struct {
	HP         int   `json:"hp"`
	HPMax      int   `json:"hp_max"`
	MP         int   `json:"mp"`
	MPMax      int   `json:"mp_max"`
	XP         int64 `json:"xp"`
	XPNext     int64 `json:"xp_next"`
	Level      int   `json:"level"`
	Str        int   `json:"str"`
	Agi        int   `json:"agi"`
	Vit        int   `json:"vit"`
	Ene        int   `json:"ene"`
	StatPoints int   `json:"stat_points"`
	HPPotions  int   `json:"hp_potions"`
	MPPotions  int   `json:"mp_potions"`
	AttackMin  int   `json:"attack_min"`
	AttackMax  int   `json:"attack_max"`
	Defense    int   `json:"defense"`
	IntervalMS int   `json:"attack_interval_ms"`
}

// Damage es un golpe (o esquiva) para números flotantes y feedback.
type Damage struct {
	Src    int64  `json:"src"`
	Dst    int64  `json:"dst"`
	Amount int    `json:"amount"`
	Crit   bool   `json:"crit"`
	Kind   string `json:"kind"` // "out" | "in" | "dodge"
}

// MobDead informa la muerte de un mob, el XP y el oro otorgados.
type MobDead struct {
	ID     int64 `json:"id"`
	XP     int64 `json:"xp"`
	Gold   int64 `json:"gold"`
	Killer int64 `json:"killer"`
}

// Death informa la muerte del jugador y el XP perdido.
type Death struct {
	XPLost int64 `json:"xp_lost"`
}

// LevelUp informa un nivel nuevo.
type LevelUp struct {
	Level      int `json:"level"`
	StatPoints int `json:"stat_points"`
}

// Teleport recoloca al jugador (respawn/muerte).
type Teleport struct {
	Map string  `json:"map"`
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	Z   float64 `json:"z"`
}
