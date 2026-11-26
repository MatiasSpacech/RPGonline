# 08 — Roadmap

> Diseñado para **una persona a tiempo parcial que aprende**. La unidad de avance es la *semana con ~6–10 h*. Las estimaciones asumen ese ritmo; si tenés una semana de 20 h, dividí por 2. **Regla inquebrantable: al final de cada fase hay algo que SE PUEDE EJECUTAR Y JUGAR.**

## Vista global

| Fase | Entregable jugable | Estimación |
|---|---|---|
| **F0** Setup | "Hola mundo" técnico: cliente conecta al server | 1–2 sem |
| **F1** Caminar juntos | 2+ jugadores se ven moverse en un mapa gris | 2–3 sem |
| **F2** Persistencia | Cuentas, personajes, login guardado en Postgres | 2–3 sem |
| **F3** Combate | Matás mobs que respawnean, ganás XP y niveles | 3–5 sem |
| **F4** Botín | Drops, inventario, equipar, mejora +N | 4–6 sem |
| **F5** Mundo | 3 mapas, 3 clases con skills, portales, vendedor | 4–6 sem |
| **F6** Social + feel | Chat, party, muerte, interpolación fina, balance | 3–4 sem |
| **F7** Producción | Deploy en VPS, playtest con amigos | 1–2 sem |
| **Total MVP** | | **≈ 5–7 meses** part-time |

El orden no es arbitrario: cada fase construye sobre la anterior y **cada una deja una demo donde "tocar" lo recién hecho**. F3–F4 son el corazón del juego: si hay que recortar, se recorta F5, nunca F3–F4.

---

## F0 — Setup y primer "hola red" (1–2 sem)

**Objetivo**: que las herramientas no sean excusa nunca más.

- [x] Instalar Godot 4.7 + Go 1.27 + Git, repo inicializado con `.gitignore` (instalados en `~/.local`, sin sudo).
- [ ] Docker + Blender — **diferidos**: Docker se necesita desde F2 (Postgres) y Blender desde F5 (assets).
- [x] Server Go: WebSocket ping/pong + `healthz` + apagado limpio + tests unitarios/integración.
- [x] Cliente Godot: escena 3D con cápsula, `WebSocketPeer`, estado y RTT en pantalla, reintento automático cada 3 s.
- [ ] Estudio: tour de Go + Godot básico (pendiente del jugador; ver [07 §7](07-flujo-de-trabajo.md)).

**Criterio de aceptación**: ✅ verificado headless de punta a punta (conexión, pong, caída del server y reconexión sola). Falta tu prueba interactiva con `godot --path client`.

## F1 — Caminar juntos (2–3 sem)

**Objetivo**: el "hello world" del netcode — dos clientes se ven mutuamente.

- [x] Protocolo F1: `login` (nombre "dev"), `move_req`, `welcome`, `snapshot` ([04-red](04-red.md) §3–4).
- [x] Game loop autoritativo 20 Hz + snapshots 10 Hz, un mapa plano con entidades de jugador.
- [x] Cliente: click-to-move por raycast al suelo, interpolación entre snapshots (~100 ms), nombres y colores por jugador.
- [x] Reconexión automática tras caída del server (heredada de F0) + re-login al reconectar.

**Criterio de aceptación**: ✅ verificado con dos clientes headless viéndose mutuamente y test de integración Go con movimiento replicado por snapshot + `-race` limpio. Falta tu prueba interactiva con dos ventanas.

## F2 — Persistencia: cuentas y personajes (2–3 sem)

- [x] PostgreSQL 16 embebido para dev (`make devdb`, sin Docker ni sudo) + `deploy/docker-compose.yml` para deploy; migración `0001_init` con el esquema de [05-base-de-datos](05-base-de-datos.md) y runner embebido.
- [x] Registro/login con **Argon2id** + token de sesión (`resume` al reconectar); lobby con crear (máx. 3), borrar y elegir personaje; stats iniciales por clase.
- [x] Mitigaciones: concurrencia de hashes limitada (2) y verificación dummy para usuarios inexistentes (anti timing/enumeration).
- [x] `welcome` con datos del personaje (el `game_data` estático completo llega en F5 con skills/ítems).
- [x] Guardado asíncrono: cada 60 s si hay cambios, al desconectarse y al apagar el servidor; carga de posición al entrar. El game loop nunca espera a la BD.
- [x] Errores de auth/lobby visibles en el cliente; pantalla de login/registro y lobby; cliente con **autotest headless**.

**Criterio de aceptación**: ✅ verificado end-to-end contra Postgres real (`TestPersistenceAcrossReconnect`) y con el cliente Godot headless (reingreso en `x=5.00`, la posición guardada). Falta tu prueba visual interactiva.

## F3 — Combate y progresión (3–5 sem) ❤ corazón técnico

- [x] Mobs cargados desde `shared/gamedata` (5 tipos, 18 spawns) con FSM idle/aggro/chase/attack/dead/respawn, leash y retaliación.
- [x] Auto-ataque con validación de rango/cooldown y auto-approach server-side; fórmulas del GDD en `internal/combat` **con tests unitarios**.
- [x] XP al matar con atenuación, level up (+5 puntos), asignación de stats (server + HUD) y guardado inmediato del progreso.
- [x] HP/maná, regeneración fuera de combate (5 s) y pociones simples: 5+5 por vida, curación 40%, cooldown 3 s, se reponen al morir (sin inventario aún).
- [x] Muerte del jugador: respawn en la ciudad, −3% del XP del nivel (sin perder nivel), los mobs vuelven a su spawn; toasts y teleport en el cliente.
- [x] Integración: test e2e contra Postgres (matar mob → nivel 2 → persistir progreso), autotest headless del cliente (mata una babosa) y tests de combate/mundo.

