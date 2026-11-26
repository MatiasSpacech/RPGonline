# 00 — Resumen ejecutivo

## Qué vamos a construir

**MuNext**: un RPG online 3D de acción "estilo MU Online" a escala indie.

- **Jugador objetivo**: el que disfruta el *grind* — matar mobs, ver caer ítems, mejorar equipo de +3 a +9, subir niveles y presumir el equipo en la ciudad.
- **Escala**: 8–64 jugadores concurrentes por servidor en el MVP. Es un "online RPG", no un MMO de miles; la arquitectura se diseña para poder crecer, pero no se paga el costo de un MMO el día 1.
- **Equipo**: 1 persona (solo), en proceso de aprendizaje. El roadmap está pensado para eso: fases cortas, algo jugable al final de cada fase, y aprendizaje dirigido (cada fase enseña solo lo que esa fase necesita).

## Decisiones clave (y por qué)

| Decisión | Elección | Razón principal |
|---|---|---|
| Motor cliente | Godot 4 + GDScript | Gratis, ligero, 3D suficiente para este estilo, networking integrado (WebSocket/ENet), curva suave para una persona sola |
| Lenguaje servidor | Go | Concurrencia simple (goroutines), un solo binario para desplegar, rendimiento sobrado para 64 jugadores |
| Modelo de autoridad | Servidor autoritativo 100% | El cliente jamás decide daño, posición "oficial", loot ni stats. Es la base del anti-cheat |
| Transporte MVP | WebSocket + JSON | Ambos lados lo soportan nativo, es depurable con herramientas simples, y a 64 jugadores el overhead es irrelevante. UDP/binario queda como optimización posterior |
| Persistencia | PostgreSQL vía `pgx` | Relacional y aburrido de la mejor manera: cuentas, personajes e ítems encajan natural en tablas |
| Datos estáticos del juego | Archivos versionados en el repo (JSON), NO en la BD | Ítems, mobs, XP tables y skills son contenido: se versionan con el código y se comparten entre servidor y cliente |
| Tick del servidor | 20 Hz simulación, snapshots 10 Hz | Suficientemente responsivo para click-to-move RPG; barato en CPU y ancho de banda |
| MVP "online pequeño" | 1 proceso servidor, 3 mapas lógicos con snapshots filtrados por mapa | A 64 jugadores no hace falta interest management sofisticado; se documenta el camino de crecimiento |

## Lo que NO es (no-objetivos del MVP)

- No es un MMO con miles de jugadores, sharding ni clúster de servidores.
- No tiene PvP abierto, guilds, crafting ni tiendas entre jugadores en el MVP (ver post-MVP en [01-vision](01-vision.md)).
- No usa nombres, arte ni datos de MU Online: es *inspiración de sensación de juego*, no una copia.

## Mapa del documento

- **¿Qué juego es?** → [01-vision](01-vision.md)
- **¿Qué sistemas tiene?** → [02-diseno-de-juego](02-diseno-de-juego.md)
- **¿Cómo se conecta todo?** → [03-arquitectura](03-arquitectura.md)
- **¿Qué viaja por el cable?** → [04-red](04-red.md)
- **¿Qué se guarda en disco?** → [05-base-de-datos](05-base-datos.md)
- **¿Cómo evitamos tramposos y fallos de seguridad?** → [06-seguridad](06-seguridad.md)
- **¿Cómo organizo el repo y mi semana?** → [07-flujo-de-trabajo](07-flujo-de-trabajo.md)
- **¿Qué hago primero y en qué orden?** → [08-roadmap](08-roadmap.md)
- **¿Qué significa cada término?** → [09-glosario](09-glosario.md)

## Presupuesto aproximado

| Ítem | Costo |
|---|---|
| Godot, Go, Postgres, Caddy | 0 (software libre) |
| VPS (2 vCPU / 4 GB) | ~US$5–15/mes (solo necesario desde F7; antes todo corre local con Docker) |
| Assets 3D iniciales | 0 (Kenney, Quaternius, Poly Pizza, animaciones Mixamo) |
| Tiempo | Ver estimaciones por fase en [08-roadmap](08-roadmap.md). Regla de oro: part-time solo, algo jugable cada 2–6 semanas |
