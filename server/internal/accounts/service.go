// Package accounts implementa registro/login y la gestión de personajes.
// Es la capa de validación entre la red y la base de datos.
package accounts

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"munext/server/internal/auth"
	"munext/server/internal/model"
)

// Errores de validación/autenticación visibles para el usuario.
var (
	ErrInvalidCredentials   = errors.New("usuario o contraseña incorrectos")
	ErrInvalidUsername      = errors.New("usuario inválido")
	ErrWeakPassword         = errors.New("contraseña inválida")
	ErrInvalidCharacterName = errors.New("nombre de personaje inválido")
	ErrInvalidClass         = errors.New("clase inválida")
)

var (
	usernamePattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{3,16}$`)
	characterPattern = regexp.MustCompile(`^[\p{L}]{3,16}$`)
)

// Store es el subconjunto de la base de datos que necesita el servicio.
// Permite testear con un fake en memoria.
type Store interface {
	CreateUser(ctx context.Context, username, passwordHash string) (model.User, error)
	GetUserByUsername(ctx context.Context, username string) (model.User, string, error)
	ListCharacters(ctx context.Context, userID int64) ([]model.CharacterSummary, error)
	CreateCharacter(ctx context.Context, params model.CreateCharacterParams) (model.Character, error)
	DeleteCharacter(ctx context.Context, userID, characterID int64) error
	LoadCharacter(ctx context.Context, userID, characterID int64) (model.Character, error)
}

// Service valida y ejecuta las operaciones de cuenta.
type Service struct {
	store Store
	// hashSem limita los hashes Argon2 concurrentes (64 MB cada uno).
	hashSem chan struct{}
}

// New crea el servicio de cuentas.
func New(store Store) *Service {
	return &Service{store: store, hashSem: make(chan struct{}, 2)}
}

// dummyHash se usa para igualar el tiempo de respuesta cuando el usuario no
// existe (evita filtrar qué nombres están registrados por timing).
var dummyHash = func() string {
	hash, err := auth.HashPassword("dummy-password-for-timing")
	if err != nil {
		panic(err)
	}
	return hash
}()

func (s *Service) hashPassword(password string) (string, error) {
	s.hashSem <- struct{}{}
	defer func() { <-s.hashSem }()
	return auth.HashPassword(password)
}

func (s *Service) verifyPassword(password, hash string) bool {
	s.hashSem <- struct{}{}
	defer func() { <-s.hashSem }()
	return auth.VerifyPassword(password, hash)
}

// Register crea una cuenta y devuelve el usuario listo para iniciar sesión.
func (s *Service) Register(ctx context.Context, username, password string) (model.User, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return model.User{}, ErrInvalidUsername
	}
	if len(password) < 6 || len(password) > 72 {
		return model.User{}, ErrWeakPassword
	}

	hash, err := s.hashPassword(password)
	if err != nil {
		return model.User{}, err
	}
	return s.store.CreateUser(ctx, username, hash)
}

// Login verifica credenciales y devuelve el usuario.
func (s *Service) Login(ctx context.Context, username, password string) (model.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return model.User{}, ErrInvalidCredentials
	}

	user, hash, err := s.store.GetUserByUsername(ctx, username)
	if errors.Is(err, model.ErrNotFound) {
		// Mismo costo que un login real: no revelar si el usuario existe.
		s.verifyPassword(password, dummyHash)
		return model.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return model.User{}, err
	}
	if !s.verifyPassword(password, hash) {
		return model.User{}, ErrInvalidCredentials
	}
	return user, nil
}

// ListCharacters devuelve los personajes de una cuenta.
func (s *Service) ListCharacters(ctx context.Context, userID int64) ([]model.CharacterSummary, error) {
	return s.store.ListCharacters(ctx, userID)
}

// CreateCharacter valida nombre/clase y crea el personaje con los stats base.
func (s *Service) CreateCharacter(ctx context.Context, userID int64, name, class string) (model.CharacterSummary, error) {
	name = strings.TrimSpace(name)
	if !characterPattern.MatchString(name) {
		return model.CharacterSummary{}, ErrInvalidCharacterName
	}
	def, ok := model.Classes[class]
	if !ok {
		return model.CharacterSummary{}, ErrInvalidClass
	}

	character, err := s.store.CreateCharacter(ctx, model.CreateCharacterParams{
		UserID: userID,
		Name:   name,
		Class:  class,
		Str:    def.Str,
		Agi:    def.Agi,
		Vit:    def.Vit,
		Ene:    def.Ene,
	})
	if err != nil {
		return model.CharacterSummary{}, err
	}
	return character.Summary(), nil
}

// DeleteCharacter borra un personaje propio.
func (s *Service) DeleteCharacter(ctx context.Context, userID, characterID int64) error {
	if characterID <= 0 {
		return model.ErrNotFound
	}
	return s.store.DeleteCharacter(ctx, userID, characterID)
}

// LoadCharacter carga un personaje propio para entrar al mundo.
func (s *Service) LoadCharacter(ctx context.Context, userID, characterID int64) (model.Character, error) {
	if characterID <= 0 {
		return model.Character{}, model.ErrNotFound
	}
	return s.store.LoadCharacter(ctx, userID, characterID)
}
