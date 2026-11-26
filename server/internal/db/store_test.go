package db

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"munext/server/internal/auth"
	"munext/server/internal/model"
)

// testStore se conecta a la BD indicada por MUNEXT_TEST_DATABASE_URL y deja
// las tablas vacías. Si la variable no está definida, el test se saltea.
func testStore(t *testing.T) *Store {
	t.Helper()

	dsn := os.Getenv("MUNEXT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("MUNEXT_TEST_DATABASE_URL no definida: se omite el test de base de datos")
	}

	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := Migrate(ctx, dsn, log); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	pool, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `TRUNCATE users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("limpiando tablas: %v", err)
	}
	return NewStore(pool)
}

func newUser(t *testing.T, s *Store, username string) model.User {
	t.Helper()
	hash, err := auth.HashPassword("123456")
	if err != nil {
		t.Fatal(err)
	}
	user, err := s.CreateUser(context.Background(), username, hash)
	if err != nil {
		t.Fatalf("CreateUser(%q): %v", username, err)
	}
	return user
}

func TestUserLifecycle(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	user := newUser(t, s, "matias")

	if _, err := s.CreateUser(ctx, "MATIAS", "x"); !errors.Is(err, model.ErrUsernameTaken) {
		t.Fatalf("usuario duplicado: err = %v, quería ErrUsernameTaken", err)
	}

	got, hash, err := s.GetUserByUsername(ctx, "MaTiAs")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if got.ID != user.ID || hash == "" {
		t.Fatalf("usuario = %+v, hash vacío=%v", got, hash == "")
	}

	if _, _, err := s.GetUserByUsername(ctx, "noexiste"); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("usuario inexistente: err = %v, quería ErrNotFound", err)
	}
}

func TestCharacterLifecycle(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	user := newUser(t, s, "matias")

	chars := make([]model.CharacterSummary, 0, 3)
	for _, name := range []string{"Kaen", "Lyr", "Nyx"} {
		c, err := s.CreateCharacter(ctx, model.CreateCharacterParams{
			UserID: user.ID, Name: name, Class: model.ClassWarrior,
			Str: 28, Agi: 20, Vit: 25, Ene: 10,
			Gold: 500,
			StarterItems: []model.StarterItem{
				{Tmpl: "sword_iron", Qty: 1, InvX: 0, InvY: 0},
				{Tmpl: "pot_hp", Qty: 10, InvX: 1, InvY: 0},
			},
		})
		if err != nil {
			t.Fatalf("CreateCharacter(%s): %v", name, err)
		}
		if c.ID == 0 || c.UserID != user.ID || c.Level != 1 || c.MapID != "refugio" {
			t.Fatalf("personaje = %+v", c)
		}
		if c.Gold != 500 || len(c.Items) != 2 {
			t.Fatalf("kit inicial = gold %d, items %d", c.Gold, len(c.Items))
		}
		for _, it := range c.Items {
			if it.ID == 0 || it.CharacterID != c.ID || it.Qty < 1 {
				t.Fatalf("ítem inicial inválido = %+v", it)
			}
		}
		chars = append(chars, c.Summary())
	}

	if _, err := s.CreateCharacter(ctx, model.CreateCharacterParams{
		UserID: user.ID, Name: "Cuarto", Class: model.ClassMage,
	}); !errors.Is(err, model.ErrCharacterLimit) {
		t.Fatalf("cuarto personaje: err = %v, quería ErrCharacterLimit", err)
	}

	other := newUser(t, s, "otro")
	if _, err := s.CreateCharacter(ctx, model.CreateCharacterParams{
		UserID: other.ID, Name: "kaen", Class: model.ClassMage,
	}); !errors.Is(err, model.ErrCharacterNameTaken) {
		t.Fatalf("nombre duplicado: err = %v, quería ErrCharacterNameTaken", err)
	}

	list, err := s.ListCharacters(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListCharacters: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("personajes = %d, quería 3", len(list))
	}

	// Cargar con dueño incorrecto no debe exponer el personaje.
	if _, err := s.LoadCharacter(ctx, other.ID, chars[0].ID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("carga ajena: err = %v, quería ErrNotFound", err)
	}

	loaded, err := s.LoadCharacter(ctx, user.ID, chars[0].ID)
	if err != nil {
		t.Fatalf("LoadCharacter: %v", err)
	}
	if loaded.Str != 28 || loaded.Agi != 20 || loaded.Vit != 25 || loaded.Ene != 10 {
		t.Fatalf("stats cargados = %+v", loaded)
	}
	if len(loaded.Items) != 2 {
		t.Fatalf("inventario cargado = %d ítems, quería 2", len(loaded.Items))
	}

	// Equipar la espada y mejorarla, luego guardar posición + progreso + ítems.
	loaded.Items[0].EquippedSlot = "weapon"
	loaded.Items[0].Enhance = 3
	if err := s.SaveCharacterState(ctx, model.CharacterState{
		CharacterID: loaded.ID, MapID: "llanuras", X: 12.5, Y: 0, Z: -3.25,
		Level: 3, XP: 42, Str: 33, Agi: 21, Vit: 30, Ene: 12,
		StatPoints: 2, Gold: 123, Items: loaded.Items,
	}); err != nil {
		t.Fatalf("SaveCharacterState: %v", err)
	}
	reloaded, err := s.LoadCharacter(ctx, user.ID, chars[0].ID)
	if err != nil {
		t.Fatalf("recargando: %v", err)
	}
	if reloaded.X != 12.5 || reloaded.Z != -3.25 {
		t.Fatalf("posición guardada = (%v, %v)", reloaded.X, reloaded.Z)
	}
	if reloaded.Level != 3 || reloaded.XP != 42 || reloaded.Str != 33 || reloaded.StatPoints != 2 || reloaded.Gold != 123 {
		t.Fatalf("progreso guardado = %+v", reloaded)
	}
	if len(reloaded.Items) != 2 {
		t.Fatalf("ítems guardados = %d, quería 2", len(reloaded.Items))
	}
	weapon := reloaded.Items[0]
	if weapon.EquippedSlot != "weapon" || weapon.Enhance != 3 {
		t.Fatalf("arma guardada = %+v", weapon)
	}

	if err := s.DeleteCharacter(ctx, user.ID, chars[0].ID); err != nil {
		t.Fatalf("DeleteCharacter: %v", err)
	}
	if _, err := s.LoadCharacter(ctx, user.ID, chars[0].ID); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("personaje borrado: err = %v, quería ErrNotFound", err)
	}
}

func TestSaveUnknownCharacter(t *testing.T) {
	s := testStore(t)
	if err := s.SaveCharacterState(context.Background(), model.CharacterState{CharacterID: 9999}); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("err = %v, quería ErrNotFound", err)
	}
}
