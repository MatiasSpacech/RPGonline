# MuNext

MMORPG 3D de acción **inspirado en MU Online** (dark fantasy, grind satisfactorio, mejora de ítems +N, click-to-move) pero con nombres, arte y diseño propios. Online a escala pequeña: **8–64 jugadores concurrentes por servidor**, con arquitectura que puede crecer después.

> Estado: **F0 completado** · cliente Godot ⇄ servidor Go por WebSocket (ping/pong, caída y reconexión automática verificados). Siguiente: **F1 — caminar juntos**.

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

## Correr F0 (hola red)

Requisitos: Go 1.27+ y Godot 4.7+ (ambos instalados en `~/.local` en esta máquina; `~/.bashrc` ya agrega `~/.local/go/bin` y `~/.local/bin` al PATH — abrí una terminal nueva o `source ~/.bashrc`).

```bash
# 1. Servidor (desde la raíz del repo)
cd server && go run ./cmd/munext
#    escucha en ws://127.0.0.1:8080/ws  ·  healthcheck: http://127.0.0.1:8080/healthz

# 2. Cliente (en otra terminal)
godot --path client
#    o abrí client/project.godot en el editor y presioná F5
```

Qué vas a ver: el cliente muestra `Conectado ✓ · pong #N · RTT X ms` y la consola del servidor registra `cliente conectado`. Si cerrás el servidor, el cliente muestra el error y reintenta solo cada 3 s.

```bash
# Tests del servidor (protocolo + integración ping/pong)
cd server && go test ./...
```

Atajos equivalentes: `make server`, `make client`, `make test`.

> **Docker/Postgres**: se instalan en F2 (persistencia). No son necesarios para F0.

