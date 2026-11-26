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

> **Implementado hasta F4**: auth/lobby, movimiento, combate, stats, botín/inventario/equipo/tienda y mejora +N. El resto del catálogo es el objetivo del MVP.

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
| `attack` | { target } | fija el mob del auto-ataque (0 = detener) |
| `alloc_stat` | { stat } | "str" / "agi" / "vit" / "ene"; gasta 1 punto |
| `use_item` | { item_id } | usa una poción del inventario |
| `potion` | { kind } | atajo de HUD: usa la primera poción hp/mp (cooldown 3 s) |
| `pickup` | { id } | recoge un drop del suelo (≤3 m) |
| `equip` | { item_id } | equipa (valida clase/requisitos) |
| `unequip` | { slot } | weapon/helm/chest/gloves/boots/shield |
| `inv_move` | { item_id, x, y } | mueve/swap en la grilla 8×5 |
| `enhance` | { item_id } | intento de mejora +N (consume núcleo + oro) |
| `shop_buy` | { tmpl, qty } | compra consumibles |
| `shop_sell` | { item_id } | vende un ítem del inventario |
| `skill` | { skillId, target } | target opcional para heals/self (F5) |
| `chat` | { ch, msg, to? } | ch ∈ global, party, whisper (F6) |
| `party_inv` / `party_accept` | { charId } | (F6) |
| `portal` | { portalId } | cambio de mapa (F5) |
| `ping` | {} | cada 2 s; el server responde `pong` (timeout 20 s) |

### S → C (estado: el servidor *informa*)

| t | d | Notas |
|---|---|---|
| `login_ok` | { token, user, chars } | token para `resume` + lista de personajes |
| `char_created` / `char_deleted` | { char } / { char_id } | actualizan el lobby |
| `welcome` | { self, character, map, tick_rate, snapshot_rate, game_data } | `game_data.items` trae las plantillas de ítems |
| `snapshot` | { tick, ents } | ver §4 (incluye drops) |
| `pong` | { server_time_ms } | respuesta al ping (RTT) |
| `self_state` | { hp, hp_max, mp, mp_max, xp, xp_next, level, str, agi, vit, ene, stat_points, hp_potions, mp_potions, attack_min, attack_max, defense, attack_interval_ms } | HUD: al entrar y cuando cambia algo |
| `dmg` | { src, dst, amount, crit, kind } | números flotantes; kind ∈ out/in/dodge |
| `mob_dead` | { id, xp, gold, killer } | feedback de kill + XP/oro ganado |
| `death` | { xp_lost } | moriste; después llega `teleport` |
| `inventory` | { gold, items } | estado completo del inventario (incluye equipados) |
| `enhance_result` | { item_id, success, new_enhance, destroyed } | momento de tensión |
| `chat` | { ch, from, msg } | (F6) |
| `party_state` | { members } | (F6) |
| `levelup` | { level, stat_points } | visual fanfarria |
| `teleport` | { map, x, y, z } | respawn |
| `err` | { code, msg } | errores de auth/lobby/mundo; logueado server-side |

## 4. Sincronización de entidades

- Servidor simula a **20 Hz** (tick 50 ms) y envía **snapshots a 10 Hz** (cada 2 ticks).
- Snapshot incluye entidades "activas" del mapa del jugador: jugadores, mobs vivos, drops en el suelo. En MVP se envían todos (broadcast por mapa); a 64 jugadores esto es aceptable (decenas de KB/s). Interés por área = post-MVP.

```json
{ "t": "snapshot", "d": {
  "tick": 104,
  "ents": [
    { "id": 1000001, "k": "mob", "tmpl": "lobo", "name": "Lobo flaco", "level": 3,
      "x": 12.3, "y": 0, "z": -45.1, "r": 1.2, "hp": 35, "hp_max": 55, "anim": "chase" },
    { "id": 7, "k": "pc", "name": "Kaen",
      "x": 15.0, "y": 0, "z": -44.0, "r": 2.3, "anim": "attack" },
    { "id": 901234, "k": "drop", "tmpl": "sword_iron", "name": "Espada de hierro",
      "rarity": "rare", "x": 13.1, "y": 0, "z": -44.2 }
  ]
}}
```

- `id`: entero asignado por el servidor. `k`: pc/mob/drop. `r`: rotación Y. `anim`: idle/run/chase/attack. `hp`/`hp_max` solo para mobs; `rarity` solo para drops.
- Los drops viven en el snapshots (no hay mensajes spawn/gone separados) y expiran a los 90 s.
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
