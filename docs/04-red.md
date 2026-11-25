# 04 — Red y sincronización

## 1. Transporte

**MVP: WebSocket con JSON.** Justificación en ADR-3 de [03-arquitectura](03-arquitectura.md).

- Godot 4: `WebSocketPeer` (nativo, sin plugins).
- Go: `nhooyr.io/websocket` o `gorilla/websocket` (gorilla está en mantenimiento comunitario y es estable).
- En producción va detrás de Caddy con TLS (`wss://`). En desarrollo local, `ws://localhost` sin cifrar.

**Regla de migración** (no hacerla antes de F6): el diseño de mensajes es "sobre de tipo + payload", así que cambiar JSON → MessagePack o binario packed solo toca la capa de serialización, no la lógica.

## 2. Formato de mensaje

Sobre JSON mínimo:

```json
// Cliente → Servidor
{ "t": "attack", "d": { "target": 412 } }

// Servidor → Cliente
{ "t": "snapshot", "d": { "tick": 104, "ents": [ ... ] } }
```

- `t` = tipo (string corta, tabla abajo). `d` = payload. Sin más campos en MVP.
- Cada mensaje = 1 frame de WebSocket (límite de payload: 32 KB, se rechaza lo que exceda).
- **Cliente nunca manda mensajes que no pidió el estado** (no hay "sync de cliente" iniciado por cliente salvo login).
- Reloj: el cliente NO necesita tiempo del servidor en MVP (no hay eventos programados visibles); el "tick" viaja en snapshots solo para debugging.

## 3. Catálogo de mensajes MVP

> **Implementado hasta F2**: `register`, `login`, `resume`, `create_char`, `delete_char`, `select_char`, `move_req`, `ping` y sus respuestas. El resto del catálogo es el objetivo del MVP.

### C → S (acciones: el cliente *pide*)

| t | d | Notas |
|---|---|---|
| `register` | { user, pass } | crea cuenta y autentica (responde `login_ok`) |
| `login` | { user, pass } | responde `login_ok` o `err` |
| `resume` | { token } | reanuda sesión tras reconectar (token de `login_ok`) |
| `create_char` | { name, class } | nombre 3–16 letras; máximo 3 por cuenta |
| `delete_char` | { char_id } | borra un personaje propio |
| `select_char` | { char_id } | entra al mundo → `welcome` |
| `move_req` | { x, y, z } | punto destino del clic (ver §5) |
| `attack` | { target } | inicia/detiene auto-ataque (0 = detener) |
| `skill` | { skillId, target } | target opcional para heals/self |
| `pickup` | { dropId } | recoger drop del suelo |
| `equip` / `unequip` | { itemId, slot? } | |
| `inv_move` | { itemId, slotX, slotY } | reordenar grilla |
| `use_item` | { itemId } | pociones |
| `enhance` | { itemId } | intento de mejora +N (consume materiales del inventario) |
| `chat` | { ch, msg, to? } | ch ∈ global, party, whisper |
| `party_inv` / `party_accept` | { charId } | |
| `portal` | { portalId } | cambio de mapa |
| `ping` | {} | cada 2 s; el server responde `pong` (timeout 20 s) |

### S → C (estado: el servidor *informa*)

| t | d | Notas |
|---|---|---|
| `login_ok` | { token, user, chars } | token para `resume` + lista de personajes |
| `char_created` / `char_deleted` | { char } / { char_id } | actualizan el lobby |
| `welcome` | { self, character, map, tick_rate, snapshot_rate } | datos para entrar al mundo |
| `snapshot` | { tick, ents } | ver §4 |
| `pong` | { server_time_ms } | respuesta al ping (RTT) |
| `self_state` | { hp, mp, xp, level, gold, stats... } | cambia poco → evento barato |
| `dmg` | { src, dst, amount, crit, kind } | para números flotantes/anim (visual only) |
| `mob_dead` | { id, xp, drops } | feedback de kill |
| `drop_spawn` / `drop_gone` / `drop_pickup` | { id, pos, tmpl, rarity } / { id } / { id, by } | |
| `inv` | { items, gold } | inventario completo (simple) al cambiar algo |
| `enhance_result` | { itemId, level, ok, destroyed? } | momento de tensión |
| `chat` | { ch, from, msg } | |
| `party_state` | { members } | al unirse/salirse |
| `levelup` | { charId, level } | visual fanfarria |
| `teleport` | { map, x, y, z, ents } | al cruzar portal o morir |
| `err` | { code, msg } | errores de auth/lobby/mundo; logueado server-side |

## 4. Sincronización de entidades

