package netio

import (
	"context"
	"sort"
	"strings"
	"sync"

	"munext/server/internal/accounts"
	"munext/server/internal/model"
)

// fakeAccounts es una implementación en memoria de Accounts para tests.
// No hashea contraseñas: la seguridad real se testea en auth/accounts.
type fakeAccounts struct {
	mu         sync.Mutex
	nextUserID int64
	nextCharID int64
	users      map[string]fakeUser // por username en minúsculas
	chars      map[int64]model.Character
}

type fakeUser struct {
	user model.User
	pass string
}

func newFakeAccounts() *fakeAccounts {
	return &fakeAccounts{
		users: make(map[string]fakeUser),
		chars: make(map[int64]model.Character),
	}
}

func (f *fakeAccounts) addUser(username, password string) model.User {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.addUserLocked(username, password)
}

func (f *fakeAccounts) addUserLocked(username, password string) model.User {
	f.nextUserID++
	user := model.User{ID: f.nextUserID, Username: username}
	f.users[strings.ToLower(username)] = fakeUser{user: user, pass: password}
	return user
}

func (f *fakeAccounts) addCharacter(userID int64, name, class string) model.Character {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.addCharacterLocked(userID, name, class)
}

func (f *fakeAccounts) addCharacterLocked(userID int64, name, class string) model.Character {
	def := model.Classes[class]
	f.nextCharID++
	c := model.Character{
		ID:     f.nextCharID,
		UserID: userID,
		Name:   name,
		Class:  class,
		Level:  1,
		Str:    def.Str,
		Agi:    def.Agi,
		Vit:    def.Vit,
		Ene:    def.Ene,
		MapID:  "llanuras",
	}
	f.chars[c.ID] = c
	return c
}

func (f *fakeAccounts) Register(_ context.Context, username, password string) (model.User, error) {
	if len(username) < 3 {
		return model.User{}, accounts.ErrInvalidUsername
	}
	if len(password) < 6 {
		return model.User{}, accounts.ErrWeakPassword
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.users[strings.ToLower(username)]; exists {
		return model.User{}, model.ErrUsernameTaken
	}
	return f.addUserLocked(username, password), nil
}

func (f *fakeAccounts) Login(_ context.Context, username, password string) (model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[strings.ToLower(username)]
	if !ok || u.pass != password {
		return model.User{}, accounts.ErrInvalidCredentials
	}
	return u.user, nil
}

func (f *fakeAccounts) ListCharacters(_ context.Context, userID int64) ([]model.CharacterSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	chars := make([]model.CharacterSummary, 0, 3)
	for _, c := range f.chars {
		if c.UserID == userID {
			chars = append(chars, c.Summary())
		}
	}
	sort.Slice(chars, func(i, j int) bool { return chars[i].ID < chars[j].ID })
	return chars, nil
}

func (f *fakeAccounts) CreateCharacter(_ context.Context, userID int64, name, class string) (model.CharacterSummary, error) {
	if _, ok := model.Classes[class]; !ok {
		return model.CharacterSummary{}, accounts.ErrInvalidClass
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	count := 0
	for _, c := range f.chars {
		if c.UserID == userID {
			count++
		}
		if strings.EqualFold(c.Name, name) {
			return model.CharacterSummary{}, model.ErrCharacterNameTaken
		}
	}
	if count >= 3 {
		return model.CharacterSummary{}, model.ErrCharacterLimit
	}
	return f.addCharacterLocked(userID, name, class).Summary(), nil
}

func (f *fakeAccounts) DeleteCharacter(_ context.Context, userID, characterID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.chars[characterID]
	if !ok || c.UserID != userID {
		return model.ErrNotFound
	}
	delete(f.chars, characterID)
	return nil
}

func (f *fakeAccounts) LoadCharacter(_ context.Context, userID, characterID int64) (model.Character, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.chars[characterID]
	if !ok || c.UserID != userID {
		return model.Character{}, model.ErrNotFound
	}
	return c, nil
}
