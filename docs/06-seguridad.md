# 06 — Seguridad y anti-cheat

> Regla mental: **el cliente corre en la máquina de una persona que puede modificar TODO lo que corre ahí.** Cualquier número que importe (HP, daño, posición, oro, drops) nace en el servidor o no cuenta.

## 1. Autenticación

- Registro/login con usuario + contraseña. **Argon2id** (`golang.org/x/crypto/argon2`) con parámetros estándar (64 MB, t=2, p=1) y salt aleatorio por usuario. Nunca MD5/SHA-1/contraseñas planas.
- Tras validar credenciales, el server emite un **token de sesión** (32 bytes aleatorios, `crypto/rand`) guardado en memoria (map token→user) y enviado al cliente. Reingreso con token = reconexión simple sin re-sendar password.
- Límites: rate-limit de intentos de login por IP y por cuenta (p. ej. 5/min → lock 10 min). Mensajes de error genéricos ("credenciales inválidas", sin revelar si existe el usuario).
- TLS: en producción Caddy termina `wss://` (Let's Encrypt automático). En dev localhost sin cifrar está bien; **nunca** producción sin TLS.
- Sin email/recaptcha en MVP (cuenta = usuario+contraseña); email de recuperación = post-MVP.

## 2. Límites de confianza (qué valida el servidor, siempre)

| El cliente dice... | El servidor verifica |
|---|---|
| "Me muevo a (x,z)" | Velocidad máx (dist/Δt), dentro del mapa, sin atravesar muros; si no, posición corregida |
| "Ataco al mob 412" | Que existe, está vivo, a rango real (con tolerancia 0.5 m), cooldown listo, línea de visión |
| "Uso skill 5" | Que el personaje CONOCE el skill, nivel, maná, cooldown, y el target es válido |
| "Equipo el ítem 321" | Que lo posee, es de su clase/stats y el slot es compatible |
| "Mejoro el ítem 77" | Que tiene materiales+oro, y el RNG lo hace el SERVIDOR |
| "Agarro drop 55" | Que está a ≤3 m del drop, el drop sigue vivo, y 1 solo pickup gana (server ordena) |
| "Tengo HP/XP/nivel X" | Nunca lo escucha: es estado del server, punto de partida y llegada |

## 3. Anti-cheat específico

| Amenaza | Mitigación MVP |
|---|---|
| Speedhack (moverse más rápido) | Validación de velocidad en cada waypoint (ver [04-red §5](04-red.md)); corrección de posición |
| Teleport | Destinos fuera del mapa o sin línea válida → snap-back. El skill de teleport del Mago valida destino server-side |
| Ataques automáticos a 100 Hz | Cooldown de ataque validado en server; rate limit de mensajes (≤30/s sostenido → descartar + log; ≥60 → kick) |
| Duplicación de ítems | Todo cambio de inventario en una tx; el server recalcula el inventario completo desde su estado en RAM, nunca "aplica diffs del cliente" |
| Fake drop/XP | Loot y XP los calcula el servidor; `mob_dead`/`drop_spawn` son solo notificaciones visuales |
| Botting simple | Fuera de alcance técnico del MVP; rate-limits suaves + detección de patrón son post-MVP. Documentado como riesgo aceptado |

## 4. Robustez de protocolo

- Deserialización con límites estrictos: máximo 1 mensaje por... (ver tabla) tamaño máx 32 KB, tipos conocidos, campos validados (rangos numéricos: `x` debe ser `float32` finito dentro de ±10.000).
- Mensaje inválido = **kick inmediato + log** (no intentar "recuperar" la sesión; un cliente honesto no manda basura).
- `t` desconocido → ignorar con warning (compatibilidad hacia adelante controlada), payload inválido → kick.
- El server **nunca envía** al cliente: hashes, tokens de otros, estructura interna de loot tables (solo resultados), IDs de otros usuarios (solo nombres de personajes).

## 5. Infraestructura

- El server corre como usuario sin privilegios, en `127.0.0.1` detrás de Caddy (solo Caddy expone puertos: 443 y 80→443).
- Postgres escucha **solo** en localhost (o socket unix); credenciales en `.env` fuera de Git.
- Secrets: `.env` + `gitignore` + ejemplo `example.env` versionado.
- Fail2ban o equivalente para SSH del VPS; llaves SSH, sin login con password root.
- Logs: eventos de seguridad en un archivo separado (`security.log`): login fallido, kick por protocolo, rate-limit.
- Dependencias Go: `govulncheck` en CI/llocal antes de deployar.

## 6. Checklist de revisión (por fase, 10 minutos)

Antes de cerrar cada fase del [roadmap](08-roadmap.md):

- [ ] ¿Algún valor nuevo que envía el cliente es "confiado" en el server? (buscar TODOs del estilo `client says`)
- [ ] ¿Toda query SQL usa parámetros (sin concatenar strings)?
- [ ] ¿Algún mensaje nuevo tiene validación de tamaño/tipo/rango?
- [ ] ¿El server tolera que 2 clientes manden comandos "simultáneos" sobre el mismo objeto? (el single-loop lo garantiza por diseño, pero verificar invariantes)
