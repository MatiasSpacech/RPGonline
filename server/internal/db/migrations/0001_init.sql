CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login    TIMESTAMPTZ
);

CREATE UNIQUE INDEX users_username_lower ON users (lower(username));

CREATE TABLE characters (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    class         TEXT NOT NULL,
    level         INT NOT NULL DEFAULT 1,
    xp            BIGINT NOT NULL DEFAULT 0,
    stat_str      INT NOT NULL,
    stat_agi      INT NOT NULL,
    stat_vit      INT NOT NULL,
    stat_ene      INT NOT NULL,
    stat_points   INT NOT NULL DEFAULT 0,
    gold          BIGINT NOT NULL DEFAULT 0,
    map_id        TEXT NOT NULL DEFAULT 'llanuras',
    pos_x         REAL NOT NULL DEFAULT 0,
    pos_y         REAL NOT NULL DEFAULT 0,
    pos_z         REAL NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_played   TIMESTAMPTZ
);

CREATE UNIQUE INDEX characters_name_lower ON characters (lower(name));
CREATE INDEX characters_user ON characters(user_id);

CREATE TABLE items (
    id            BIGSERIAL PRIMARY KEY,
    character_id  BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    tmpl          TEXT NOT NULL,
    enhance       INT NOT NULL DEFAULT 0,
    rarity        TEXT NOT NULL DEFAULT 'common',
    equipped_slot TEXT,
    inv_x         INT,
    inv_y         INT,
    qty           INT NOT NULL DEFAULT 1
);

CREATE INDEX items_character ON items(character_id);

CREATE TABLE character_skills (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    skill_id     TEXT NOT NULL,
    PRIMARY KEY (character_id, skill_id)
);
