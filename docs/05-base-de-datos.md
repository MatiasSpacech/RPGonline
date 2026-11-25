# 05 — Base de datos

## 1. Principio de diseño

> **La BD guarda solo ESTADO dinámico. El CONTENIDO estático del juego vive en archivos versionados.**

- Estado dinámico (BD): cuentas, personajes, inventario, mejoras, posición.
- Contenido estático (`shared/gamedata/*.json` en el repo): plantillas de ítems, mobs, loot tables, skills, mapas. El servidor los carga al arrancar y los valida.

Beneficios: contenido se code-reviewa y versiona como código; la BD queda pequeña, simple y cacheable en RAM (el mundo carga personajes en memoria y el game loop nunca espera a la BD).

## 2. Modelo de datos

```
users 1───* characters 1───* items
                 │
                 └───* character_skills
```

### DDL de referencia (Postgres 16)

```sql
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,              -- argon2id con salt incluido
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login    TIMESTAMPTZ
);

CREATE TABLE characters (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id),
    name          TEXT NOT NULL UNIQUE,       -- nombre visible, único global
    class         TEXT NOT NULL,              -- 'warrior' | 'mage' | 'archer'
    level         INT  NOT NULL DEFAULT 1,
    xp            BIGINT NOT NULL DEFAULT 0,
    stat_str      INT  NOT NULL,
    stat_agi      INT  NOT NULL,
    stat_vit      INT  NOT NULL,
    stat_ene      INT  NOT NULL,
    stat_points   INT  NOT NULL DEFAULT 0,
    gold          BIGINT NOT NULL DEFAULT 0,
    map_id        TEXT NOT NULL,              -- id de archivo de mapa ('town')
    pos_x         REAL NOT NULL,
    pos_y         REAL NOT NULL,
    pos_z         REAL NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_played   TIMESTAMPTZ
);
CREATE INDEX idx_characters_user ON characters(user_id);

CREATE TABLE items (
    id            BIGSERIAL PRIMARY KEY,
    character_id  BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    tmpl          TEXT NOT NULL,              -- id de plantilla ('sword_iron')
    enhance       INT  NOT NULL DEFAULT 0,    -- nivel +N
    rarity        TEXT NOT NULL,              -- se fija al dropear, no cambia
    equipped_slot TEXT,                       -- NULL = en inventario; 'weapon','helm',...
    inv_x         INT,                        -- grilla 8x5 (NULL si equipped)
    inv_y         INT,
    qty           INT  NOT NULL DEFAULT 1     -- stackables (pociones, núcleos)
);
CREATE INDEX idx_items_character ON items(character_id);

CREATE TABLE character_skills (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    skill_id    TEXT NOT NULL,
    PRIMARY KEY (character_id, skill_id)
);
```

### Notas sobre este esquema

- `map_id` es `TEXT` (id del JSON de mapa), no FK: los mapas son contenido, no filas.
- `rarity` se guarda en la instancia porque se decide en el drop y afecta stats permanentemente.
- No hay tabla de "loot drops en el suelo": eso vive en RAM del servidor; si el server cae, los drops del piso se pierden (aceptable MVP).
- Post-MVP: `parties`/`guilds` (tabla propia), `trade_log` (auditoría), `chat_log` (moderación, con retención corta).

## 3. Acceso y patrón de persistencia

- Librería: **`pgx`** (driver puro Go, pool incluido: `pgxpool`).
- El **game loop nunca bloquea esperando la BD**: cuando guarda un personaje, encola el job al pool de guardado (2–4 goroutines) y sigue simulando. El resultado (ok/err) llega por canal y solo se loguea.
- Guardado:
  - **Cada 60 s** para personajes con cambios (dirty flag) — perder ≤60 s de progreso ante un crash es aceptable en MVP;
  - **inmediato** en eventos críticos: crear personaje, mejora de ítem (+N), level up, logout.
  - Transacciones: la mejora de ítem hace todo en una tx (leer materiales, descontar, aplicar resultado) para no perder ítems a mitad.
- Cache de personajes en memoria mientras están conectados; la BD es "snapshot + cola de guardado", no lectura continua.

## 4. Migraciones

- Herramienta: **golang-migrate** con archivos SQL versionados en `server/migrations/`.
- Reglas: migraciones solo hacia adelante (forward-only) en dev; cada migración se puede revertir durante el desarrollo pero nunca en producción sin backup.
- Ejemplo de flujo: F2 crea `0001_init.sql` con las tablas de arriba.

## 5. Operación

- **Dev**: Postgres en Docker Compose (`docker compose up db`) con volumen local; usuario/db `munext`.
- **Backup**: `pg_dump` diario programado (cron del VPS) a archivo + copia fuera del VPS (F7).
- **Passwords**: ver [06-seguridad](06-seguridad.md) — Argon2id, nunca MD5/SHA solos.
- No hay adminer/pgweb en producción; en dev, cualquier GUI local (DBeaver/pgAdmin) apuntando a localhost.
