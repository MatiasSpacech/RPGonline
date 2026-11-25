package protocol

// Mensajes de F1. El catálogo completo está en docs/04-red.md.
// Los nombres de campos JSON son parte del protocolo: no cambiarlos sin
// actualizar el cliente y la documentación.

// Login entra a un nombre de usuario (sin contraseña todavía: F2 agrega auth).
type Login struct {
	User string `json:"user"`
}

// MoveReq pide moverse a un punto del mapa. Y se ignora por ahora (suelo plano).
type MoveReq struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Entity es el estado visible de una entidad en una snapshot.
type Entity struct {
	ID   int64   `json:"id"`
	Kind string  `json:"k"`
	Name string  `json:"name,omitempty"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Z    float64 `json:"z"`
	Rot  float64 `json:"r"`
	Anim string  `json:"anim,omitempty"`
}

// Welcome es la primera respuesta tras un login válido.
type Welcome struct {
	Self         Entity `json:"self"`
	Map          string `json:"map"`
	TickRate     int    `json:"tick_rate"`
	SnapshotRate int    `json:"snapshot_rate"`
}

// Snapshot es el estado autoritativo del mapa en un tick.
type Snapshot struct {
	Tick uint64   `json:"tick"`
	Ents []Entity `json:"ents"`
}

// Error informa un fallo de protocolo/validación.
type Error struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}
