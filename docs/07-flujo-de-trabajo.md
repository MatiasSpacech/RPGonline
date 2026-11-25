# 07 — Flujo de trabajo y herramientas

## 1. Estructura del repositorio

```
MuNext/
├── README.md
├── docs/                    # esta planificación
├── client/                  # proyecto Godot 4
│   ├── project.godot
│   ├── scenes/              # main.tscn, player.tscn, ui/*.tscn
│   ├── scripts/             # net/ (ws + mensajes), game/ (control, interp), ui/
│   ├── assets/              # models/, anims/, ui/, sfx/  (¡solo assets LICENCIADOS!)
│   └── export_presets.cfg   # builds PC (Linux/Windows)
├── server/                  # módulo Go
│   ├── cmd/munext/main.go
│   ├── internal/{world,combat,loot,netio,db,auth,gamedata}/
│   ├── migrations/          # SQL de golang-migrate
│   ├── go.mod
│   └── .air.toml            # hot-reload en dev (opcional)
├── shared/
│   ├── gamedata/            # contenido ESTÁTICO: items.json, mobs.json, skills.json,
│   │                        # maps/town.json, xptable.json, loot/*.json
│   └── docs/                # licencias de assets (LICENSES.md obligatorio)
├── tools/                   # scripts utilitarios (exportar datos a cliente, bot de stress)
└── deploy/
    ├── Caddyfile
    └── docker-compose.yml   # postgres (+ server en F7)
```

Un solo repo monolítico (sin submódulos): contenido y protocolo cambian juntos, y es una persona.

## 2. Convenciones

- **Git**: trunk-based (solo `main` + commits descriptivos). Para experimentos: `exp/<tema>`.
- **Mensajes de commit**: `F3: loot roll con rareza ponderada` — la fase del roadmap como prefijo ayuda a leer la historia del proyecto.
- **Go**: `gofmt`/`go vet` no negociables; errores como valores; interfaces chicas.
- **GDScript**: type hints siempre (`var x: int`), nombres `snake_case` (estándar Godot), señales para UI.
- **IDs de contenido**: `snake_case` semántico (`sword_iron`, `skeleton_rusty`, `skill_fireball`) — estables para siempre (se guardan en BD).
- Cada asset de terceros entra con su licencia registrada en `shared/docs/LICENSES.md` **el mismo día** que se descarga. Sin licencia = no entra.

## 3. Pipeline de assets 3D (¡la trampa de todo indie!)

Estrategia: **estilo estilizado low-poly, assets libres, tintado por código**.

| Necesidad | Fuente | Licencia típica |
|---|---|---|
| Sets completos de personajes low-poly | Kenney (kenney.nl), Quaternius (quaternius.com) | CC0 |
| Modelos sueltos / props | Poly Pizza (poly.pizza), Sketchfab (filtro descargable) | CC0/CC-BY (¡verificar!) |
| Animaciones humanoides | Mixamo (mixamo.com) | uso gratuito con cuenta |
| Edición/re-rig | Blender | GPL (irrelevante para assets) |
| Materiales de superficie | ambientCG | CC0 |

Reglas para no ahogarse:

1. **F1–F4 usan "placeholder honesto"**: cápsulas con nombre y color por clase. El arte se agrega DESPUÉS de que el loop funcione. El arte no bloquea ninguna fase de código.
2. Un solo "estilo pack" por categoría: todos los humanoides de la misma familia de assets (compatibilidad de rig).
3. Flujo: FBX/GLTF → Blender (escala a metros, pivote en pies, +Y arriba) → exportar GLTF 2.0 a `client/assets/models/`.
4. Animaciones mínimas: idle, run, attack, death (4 clips de Mixamo re-utilizados por rig).
5. Ítems equipables visibles: MVP = tintado del slot + "glow" para +7 (shader simple), modelos de armas sueltos solo si el tiempo sobra.

## 4. Bucle de desarrollo diario

```
1. docker compose up -d db          # Postgres local
2. go run ./cmd/munext              # server en :8080 (con -v para ver mensajes)
3. godot client/project.godot       # editor; F5 corre el juego
4. Segunda instancia de Godot (--path client -- --instance 2) para probar 2 jugadores
5. wscat / script de prueba        # cliente "invisible" para observar protocolo
```

Típico ciclo: cambiar server → reconectar cliente (2 min); cambiar contenido (`shared/gamedata`) → reiniciar server. Nada de rebuilds largos: Go y Godot compilan en segundos.

## 5. Testing (realista para solo-learning)

| Nivel | Qué | Cuándo |
|---|---|---|
| Unit (Go) | `combat`, `loot`, `xptable`: fórmulas puras con tablas de casos (crit/no-crit, rareza mínima, atenuación de XP) | cada feature numérica (F3–F6) |
| Integración (Go) | Levantar server en memoria + cliente fake: login→move→attack→matar mob→drop ok | fin de F2 y F3 |
| Stress | `tools/botstress`: 20 bots WebSocket que caminan y atacan 10 min; medir CPU y RAM | fin de F3 y antes de F7 |
| Manual (Godot) | Checklist por fase (abajo) | cada "terminé la fase" |
| Playtest | Amigos (2–4) en build exportada | F6 y F7 |

Checklist manual mínimo (crece por fase): crear cuenta y personaje, moverse con clic, ver al otro jugador moverse, atacar un mob y verlo morir, recoger drop, equipar, mejorar, comprar poción, chatear, morir y revivir, cerrar el cliente y re-loginear (estado persistido).

## 6. Herramientas

| Herramienta | Uso |
|---|---|
| Godot 4.x (última estable 4.3+) | editor cliente |
| Go 1.22+ | server |
| Docker + docker compose | Postgres local, deploy F7 |
| golang-migrate | migraciones SQL |
| wscat / Postman (WS) | inspección manual del protocolo |
| Git + GitHub (repo privado gratis) | control de versiones + backup off-site |
| Blender 4.x | preparación de assets |
| htop / `go tool pprof` | perf del server cuando toque |
| Caddy 2 | TLS proxy (F7) |

## 7. Aprendizaje dirigido (qué estudiar y CUÁNDO)

Estudiar **justo a tiempo**, no por adelantado (evita el tutorial-hell):

| Antes de... | Estudiar (mínimo viable) |
|---|---|
| F0 | Godot: nodos/escenas/señales (2–3 h). Go: tour of Go (2–3 h). Docker básico. |
| F1 | WebSocket en Go (ejemplo echo server), `WebSocketPeer` de Godot, el game-loop de [03-arquitectura §3](03-arquitectura.md) |
| F2 | pgx básico + SQL joins mínimos; Argon2 en Go; diseño del `welcome` |
| F3 | Game AI básica (FSM: estados idle/chase/attack); timeline/animation en Godot |
| F4 | Transacciones SQL; modelo de instancias de ítems ([05-base-de-datos](05-base-datos.md)) |
| F5 | Navmesh de Godot (para el cliente); export de datos estáticos al cliente |
| F6 | Interpolación y buffers de snapshot (deep dive de [04-red §4](04-red.md)) |
| F7 | systemd, Caddyfile, pg_dump, hardening básico SSH |

Cada tema: 1 tarde de lectura + implementarlo de inmediato en la fase que lo necesita.
