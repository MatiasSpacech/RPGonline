# MuNext

MMORPG 3D de acción **inspirado en MU Online** (dark fantasy, grind satisfactorio, mejora de ítems +N, click-to-move) pero con nombres, arte y diseño propios. Online a escala pequeña: **8–64 jugadores concurrentes por servidor**, con arquitectura que puede crecer después.

> Estado: **F4 completado** · drops, inventario 8×5, equipamiento con requisitos, mejora +N con riesgo, oro y vendedor. Siguiente: **F5 — mapas, clases y skills**.

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
6. **F5** – Mapas múltiples, clases y skills
7. **F6** – Chat, party, muerte, balance
8. **F7** – Despliegue y playtest con amigos

## Correr el juego (F4 — botín e ítems)

Requisitos: Go 1.27+ y Godot 4.7+ (instalados en `~/.local`; `~/.bashrc` ya agrega `~/.local/go/bin` y `~/.local/bin` al PATH — abrí una terminal nueva o `source ~/.bashrc`).

```bash
# Terminal 1 — base de datos (PostgreSQL 16 embebido, sin Docker ni sudo)
make devdb
#    esperá el mensaje "Postgres listo: postgres://..."

# Terminal 2 — servidor (aplica migraciones y carga shared/gamedata)
make server          # ws://127.0.0.1:8080/ws · healthz: :8080/healthz

# Terminal 3 — cliente Godot (una o dos ventanas)
godot --path client
```

Cada personaje nuevo recibe un **kit inicial**: arma de su clase, 10 pociones de HP, 10 de MP, 1 Núcleo de mejora y **500 de oro**.

Controles y loop:

- **Click en el suelo** → caminar. **Click en un mob** → auto-ataque.
- **Click en un drop** (caja brillante) → recogerlo; si estás lejos, tu personaje camina hasta él.
- **I** o **Inventario** → mochila 8×5, equipo, oro, detalle y tienda. Click en un ítem para ver sus stats/requisitos; arrastrá para reordenar.
- **Equipar** usa clase + requisitos de stats; **Mejorar** consume 1 Núcleo + oro: a partir de +7 un fallo baja un nivel y desde +10 hay 5% de destrucción.
- **Tienda**: comprá pociones/núcleos, vendé ítems (30% del valor, ajustado por rareza y mejora).
- Los mobs sueltan **oro e ítems** (loot tables en `shared/gamedata/`); las rarezas (común/bueno/raro/épico/legendario) se ven por color.
- Todo (inventario, equipo, mejoras y oro) persiste en PostgreSQL.

Tests:

```bash
make test            # unitarios + integración en memoria (sin DB)
make test-db         # incluye PostgreSQL real (con `make devdb` corriendo)
make client-autotest # headless: kit → equipar → mejorar → matar mob
```

Atajos: `make devdb`, `make server`, `make client`, `make test`, `make test-db`.

> **Docker** (opcional, para deploy): `docker compose -f deploy/docker-compose.yml up -d db` levanta un Postgres 16 equivalente. El entorno actual no lo tiene instalado, por eso `make devdb`.

