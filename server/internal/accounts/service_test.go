package accounts

import (
	"context"
	"errors"
	"testing"

	"munext/server/internal/auth"
	"munext/server/internal/model"
)

// fakeStore es una implementación en memoria mínima para testear validaciones.
type fakeStore struct {
	user      model.User
	hash      string
	created   model.CreateCharacterParams
	charError error
}

func (f *fakeStore) CreateUser(_ context.Context, username, passwordHash string) (model.User, error) {
	return model.User{ID: 1, Username: username}, nil
}

func (f *fakeStore) GetUserByUsername(_ context.Context, username string) (model.User, string, error) {
	if f.user.ID == 0 || f.user.Username != username {
		return model.User{}, "", model.ErrNotFound
	}
	return f.user, f.hash, nil
}

func (f *fakeStore) ListCharacters(context.Context, int64) ([]model.CharacterSummary, error) {
	return nil, nil
}

func (f *fakeStore) CreateCharacter(_ context.Context, p model.CreateCharacterParams) (model.Character, error) {
	if f.charError != nil {
		return model.Character{}, f.charError
	}
	f.created = p
	return model.Character{ID: 10, UserID: p.UserID, Name: p.Name, Class: p.Class, Level: 1}, nil
}

func (f *fakeStore) DeleteCharacter(context.Context, int64, int64) error { return nil }

func (f *fakeStore) LoadCharacter(context.Context, int64, int64) (model.Character, error) {
	return model.Character{}, model.ErrNotFound
}

func TestRegisterValidationAndLogin(t *testing.T) {
	svc := New(&fakeStore{})

	if _, err := svc.Register(context.Background(), "ab", "123456"); !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("usuario corto: err = %v, quería ErrInvalidUsername", err)
	}
	if _, err := svc.Register(context.Background(), "jugador", "12345"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("password corta: err = %v, quería ErrWeakPassword", err)
	}

	user, err := svc.Register(context.Background(), "jugador", "123456")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if user.ID != 1 || user.Username != "jugador" {
		t.Fatalf("usuario = %+v", user)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc := New(&fakeStore{})
	user, err := svc.Register(context.Background(), "jugador", "123456")
	if err != nil {
		t.Fatal(err)
	}

	// Simula el store con el hash generado con los parámetros reales.
	hash, err := auth.HashPassword("123456")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	store := &fakeStore{user: user, hash: hash}
	svc = New(store)

	got, err := svc.Login(context.Background(), "jugador", "123456")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("login devolvió %+v, quería %+v", got, user)
	}

	if _, err := svc.Login(context.Background(), "jugador", "incorrecta"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("password incorrecta: err = %v, quería ErrInvalidCredentials", err)
	}
	if _, err := svc.Login(context.Background(), "otro", "123456"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("usuario inexistente: err = %v, quería ErrInvalidCredentials", err)
	}
}

func TestCreateCharacterUsesClassDefaults(t *testing.T) {
	store := &fakeStore{}
	svc := New(store)

	if _, err := svc.CreateCharacter(context.Background(), 1, "Ka", model.ClassWarrior); !errors.Is(err, ErrInvalidCharacterName) {
		t.Fatalf("nombre corto: err = %v, quería ErrInvalidCharacterName", err)
	}
	if _, err := svc.CreateCharacter(context.Background(), 1, "Kaen", "paladin"); !errors.Is(err, ErrInvalidClass) {
		t.Fatalf("clase inválida: err = %v, quería ErrInvalidClass", err)
	}

	summary, err := svc.CreateCharacter(context.Background(), 1, "Kaen", model.ClassWarrior)
	if err != nil {
		t.Fatalf("CreateCharacter: %v", err)
	}
	if summary.Name != "Kaen" || summary.Class != model.ClassWarrior {
		t.Fatalf("summary = %+v", summary)
	}

	def := model.Classes[model.ClassWarrior]
	if store.created.Str != def.Str || store.created.Agi != def.Agi || store.created.Vit != def.Vit || store.created.Ene != def.Ene {
		t.Fatalf("stats aplicados = %+v, quería %+v", store.created, def)
	}
}

func TestCreateCharacterPropagatesLimit(t *testing.T) {
	store := &fakeStore{charError: model.ErrCharacterLimit}
	svc := New(store)
	if _, err := svc.CreateCharacter(context.Background(), 1, "Kaen", model.ClassWarrior); !errors.Is(err, model.ErrCharacterLimit) {
		t.Fatalf("err = %v, quería ErrCharacterLimit", err)
	}
}
