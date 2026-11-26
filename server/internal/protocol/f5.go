package protocol

// Mensajes de F5: mapas/portales y skills.

// UsePortal viaja a otro mapa usando un portal del mapa actual.
type UsePortal struct {
	ID string `json:"id"`
}

// UseSkill lanza una skill. Target es el mob opcional; X/Z se usan para
// teletransporte (punto elegido con el mouse).
type UseSkill struct {
	SkillID string  `json:"skill_id"`
	Target  int64   `json:"target"`
	X       float64 `json:"x"`
	Z       float64 `json:"z"`
}

// SkillCast confirma el lanzamiento y arranca el cooldown en el cliente.
type SkillCast struct {
	SkillID    string `json:"skill_id"`
	CooldownMS int    `json:"cooldown_ms"`
}

// SkillEffect es el evento visual que se difunde a los jugadores del mapa.
type SkillEffect struct {
	SkillID string  `json:"skill_id"`
	Caster  int64   `json:"caster"`
	X       float64 `json:"x"`
	Z       float64 `json:"z"`
	Radius  float64 `json:"radius"`
	Kind    string  `json:"kind"`
}
