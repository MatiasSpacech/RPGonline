package protocol

import "munext/server/internal/model"

// Mensajes de F2 (cuentas/lobby) además de los de F1. El catálogo completo
// está en docs/04-red.md.

// Auth sirve tanto para login como para registro.
type Auth struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

// Resume reanuda una sesión con el token emitido en login_ok.
type Resume struct {
	Token string `json:"token"`
}

// LoginOK confirma autenticación y devuelve token + personajes.
type LoginOK struct {
	Token string                   `json:"token"`
	User  string                   `json:"user"`
	Chars []model.CharacterSummary `json:"chars"`
}

// CreateChar pide crear un personaje (máximo 3 por cuenta).
type CreateChar struct {
	Name  string `json:"name"`
	Class string `json:"class"`
}

// CharacterCreated confirma la creación.
type CharacterCreated struct {
	Char model.CharacterSummary `json:"char"`
}

// DeleteChar borra un personaje propio.
type DeleteChar struct {
	CharID int64 `json:"char_id"`
}

// CharacterDeleted confirma el borrado.
type CharacterDeleted struct {
	CharID int64 `json:"char_id"`
}

// SelectChar entra al mundo con un personaje.
type SelectChar struct {
	CharID int64 `json:"char_id"`
}

// MoveReq pide moverse a un punto del mapa. Y se ignora por ahora (suelo plano).
type MoveReq struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// Entity es el estado visible de una entidad en una snapshot.
type Entity struct {
	ID    int64   `json:"id"`
	Kind  string  `json:"k"`
	Tmpl  string  `json:"tmpl,omitempty"`
	Name  string  `json:"name,omitempty"`
	Level int     `json:"level,omitempty"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
	Rot   float64 `json:"r"`
	Anim  string  `json:"anim,omitempty"`
	HP    int     `json:"hp,omitempty"`
	HPMax int     `json:"hp_max,omitempty"`
}

// Welcome es la primera respuesta tras seleccionar un personaje.
type Welcome struct {
	Self         Entity                 `json:"self"`
	Map          string                 `json:"map"`
	TickRate     int                    `json:"tick_rate"`
	SnapshotRate int                    `json:"snapshot_rate"`
	Character    model.CharacterSummary `json:"character"`
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
