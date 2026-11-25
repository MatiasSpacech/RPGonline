# MuNext

MMORPG 3D de acción **inspirado en MU Online** (dark fantasy, grind satisfactorio, mejora de ítems +N, click-to-move) pero con nombres, arte y diseño propios. Online a escala pequeña: **8–64 jugadores concurrentes por servidor**, con arquitectura que puede crecer después.

> Estado: **F1 completado** · game loop autoritativo a 20 Hz, snapshots a 10 Hz, login de dev, click-to-move e interpolación: dos clientes se ven moverse. Siguiente: **F2 — persistencia**.

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

1. **F0** – Setup de herramientas y aprendizaje dirigido
2. **F1** – "Walking simulator" online: conectar, ver a otros jugadores moverse
3. **F2** – Cuentas y personajes persistentes (login + Postgres)
4. **F3** – Combate contra mobs, XP y niveles
5. **F4** – Loot, inventario, equipamiento y mejora +N
6. **F5** – Mapas múltiples, clases y skills
7. **F6** – Chat, party, muerte, balance
8. **F7** – Despliegue y playtest con amigos

## Correr el juego (F1 — caminar juntos)

Requisitos: Go 1.27+ y Godot 4.7+ (instalados en `~/.local`; `~/.bashrc` ya agrega `~/.local/go/bin` y `~/.local/bin` al PATH — abrí una terminal nueva o `source ~/.bashrc`).

```bash
# Terminal 1 — servidor autoritativo
make server          # ws://127.0.0.1:8080/ws · healthz: :8080/healthz

# Terminal 2 — dos clientes (dos ventanas; ambas usan "dev")
godot --path client &
godot --path client &
```

Qué vas a ver:

- Una cápsula dorada (vos) y otra de color con su nombre flotante. El server reparte nombres únicos: el segundo `dev` pasa a `dev-2`.
- **Click izquierdo en el suelo** → tu personaje camina hasta el marcador verde.
- El otro jugador se mueve interpolado (buffer de ~100 ms sobre snapshots de 10 Hz).
- HUD: tick del servidor, jugadores conectados y RTT.
- Si cerrás el servidor, ambos clientes avisan y reintentan solos cada 3 s.

```bash
# Tests del servidor: protocolo, mundo y un test de integración con dos
# clientes WebSocket reales verificando que el movimiento se replica.
cd server && go test ./...
```

Atajos: `make server`, `make client`, `make test`.

> **Docker/Postgres**: se suman en F2 (cuentas y personajes persistentes).