**Criterio de aceptación**: ✅ verificado en tests (combate, level up, muerte, persistencia) y cliente headless. Falta tu sesión interactiva de 15 min y el stress de 2+ jugadores (previsto en F7).

## F4 — Botín, inventario y mejora (4–6 sem) ❤ corazón emocional

- [x] Loot tables en JSON (26 ítems, 5 mobs) con oro; drops en el suelo con rareza y despawn (90 s), pickup validado server-side (≤3 m).
- [x] Inventario 8×5 con stacks, equipar/desequipar/usar/mover (drag & drop), instancias con rareza y mejora; persistencia transaccional del inventario completo.
- [x] Equipamiento con requisitos de clase/stats; daño, intervalo, rango y defensa derivados del equipo (starter kit por clase).
- [x] **Mejora +N** con tabla del GDD, consumo de núcleo + oro, fallo −1 nivel desde +7 y 5% de destrucción desde +10; resolución pura testeada.
- [x] Oro (drops automáticos al killer) + vendedor: pociones/núcleos, venta al 30% ajustado por rareza/mejora.
- [x] HUD/UI completa: mochila, equipo, tooltips con stats/requisitos, colores por rareza, tienda, ATK/DEF y oro.

**Criterio de aceptación**: ✅ verificado por tests (loot/pickup, equipo, mejora incluida destrucción, tienda, persistencia e2e con Postgres) y autotest headless (kit → equipar → mejorar → matar). Falta tu prueba visual y la sesión de 15 min con la tensión real de +6→+7.

## F5 — Mundo real: mapas, clases, skills (4–6 sem)

- [ ] 3 mapas con geometría simple pero temática + portales con requisito de nivel.
- [ ] Catálogo completo de mobs del GDD + 2 mini-bosses con mecánica simple.
- [ ] Las 3 clases jugables con sus stats y 3–4 skills cada una (mana, CD, anim).
- [ ] Primer pase de arte real: reemplazar cápsulas por modelos low-poly con anims idle/run/attack/death.
- [ ] Spawn/guardado por mapa; `teleport` message.

**Criterio de aceptación**: un amigo entra con Mago, vos con Guerrero; cada uno progresa distinto; hay razón para ir a la mazmorra.

## F6 — Social y "game feel" (3–4 sem)

- [ ] Chat (global/susurro/party) con rate-limit.
- [ ] Party de hasta 5 con XP compartida (fórmula del GDD).
- [ ] Interpolación fina + predicción del propio movimiento con corrección ([04-red §5](04-red.md)).
- [ ] Audio (hits, level up, portal) + números flotantes de daño + glow para ítems +7.
- [ ] Balance: primera pasada de números (matar lvl 15 en 60–90 s, 1h farm ≈ 12 intentos de mejora).
- [ ] Bot de stress: 20 bots, 10 min, sin crecimiento de RAM (leaks).

**Criterio de aceptación**: 3+ amigos en una party en la mazmorra, chateando, sin notar lag en LAN.

## F7 — Producción y playtest (1–2 sem)

- [ ] VPS + systemd (server), Caddy TLS (wss://), Postgres local al VPS + pg_dump diario.
- [ ] Export de cliente (Linux/Windows), README de instalación para amigos.
- [ ] Bots de stress contra el VPS real + una playtest organizada (2 h, 4–8 personas).
- [ ] Hotfix mínimo: log rotativo + reinicio del server como respuesta a bugs.
- [ ] Recopilar feedback en un archivo `docs/feedback-mvp.md`.

**Criterio de aceptación (cierre del MVP)**: partida de 1 h con 4+ jugadores en el VPS sin caída. Cumples los 3 criterios de [01-vision](01-vision.md).

---

## Riesgos y mitigaciones

| Riesgo | Prob. | Mitigación |
|---|---|---|
| **Scope creep** ("y si agrego guilds...") | Alta 🚨 | Los no-objetivos de la visión son ley. Nueva idea → `docs/backlog.md`, no al código. |
| Ahogo en el arte | Alta | Placeholders honestos hasta F5; packs CC0; nunca modelar desde cero en MVP |
| Bug de netcode desmotivante | Media | Cada fase deja demo jugable; [04-red §8](04-red.md) da herramientas de depuración; si algo se traba 3 días, simplificar y retomar |
| Burnout / vida part-time | Media | Semanas de 6–10 h sostenibles; hitos visibles; commits como diario de progreso |
| "Se cayó mi disco" | Baja catastrófica | Repo Git en GitHub + pg_dump del mundo de dev semanal |
| Balance roto (todo muy fácil/difícil) | Cierta | Los números del GDD son v1; F6 existe para eso; fórmulas puras → re-balancear es cambiar constantes |

## Cadencia semanal sugerida

- **L–V (3–4 sesiones de 1.5–2 h)**: codear la tarea de la semana (una sola historia: "drop al suelo").
- **1 sesión = "cerrar"**: commit, checklist manual de la fase en curso, anotar qué quedó turbio (`docs/notas.md`).
- **Fin de fase**: demo de 10 min (grabar GIF/clip para ver el progreso:combustible emocional).
- Si una fase se estira +50%: recortar SU alcance, no saltarla (una fase "fina" es mejor que una abandonada).
