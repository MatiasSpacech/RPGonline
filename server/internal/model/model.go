// Package model define los tipos de dominio compartidos por las capas del
// servidor (cuentas, base de datos, mundo y red).
package model

import "errors"

// Errores de dominio. Las capas de red los traducen a mensajes para el cliente.
var (
	ErrNotFound           = errors.New("no encontrado")
	ErrUsernameTaken      = errors.New("nombre de usuario en uso")
	ErrCharacterNameTaken = errors.New("nombre de personaje en uso")
	ErrCharacterLimit     = errors.New("límite de personajes alcanzado")
)

// Clases jugables (ids estables: se guardan en la base de datos).
const (
	ClassWarrior = "warrior"
	ClassMage    = "mage"
	ClassArcher  = "archer"
)

// ClassDef agrupa los stats iniciales de una clase (ver docs/02-diseno-de-juego.md).
type ClassDef struct {
	ID   string
	Name string // nombre visible en español
	Str  int
	Agi  int
	Vit  int
	Ene  int
}

// Classes es el catálogo estático de clases del MVP.
var Classes = map[string]ClassDef{
	ClassWarrior: {ID: ClassWarrior, Name: "Guerrero", Str: 28, Agi: 20, Vit: 25, Ene: 10},
	ClassMage:    {ID: ClassMage, Name: "Mago", Str: 18, Agi: 18, Vit: 15, Ene: 30},
	ClassArcher:  {ID: ClassArcher, Name: "Arquera", Str: 22, Agi: 25, Vit: 20, Ene: 22},
}

// User es una cuenta.
type User struct {
	ID       int64
	Username string
}

// CharacterSummary es lo mínimo para el lobby (lista de personajes).
type CharacterSummary struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
	Level int    `json:"level"`
}

// Character es el estado completo de un personaje guardado en la BD.
type Character struct {
	ID         int64
	UserID     int64
	Name       string
	Class      string
	Level      int
	XP         int64
	Str        int
	Agi        int
	Vit        int
	Ene        int
	StatPoints int
	Gold       int64
	MapID      string
	X, Y, Z    float64
}

// Summary devuelve la versión resumida (lobby).
func (c Character) Summary() CharacterSummary {
	return CharacterSummary{ID: c.ID, Name: c.Name, Class: c.Class, Level: c.Level}
}

// CreateCharacterParams son los datos necesarios para insertar un personaje.
type CreateCharacterParams struct {
	UserID int64
	Name   string
	Class  string
	Str    int
	Agi    int
	Vit    int
	Ene    int
}

// CharacterState es lo que se persiste periódicamente y al desconectarse.
type CharacterState struct {
	CharacterID int64
	MapID       string
	X, Y, Z     float64
}
