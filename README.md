# MuNext

MMORPG 3D de acción **inspirado en MU Online** (dark fantasy, grind satisfactorio, mejora de ítems +N, click-to-move) pero con nombres, arte y diseño propios. Online a escala pequeña: **8–64 jugadores concurrentes por servidor**, con arquitectura que puede crecer después.

> Estado: **F5 completado** (mecánicas) · 3 mapas con portales y safe zone, mobs de mazmorra, bosses con mecánicas y 9 skills con maná/cooldown. Arte low-poly real aún pendiente. Siguiente: **F6 — social y game feel**.

## Stack decidido

| Capa | Tecnología | Rol |
|---|---|---|
| Cliente 3D | **Godot 4** (GDScript) | Render, input, HUD, interpolación |
| Servidor | **Go** (binario único) | Simulación autoritativa: combate, drops, movimiento, persistencia |
| Transporte | **WebSocket** (JSON al inicio) | Comunicación cliente ⇄ servidor |
| Base de datos | **PostgreSQL** | Cuentas, personajes, ítems |
| Despliegue | 1 VPS + Caddy (TLS) + Docker Compose | Producción mínima viable |

## Documentación

| Doc | Contenido |
|---|---|
| [00-resumen](docs/00-resumen.md) | Resumen ejecutivo y mapa de decisiones |
| [01-vision](docs/01-vision.md) | Visión, pilares, alcance MVP, no-objetivos, nota legal |
| [02-diseno-de-juego](docs/02-diseno-de-juego.md) | GDD: clases, stats, combate, ítems +N, mobs, mapas, skills |
| [03-arquitectura](docs/03-arquitectura.md) | Topología, modelo de procesos, game loop, decisiones técnicas |
| [04-red](docs/04-red.md) | Protocolo de mensajes, sincronización, validación de movimiento |
| [05-base-de-datos](docs/05-base-de-datos.md) | Esquema Postgres, principio "datos estáticos en archivos" |
| [06-seguridad](docs/06-seguridad.md) | Autenticación, anti-cheat, límites de confianza |
| [07-flujo-de-trabajo](docs/07-flujo-de-trabajo.md) | Estructura del repo, pipeline de assets, testing, herramientas |
| [08-roadmap](docs/08-roadmap.md) | Plan por fases con criterios de aceptación y riesgos |
| [09-glosario](docs/09-glosario.md) | Términos de netcode/gamedev EN→ES |

## Ruta rápida (resumen del roadmap)

1. **F0 ✅** – Setup de herramientas y aprendizaje dirigido
2. **F1 ✅** – "Walking simulator" online: conectar, ver a otros jugadores moverse
3. **F2 ✅** – Cuentas y personajes persistentes (login + Postgres)
4. **F3 ✅** – Combate contra mobs, XP y niveles
5. **F4 ✅** – Loot, inventario, equipamiento y mejora +N
6. **F5 ✅** – Mapas múltiples, clases y skills
7. **F6** – Chat, party, muerte, balance
8. **F7** – Despliegue y playtest con amigos

## Correr el juego (F5 — mapas, clases y skills)

Requisitos: Go 1.27+ y Godot 4.7+ (instalados en `~/.local`; `~/.bashrc` ya agrega `~/.local/go/bin` y `~/.local/bin` al PATH — abrí una terminal nueva o `source ~/.bashrc`).

```bash
# Terminal 1 — base de datos (PostgreSQL 16 embebido, sin Docker ni sudo)
make devdb

# Terminal 2 — servidor (migraciones + shared/gamedata)
make server          # ws://127.0.0.1:8080/ws

# Terminal 3 — cliente Godot
godot --path client
```

Cada personaje nuevo aparece en **Refugio de Piedra** (zona segura) con su kit inicial.

Mapas y loop:

- **Refugio de Piedra**: hub seguro (no se puede atacar), vendedor e inventario. Portal oeste → **Llanuras Quebradas**; portal este → **Catacumbas de Óxido** (requiere nivel 13).
- **Llanuras**: campo abierto con babosas, lobos, bandidos, escorpiones y el **Golem** (se enfurece bajo 30% de HP).
- **Catacumbas**: esqueletos, cultistas, caballeros y **El Carcelero**, que invoca esqueletos al 50% de HP.
- **Click en un portal** para viajar; tu mapa y posición se guardan. Al morir revivís en la ciudad.
- **Skills**: Q/W/E/R o la barra inferior. Se desbloquean por nivel; consumen maná y tienen cooldown.
  - Guerrero: Tajo (3), Grito (10), Remolino (18).
  - Mago: Bola de Fuego (1), Nova (8), Teletransporte (14).
  - Arquera: Flecha Triple (3), Curativa (12), Multidisparo (20).
- Inventario (**I**), equipo, mejoras +N y tienda siguen funcionando igual que en F4.

> El arte sigue siendo procedural/low-poly simple (cápsulas + geometría temática). El reemplazo por modelos animados quedó diferido a la fase de pulido.

Tests:

```bash
make test            # unitarios + integración en memoria (sin DB)
make test-db         # incluye PostgreSQL real (con `make devdb` corriendo)
make client-autotest # headless: kit → equipar → mejorar → portal → matar mob
```

Atajos: `make devdb`, `make server`, `make client`, `make test`, `make test-db`.

> **Docker** (opcional, para deploy): `docker compose -f deploy/docker-compose.yml up -d db` levanta un Postgres 16 equivalente. El entorno actual no lo tiene instalado, por eso `make devdb`.