- Servidor simula a **20 Hz** (tick 50 ms) y envía **snapshots a 10 Hz** (cada 2 ticks).
- Snapshot incluye entidades "activas" del mapa del jugador: jugadores, mobs vivos, drops en el suelo. En MVP se envían todos (broadcast por mapa); a 64 jugadores esto es aceptable (decenas de KB/s). Interés por área = post-MVP.

```json
{ "t": "snapshot", "d": {
  "tick": 104,
  "ents": [
    { "id": 412, "k": "mob",  "tmpl": "skeleton", "x": 12.3, "y": 0, "z": -45.1, "r": 1.2, "hp%": 64, "st": "attack", "tgt": 7 },
    { "id": 7,   "k": "pc",   "x": 15.0, "y": 0, "z": -44.0, "r": 2.3, "hp%": 100, "name": "Kaen", "anim": "run" }
  ]
}}
```

- `id`: entero asignado por el servidor. `k`: kind. `r`: rotación Y. `st`: estado para animación (idle/run/attack/dead). Solo `hp%` (no HP exacto de otros: visual).
- **Interpolación del cliente**: mantener un buffer de ~100 ms de snapshots; renderizar entidades interpoladas entre las dos últimas posiciones conocidas. Con 10 Hz de snapshots la respuesta visual es fluida aunque el estado llegue "a saltos".
- **Eventos puntuales** (`dmg`, `drop_spawn`, `enhance_result`...) viajan como mensajes separados; son "efímeros": si se pierdan uno, la snapshot siguiente corrige el estado real.

## 5. Movimiento (la parte delicada)

### Modelo MVP: "cliente propone, servidor valida y corrige"

1. El jugador clickea → el cliente hace pathfinding local (navmesh de Godot) → mueve el personaje localmente (respuesta inmediata, se siente bien) → manda `move_req` con el **destino** y luego actualizaciones del **waypoint actual** cada ~200 ms.
2. El servidor:
   - valida velocidad: distancia recorrida desde el último waypoint ≤ `velocidad * Δt * 1.3` (tolerancia por red);
   - valida que el punto no atraviese geometría (colisión del mapa simplificada: círculos + muros 2D "top-down" proyectados);
   - si es ilegítimo: **no actualiza** su posición oficial y en la próxima snapshot el jugador verá a su personaje "telentransportado de vuelta" a la posición válida (corrección).
3. La posición oficial del jugador para TODOS (y para el combate: rangos, line of sight) es la del servidor.

Este modelo evita el problema difícil (navmesh server-side en Go) sin sacrificar autoridad: **la posición del servidor siempre se puede validar con una inecuación barata** (velocidad y colisión de círculos).

### Riesgo aceptado en MVP

El cliente "lidera" el movimiento → un tramposo puede *verse* moverse raro en su pantalla, pero no puede atravesar el rango de un golpe sin que el servidor lo valide, ni moverse a una velocidad que el servidor no acepta (lo devuelve). Suficiente para MVP; el paso siguiente (F6+) es pathfinding server-side.

## 6. Conexión: reconexión y estados

```
[connect] → [auth: login/login_err] → [lobby: lista de chars] → [in-world]
                                                              │
        timeout heartbeat 15 s ◀──────────────────────────────┤ cualquier estado: expulsar
        error de protocolo (t desconocido, payload inválido) → close + log
```

- **Reconexión con mismo token** (F6, no imprescindible): si el socket cae y vuelve en <30 s, el token de sesión reingresa al personaje sin pasar por login.
- **Logout limpio**: el server guarda al personaje, remueve al jugador de la party, broadcast de `pc_gone` (implícito: desaparece del snapshot).
- **Server shutdown**: cierra listeners, guarda TODOS los personajes, cierra sockets (drain 5 s).

## 7. Presupuesto de ancho de banda (back-of-napkin)

- Snapshot de 30 entidades × ~80 bytes JSON ≈ 2.4 KB × 10 Hz ≈ **24 KB/s** por cliente. ×64 clientes ≈ 1.5 MB/s total: trivial para un VPS.
- Si en F5 el "campo" tiene 80 mobs vivos por mapa → 6 KB × 10 Hz ≈ 60 KB/s/cliente: todavía OK, pero es la señal para (a) recortar mobs dormidos fuera de radio de 60 m, (b) binario. La arquitectura lo permite sin tocar lógica.

## 8. Depuración (imprescindible para aprender)

- `wscat` o un script Go pequeño: conectarse "a mano", mandar `login`, ver snapshots → sirve para probar el server sin cliente.
- Log de todos los mensajes con `t` y tamaño en modo verbose (`-v`), nunca en producción (spam).
- El cliente muestra `ping` (tiempo del heartbeat) en overlay debug (F1).
