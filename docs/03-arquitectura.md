# 03 — Arquitectura

## 1. Topología general

```
┌────────────────────┐         ┌──────────────────────────────────┐
│  CLIENTE  Godot 4  │         │  VPS (producción) / localhost    │
│                    │         │                                  │
│  - Render 3D       │  WS/TLS │  ┌────────────┐   ┌───────────┐  │
│  - Input (clic)    │◀───────▶│  │   Caddy    │   │ PostgreSQL │  │
│  - Interpolación   │  JSON   │  │ (reverse   │   │           │  │
│  - HUD/inventario  │         │  │  proxy TLS)│   └─────▲─────┘  │
│  - Audio/anim      │         │  └─────┬──────┘         │        │
└────────────────────┘         │        ▼                │        │
                               │  ┌──────────────────────┴─────┐  │
                               │  │   SERVIDOR Go (1 binario)  │  │
                               │  │                            │  │
                               │  │  [goroutines IO por conex] │  │
                               │  │        │ canales           │  │
                               │  │        ▼                    │  │
                               │  │  GAME LOOP (1 goroutine,   │  │
                               │  │  tick 20 Hz, autoritativo) │  │
                               │  │   - simulación de mundo    │  │
                               │  │   - combate/loot/mobs      │  │
                               │  │        │                   │  │
                               │  │  [pool de guardado → pgx]──┼──▶ BD
                               │  └────────────────────────────┘  │
                               └──────────────────────────────────┘
```

## 2. Principio rector: servidor autoritativo

| Responsabilidad | Cliente | Servidor |
|---|---|---|
| Render, animación, HUD, sonido, input | ✅ | ❌ |
| Predicción de TU propio movimiento | ✅ (suave, corregible) | ❌ |
| Posición "oficial", HP, daño, críticos, loot, XP, oro | ❌ (solo muestra) | ✅ |
| Lógica de skills/cooldowns/inventario | ❌ (solo pide acciones) | ✅ |

El cliente **pide acciones** (`moverme a X`, `atacar mob 42`, `mejorar ítem 7`) y **recibe estado**. Todo cálculo que afecte progreso ocurre en el servidor.

## 3. Modelo de procesos

**MVP = UN solo proceso Go** con:

1. **Goroutine de aceptación** (`net/http` con WebSocket upgrade): acepta conexiones.
2. **Una goroutine por conexión** para lectura (deserializa → envía comando por canal al mundo) y otra para escritura (batch de mensajes salientes).
3. **UNA goroutine de simulación (game loop)** que posee TODO el estado del mundo en memoria. Sin mutexes: el mundo es propiedad exclusiva de esa goroutine y el resto se comunica con canales. Esta decisión elimina de raíz las race conditions en la lógica del juego.
4. **Goroutines "trabajadoras"** solo para I/O de BD (guardar personajes cada 60 s y en logout) con un pool pequeño (2–4), reportando resultados al game loop por canal.

```go
// Pseudocódigo del game loop (tick fijo 50 ms = 20 Hz)
const tickRate = 50 * time.Millisecond
for {
    start := time.Now()

    // 1) Consumir TODO lo encolado por las conexiones (comandos de jugadores)
    for {
        select {
        case cmd := <-worldInbox:  world.Apply(cmd)   // login, move, attack, chat...
        default: break
        }
    }

    // 2) Simular el mundo
    world.Simulate(50ms) // mobs (IA, respawn), cooldowns, regen, drops que expiran...

    // 3) Snapshot a los clientes conectados (10 Hz → cada 2 ticks)
    if world.tick%2 == 0 { world.BroadcastSnapshots() }

    // 4) Dormir lo que sobra del tick
    sleep := tickRate - time.Since(start)
    if sleep > 0 { time.Sleep(sleep) }
    // (si sleep < 0 con frecuencia: el tick está sobrecargado → log de advertencia)
}
```

**Regla de escala**: a ≤64 jugadores y ≤300 entidades vivas, un solo hilo de simulación sobra (Go hace 20 ticks de un mundo así en <5 ms en un VPS chico). El camino de crecimiento se documenta en §8.

## 4. Divisiones internas del servidor

```
server/
  cmd/munext/main.go        → flags, arranque, shutdown
  internal/world/           → game loop, entidades, mapas, spawn
  internal/combat/          → fórmulas puras (¡testeable sin red!)
  internal/loot/            → tablas y rolls (ídem testeable)
  internal/netio/           → websocket, sesión por conexión, serialización
  internal/db/              → pgx, queries, migraciones
  internal/auth/            → hash, tokens
  internal/gamedata/        → carga de los JSON estáticos (ítems, mobs, skills, mapas)
```

