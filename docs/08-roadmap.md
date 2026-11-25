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

- [ ] Protocolo mínimo: `login` (user fijo "dev"), `move_req`, `snapshot` ([04-red](04-red.md) §3–4).
- [ ] Game loop 20 Hz + snapshots 10 Hz por mapa único, con entidades PCs.
- [ ] Cliente: click-to-move sobre plano con navmesh simple, interpolación básica de otros jugadores (lerp).
- [ ] Volver a conectar tras caída del server (reconexión manual, sin estado).

**Criterio de aceptación**: dos ventanas de Godot, cada una ve a la cápsula ajena moverse con fluidez aceptable (sin "teleports" visibles a simple vista).

## F2 — Persistencia: cuentas y personajes (2–3 sem)

- [ ] Postgres en Docker Compose; migración `0001` con esquema de [05-base-de-datos](05-base-de-datos.md).
- [ ] Registro/login con Argon2id + token de sesión; lobby de personajes (crear hasta 3, elegir, borrar).
- [ ] `welcome` con datos del personaje + `game_data` estático inicial.
- [ ] Guardado: posición al logout y cada 60 s; carga al entrar.
- [ ] Manejo de errores visible en cliente (mensajes de login).

**Criterio de aceptación**: creo "Kaen" (guerrero), camino, cierro el cliente, re-entro y estoy donde estaba. Un segundo personaje puede existir.

## F3 — Combate y progresión (3–5 sem) ❤ corazón técnico

- [ ] Mobs en memoria con FSM (idle/aggro/chase/attack/dead/respawn), spawn points del archivo de mapa.
- [ ] Auto-ataque con validación de rango/cooldown; fórmulas del GDD en paquete `combat` **con tests unitarios**.
- [ ] XP al matar (atenuación por nivel), level up con stat points; barra de XP en HUD.
- [ ] HP/maná con regen fuera de combate; potiones simples (objeto fijo, sin inventario aún).
- [ ] Muerte del jugador: respawn + pérdida de XP del GDD.
- [ ] Integración: script cliente-fake (login→mover→matar 10 mobs) como test smoke.

**Criterio de aceptación**: sesión completa de 15 min matando lobos, subo 2–3 niveles, muero una vez, todo persiste. Sin crashes en 30 min con 2 jugadores.

## F4 — Botín, inventario y mejora (4–6 sem) ❤ corazón emocional

- [ ] Loot tables en JSON; drops al suelo con visual, pickup validado server-side.
- [ ] Inventario de grilla (equip/unequip/use/move), instancias de ítems con rareza.
- [ ] Equipamiento con requisitos de stat/clase; stats derivados recalculados server-side.
- [ ] **Mejora +N** con probabilidades y fallo ([GDD §5.3](02-diseno-de-juego.md)); transacción atómica.
- [ ] Oro + vendedor básico (compra pociones/núcleos, vende ítems).
- [ ] HUD completo: inventario, tooltips, stats del personaje.

**Criterio de aceptación**: matás, juntás, equipás, y una mejora +6→+7 te hace sostener la respiración (y a veces gritar). El oro persiste.

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
