package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"munext/server/internal/model"
)

// Store implementa las consultas de cuentas y personajes.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore envuelve un pool ya abierto.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateUser inserta una cuenta nueva.
func (s *Store) CreateUser(ctx context.Context, username, passwordHash string) (model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id, username`,
		username, passwordHash,
	).Scan(&u.ID, &u.Username)
	if err != nil {
		return model.User{}, mapUniqueViolation(err)
	}
	return u, nil
}

// GetUserByUsername busca una cuenta sin distinguir mayúsculas/minúsculas.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (model.User, string, error) {
	var u model.User
	var hash string
	err := s.pool.QueryRow(ctx,
		`SELECT id, username, password_hash FROM users WHERE lower(username) = lower($1)`,
		username,
	).Scan(&u.ID, &u.Username, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, "", model.ErrNotFound
	}
	if err != nil {
		return model.User{}, "", fmt.Errorf("db: buscando usuario: %w", err)
	}
	return u, hash, nil
}

// TouchLastLogin actualiza la fecha de último login.
func (s *Store) TouchLastLogin(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET last_login = now() WHERE id = $1`, userID)
	return err
}

// ListCharacters devuelve los personajes de una cuenta (máximo 3).
func (s *Store) ListCharacters(ctx context.Context, userID int64) ([]model.CharacterSummary, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, class, level FROM characters WHERE user_id = $1 ORDER BY created_at, id`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("db: listando personajes: %w", err)
	}
	defer rows.Close()

	chars := make([]model.CharacterSummary, 0, 3)
	for rows.Next() {
		var c model.CharacterSummary
		if err := rows.Scan(&c.ID, &c.Name, &c.Class, &c.Level); err != nil {
			return nil, fmt.Errorf("db: leyendo personaje: %w", err)
		}
		chars = append(chars, c)
	}
	return chars, rows.Err()
}

// CreateCharacter inserta un personaje respetando el límite de 3 por cuenta.
// Bloquea la fila del usuario para serializar creaciones concurrentes.
func (s *Store) CreateCharacter(ctx context.Context, p model.CreateCharacterParams) (model.Character, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return model.Character{}, fmt.Errorf("db: iniciando transacción: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, p.UserID).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Character{}, model.ErrNotFound
		}
		return model.Character{}, fmt.Errorf("db: bloqueando usuario: %w", err)
	}

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM characters WHERE user_id = $1`, p.UserID).Scan(&count); err != nil {
		return model.Character{}, fmt.Errorf("db: contando personajes: %w", err)
	}
	if count >= 3 {
		return model.Character{}, model.ErrCharacterLimit
	}

	var c model.Character
	err = tx.QueryRow(ctx, `
		INSERT INTO characters (user_id, name, class, stat_str, stat_agi, stat_vit, stat_ene, gold)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, name, class, level, xp, stat_str, stat_agi, stat_vit, stat_ene,
		          stat_points, gold, map_id, pos_x, pos_y, pos_z`,
		p.UserID, p.Name, p.Class, p.Str, p.Agi, p.Vit, p.Ene, p.Gold,
	).Scan(
		&c.ID, &c.UserID, &c.Name, &c.Class, &c.Level, &c.XP, &c.Str, &c.Agi, &c.Vit, &c.Ene,
		&c.StatPoints, &c.Gold, &c.MapID, &c.X, &c.Y, &c.Z,
	)
	if err != nil {
		return model.Character{}, mapUniqueViolation(err)
	}

	// Kit inicial: armas/pociones/núcleos. Los IDs los asigna la secuencia.
	for _, starter := range p.StarterItems {
		var item model.Item
		err := tx.QueryRow(ctx, `
			INSERT INTO items (character_id, tmpl, enhance, rarity, inv_x, inv_y, qty)
			VALUES ($1, $2, 0, 'common', $3, $4, $5)
			RETURNING id, character_id, tmpl, enhance, rarity, COALESCE(equipped_slot, ''), COALESCE(inv_x, -1), COALESCE(inv_y, -1), qty`,
			c.ID, starter.Tmpl, starter.InvX, starter.InvY, starter.Qty,
		).Scan(
			&item.ID, &item.CharacterID, &item.Tmpl, &item.Enhance, &item.Rarity,
			&item.EquippedSlot, &item.InvX, &item.InvY, &item.Qty,
		)
		if err != nil {
			return model.Character{}, fmt.Errorf("db: insertando ítem inicial: %w", err)
		}
		c.Items = append(c.Items, item)
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Character{}, fmt.Errorf("db: confirmando personaje: %w", err)
	}
	return c, nil
}