**Detalle clave para aprender**: `combat` y `loot` son paquetes **puros** (entran números, salen números) → tests unitarios triviales. La complejidad de red está aislada en `netio`/`world`.

## 5. Datos del juego: estáticos vs dinámicos

| Tipo | Dónde vive | Ejemplos |
|---|---|---|
| **Estáticos** (contenido) | Archivos JSON en `shared/gamedata/`, versionados en Git | Plantillas de ítems, stats de mobs, loot tables, skills, XP curve, spawn points, geometría de colisión del mapa |
| **Dinámicos** (estado) | PostgreSQL | Cuentas, personajes, inventario, posición, mejoras |

El servidor carga los JSON al iniciar y los valida (si un mob referencia un loot inexistente → error de arranque, no crash en runtime).
El **cliente** recibe los mismos datos por un mensaje `game_data` al conectarse (o bundle exportado en build): una sola fuente de verdad para tooltips y stats visibles.

## 6. Flujo de ejemplo: "clickeo un mob"

```
1. Jugador clickea al mob            [CLIENTE]
   → si el mob está a rango: manda {t:"attack", target:412}   (si no: primero mueve, el cliente decide localmente)
2. netio recibe, valida sesión y formato, encola al mundo      [SERVIDOR: goroutine de conexión]
3. El game loop procesa el comando:
   - ¿target existe y está vivo? ¿está a rango (con tolerancia)? ¿cooldown listo?
   - calcula daño (combat.Calculate), crítico, aplica al mob → evento
4. En el próximo snapshot: HP nuevo del mob para todos los cercanos
   + evento puntual {t:"dmg", mob:412, amount:37, crit:true}  para efectos visuales/floaties
5. El mob (IA) pasa a "aggro" y responde
6. Al morir el mob: loot.LootRoll() → spawn de drops en el suelo, XP a la party
7. Cada 60 s (y en logout) el pool de BD guarda personajes afectados
```

## 7. Decisiones técnicas (mini-ADRs)

| # | Decisión | Alternativas | Razón / Disparador de revisión |
|---|---|---|---|
| ADR-1 | Godot 4 + GDScript | Unity, Unreal, Three.js | Gratis, ligero, WebSocket nativo, 3D suficiente para semi-isométrico. *Revisar si:* el proyecto exige asset store enorme o C# fuerte. |
| ADR-2 | Go para el servidor | Node, C#, Rust | Concurrency simple, deploy = 1 binario, stdlib alcanza. *Revisar si:* se necesita el ecosistema Unity-shared (C#). |
| ADR-3 | WebSocket + JSON en MVP | ENet/UDP, MessagePack | Depuración trivial (wscat), 64 jugadores = overhead irrelevante. *Revisar en F5+ si:* el ancho de banda de snapshots molesta (>100 KB/s por cliente) → migrar a binario (el protocolo de [04-red](04-red.md) está diseñado para ese cambio). |
| ADR-4 | Game loop single-goroutine | Hilo por zona, lock-free | Simple y suficiente a esta escala. *Revisar si:* CPU del tick > 60% sostenido o >150 jugadores. |
| ADR-5 | Postgres + pgx | MySQL, Mongo, SQLite | Relacional para datos relacionales; SQLite tentador para dev, pero Postgres local con Docker es igual de fácil y no migro después. |
| ADR-6 | Caddy como proxy TLS | nginx, certbot manual | Config de 5 líneas y HTTPS automático (Let's Encrypt). |
| ADR-7 | Cámara orbital semi-fija (estilo MU) con zoom | Libre 3ª persona | Identidad visual + pathfinding visual más legible + menos cámara/motion sickness. |

## 8. Camino de crecimiento (documentado, NO para el MVP)

| Problema futuro | Solución futura |
|---|---|
| >150 jugadores | Partir el mundo en "zonas" (cada zona una goroutine), interest management por grid |
| Ancho de banda | Snapshots binarios + solo campos cambiados (delta encoding) |
| Latencia mundial | UDP confiable (implementar o lib ENet-compatible) |
| Parche de contenido sin recompilar | Los JSON de gamedata ya separan contenido de código |
| Caídas de servidor | Supervisor (systemd) + Postgres como fuente de verdad + re-login |