// DeleteCharacter borra un personaje que pertenezca al usuario.
func (s *Store) DeleteCharacter(ctx context.Context, userID, characterID int64) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM characters WHERE id = $1 AND user_id = $2`, characterID, userID)
	if err != nil {
		return fmt.Errorf("db: borrando personaje: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}

// LoadCharacter carga un personaje (con su inventario) verificando que sea
// del usuario.
func (s *Store) LoadCharacter(ctx context.Context, userID, characterID int64) (model.Character, error) {
	var c model.Character
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, name, class, level, xp, stat_str, stat_agi, stat_vit, stat_ene,
		       stat_points, gold, map_id, pos_x, pos_y, pos_z
		FROM characters WHERE id = $1 AND user_id = $2`,
		characterID, userID,
	).Scan(
		&c.ID, &c.UserID, &c.Name, &c.Class, &c.Level, &c.XP, &c.Str, &c.Agi, &c.Vit, &c.Ene,
		&c.StatPoints, &c.Gold, &c.MapID, &c.X, &c.Y, &c.Z,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Character{}, model.ErrNotFound
	}
	if err != nil {
		return model.Character{}, fmt.Errorf("db: cargando personaje: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, character_id, tmpl, enhance, rarity,
		       COALESCE(equipped_slot, ''), COALESCE(inv_x, -1), COALESCE(inv_y, -1), qty
		FROM items WHERE character_id = $1 ORDER BY id`,
		characterID,
	)
	if err != nil {
		return model.Character{}, fmt.Errorf("db: cargando inventario: %w", err)
	}
	defer rows.Close()

	c.Items = make([]model.Item, 0, 8)
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.CharacterID, &it.Tmpl, &it.Enhance, &it.Rarity,
			&it.EquippedSlot, &it.InvX, &it.InvY, &it.Qty); err != nil {
			return model.Character{}, fmt.Errorf("db: leyendo ítem: %w", err)
		}
		c.Items = append(c.Items, it)
	}
	if err := rows.Err(); err != nil {
		return model.Character{}, fmt.Errorf("db: leyendo inventario: %w", err)
	}
	return c, nil
}

// SaveCharacterState persiste posición, progreso e inventario. El inventario
// se reemplaza completo dentro de la misma transacción (snapshot atómico).
func (s *Store) SaveCharacterState(ctx context.Context, st model.CharacterState) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: iniciando guardado: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE characters
		SET map_id = $2, pos_x = $3, pos_y = $4, pos_z = $5,
		    level = $6, xp = $7, stat_str = $8, stat_agi = $9, stat_vit = $10, stat_ene = $11,
		    stat_points = $12, gold = $13, last_played = now()
		WHERE id = $1`,
		st.CharacterID, st.MapID, st.X, st.Y, st.Z,
		st.Level, st.XP, st.Str, st.Agi, st.Vit, st.Ene, st.StatPoints, st.Gold)
	if err != nil {
		return fmt.Errorf("db: guardando personaje %d: %w", st.CharacterID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("db: guardando personaje %d: %w", st.CharacterID, model.ErrNotFound)
	}

	if st.Items != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM items WHERE character_id = $1`, st.CharacterID); err != nil {
			return fmt.Errorf("db: limpiando inventario %d: %w", st.CharacterID, err)
		}
		for _, it := range st.Items {
			if it.ID == 0 {
				return fmt.Errorf("db: ítem sin id en personaje %d", st.CharacterID)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO items (id, character_id, tmpl, enhance, rarity, equipped_slot, inv_x, inv_y, qty)
				VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8, $9)`,
				it.ID, st.CharacterID, it.Tmpl, it.Enhance, it.Rarity,
				it.EquippedSlot, it.InvX, it.InvY, it.Qty,
			); err != nil {
				return fmt.Errorf("db: guardando ítem %d: %w", it.ID, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: confirmando guardado %d: %w", st.CharacterID, err)
	}
	return nil
}

func mapUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_username_lower":
			return model.ErrUsernameTaken
		case "characters_name_lower":
			return model.ErrCharacterNameTaken
		}
	}
	return err
}
