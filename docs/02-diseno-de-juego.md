# 02 — Diseño de juego (GDD resumido)

> Este documento define QUÉ es el juego y con qué números. Cómo se implementa está en [03-arquitectura](03-arquitectura.md) y [04-red](04-red.md).
> Todos los valores son "primera propuesta": se balancean en F6, pero hay que arrancar desde algún número.

## 1. Core loop

```
  Entrar a la ciudad (zona segura)
        │  comprar pociones / material de mejora / vendé drops
        ▼
  Salir a cazar (campo o mazmorra según nivel)
        │  click en mob → auto-ataque → skills → matás mobs
        ▼
  Drop de ítem/oro ──┐
  XP → nivel ────────┤  (cada kill: barra de XP sube, a veces brilla algo en el piso)
        │           │
        ▼           ▼
  Volver a la ciudad: equiparte lo nuevo
        │
        ▼
  PROBAR LA MEJORA: +6 → +7?  ♥   tensión → euforia o pérdida
        │
        └──> repetir, "una sesión más"
```

El **loop emocional real** es: *cazar → botín → mejora con riesgo → presumir*. Cada fase del roadmap debe respetarlo.

## 2. Clases

| | **Guerrero** | **Mago** | **Arquera** |
|---|---|---|---|
| Rol | Melé / tanque | Burst mágico / AoE | Rango / híbrido soporte |
| Arma | Espadas, hachas | Bastones | Arcos |
| Rango de ataque | 1.8 m | 8 m | 9 m |
| HP base (lvl 1) | 80 | 50 | 60 |
| Maná base | 20 | 60 | 35 |
| Stats iniciales (S/A/V/E) | 28/20/25/10 | 18/18/15/30 | 22/25/20/22 |
| Dificultad | ★☆☆ (recomendada para empezar) | ★★☆ | ★★☆ |

- Stats: **STR** (fuerza, daño melé), **AGI** (agilidad, velocidad de ataque y evasión), **VIT** (vitalidad, HP), **ENE** (energía, maná y daño mágico).
- +5 puntos de stat por nivel, asignables a mano (sin respec en MVP; respec es post-MVP).
- Requisitos de equipo por stat: un arma nivel 15 de Guerrero puede pedir STR 60 — el "build" nace de qué querés poder equipar.

### Fórmulas derivadas (v1, todas server-side)

```
HP máx  = 20 + VIT * 2 + nivel * 3
Maná    = 10 + ENE * 1.5 + nivel * 1

Daño melé (min/max)  = arma.min..arma.max + STR * 0.6
Daño mágico           = arma.magic + ENE * 0.8
Daño rango            = arma.min..arma.max + AGI * 0.5

Vel. ataque           = arma.base_ms / (1 + AGI * 0.005)      (ms entre golpes)
Defensa               = Σ armaduras + VIT * 0.3
Evasión               = 5 + AGI * 0.1 (%)                      (cap 30%)
Daño recibido         = max(1, daño_atacante * (100/(100 + defensa))) * (1 - evasión_si_aplica)
Crítico               = 5% base, x1.5 daño
```

Regla de oro: **el cliente solo muestra el resultado**, nunca calcula nada de esto (ver [06-seguridad](06-seguridad.md)).

## 3. Progresión de niveles

- Nivel máx MVP: **30**. XP para pasar del nivel `n` al `n+1`: `xp(n) = round(60 * n^1.7)`.
- **Implementación F3**: la BD guarda el XP como progreso *dentro* del nivel (no acumulado). La tabla de abajo es una referencia acumulada aproximada.
- Kill de un mob otorga su XP *atenuada* si el jugador lo supera por ≥5 niveles (`factor = 0.3^((nivel_jugador - nivel_mob - 5) / 5)`, piso 0.1).
- Muerte: **pierde el 3% del XP de tu nivel actual** (nunca baja de nivel, nunca pierde ítems en MVP).

| Nivel | XP acumulada aprox. | Evento |
|---|---|---|
| 1→2 | 60 | Primeros puntitos |
| 5 | ~2.000 | Skill 2, mapas de campo completos |
| 10 | ~11.000 | Mini-boss de campo disponible |
| 15 | ~28.000 | Acceso a mazmorra |
| 20 | ~53.000 | Set de armadura "veterano" |
| 25 | ~88.000 | Skills finales |
| 30 | ~132.000 | Final del MVP: mini-boss de mazmorra |

## 4. Combate

- **Auto-ataque con objetivo**: clickeás (o Tab) al mob; el personaje ataca en bucle mientras esté en rango. El servidor valida rango y cooldown de cada golpe.
- **Skills**: se desbloquean por nivel de clase, cuestan maná, tienen cooldown por skill (no global, MVP simple), y animación con "casteo" corto en el Mago.
- Targeting: solo un objetivo activo (MU-style). AoE del Mago se centra en el objetivo.
- Line of sight: en MVP, mobs y jugadores pueden atacar a través de obstáculos bajos pero NO a través de paredes (check simple de raycast en el server).

### Skills MVP (3–4 por clase)

| Clase | Skill | Nivel | Efecto v1 |
|---|---|---|---|
| Guerrero | Tajo Poderoso | 3 | x2.2 daño melé, CD 6 s |
| Guerrero | Grito de Guerra | 10 | +15% daño propio/party 30 s, CD 60 s |
| Guerrero | Remolino | 18 | Golpea hasta 5 enemigos a 2.5 m, x1.5, CD 10 s |
| Mago | Bola de Fuego | 1 | x1.8 mágico, CD 3 s |
| Mago | Nova AoE | 8 | x1.2 mágico a todos en 4 m del objetivo, CD 12 s |
| Mago | Teletransporte | 14 | Salta 6 m hacia el cursor (mismo mapa, server valida destino), CD 20 s |
| Arquera | Flecha Triple | 3 | 3 flechas x0.7 cada una, CD 8 s |
| Arquera | Flecha Curativa | 12 | Cura al objetivo aliado 25% del daño de arco, CD 15 s |
| Arquera | Multidisparo | 20 | Golpea a 4 enemigos del target a 9 m, x0.8, CD 12 s |

## 5. Ítems

### 5.1 Estructura

- **Plantilla** (estático, en archivos JSON versionados): id, nombre, tipo, requisitos de stat, stats base, tier, valor de venta.
- **Instancia** (dinámico, en BD): plantilla + nivel de mejora + dueño + slot.
- Equipamiento: arma, casco, pecho, guantes, botas, escudo/anillo (6 slots). Visual: arma + 4 piezas de armadura tintadas por tier.

### 5.2 Rareza

| Rareza | Color UI | Mult. stats base | Drop weight (campo) |
|---|---|---|---|
| Común | blanco | x1.0 | 60% |
| Bueno | verde | x1.15 | 25% |
| Raro | azul | x1.3 | 10% |
| Épico | violeta | x1.5 | 4% |
| Legendario | naranja | x1.8 | 1% |

### 5.3 Mejora "+N" (el corazón emocional)

- Niveles **+0 → +12**. Cada intento consume 1 **Núcleo de Mejora** (drop de mobs / vendedor caro) + oro según nivel.
- Mejora aplica `+ (N * 8%)` a los stats base del ítem (redondeado).
- Tensión: **si falla en +7 o más, el ítem pierde 1 nivel de mejora** (y en +10 o más, 5% de destrucción — decisión final en balance).

| Intento | Éxito | Costo aprox. | Nota |
|---|---|---|---|
| +0→+1 … +5→+6 | 95% → 70% | bajo | Zona "cómoda" |
| +6→+7 | 55% | medio | Primer muro emocional |
| +7→+8 … +9→+10 | 45% → 30% | alto | Fallo = -1 nivel |
| +10→+11 … +11→+12 | 20% → 12% | muy alto | +5% destrucción |

- Un ítem +7 o más **brilla** en el personaje (indicador de estatus, clave del pilar 2).

### 5.4 Inventario

- Grilla de **8x5 = 40 slots** (MU-style, stack de consumibles x20, equipo 1 slot c/u).
- Oro aparte (no ocupa slots, no dropea al morir en MVP).

## 6. Mobs

IA MVP = máquina de estados simple: `idle → (jugador en radio de agro) → chase → (rango) → attack → (muere) → dead → (timer) → respawn`.

| Mob (nombres provisionales) | Mapa | Nivel | HP | Daño | XP | Respawn | Comportamiento |
|---|---|---|---|---|---|---|---|
| Babosa del camino | Campo | 1 | 30 | 4 | 12 | 20 s | Pasivo (no agrede) |
| Lobo flaco | Campo | 3 | 55 | 7 | 25 | 25 s | Agresivo, radio 8 m |
| Bandido | Campo | 6 | 110 | 12 | 55 | 30 s | Agresivo, patrulla |
| Escorpión rey | Campo | 9 | 160 | 16 | 90 | 40 s | Agresivo, veneno leve |
| **Golem del puente** | Campo (mini-boss) | 10 | 900 | 28 | 700 | 5 min | Inmune a stun, lento |
| Esqueleto oxidado | Mazmorra | 13 | 210 | 22 | 130 | 35 s | Agresivo |
| Cultista | Mazmorra | 16 | 260 | 26 | 180 | 35 s | Caster a 8 m |
| Caballero caído | Mazmorra | 19 | 380 | 34 | 260 | 45 s | Agresivo, rápido |
| **Carcelero** | Mazmorra (boss) | 23 | 2.500 | 60 | 1.800 | 10 min | 2 fases: melé → invoca 2 esqueletos al 50% |

- **Loot**: cada mob tiene loot table (tabla de plantillas con peso). Oro siempre (rango). Drop cae **al suelo** con visual brillante y cualquier jugador puede agarrarlo (MVP: sin timer de "solo tú"; timer de limpieza 90 s).
- Spawn points definidos en el archivo del mapa (no procedural).

## 7. Mapas MVP

| Mapa | Tipo | Niveles | Descripción |
|---|---|---|---|
| **Refugio de Piedra** | Ciudad, 100% segura | 1–30 | Hub: vendedores, banco visual, portales. Sin combate. |
| **Llanuras Quebradas** | Campo abierto | 1–12 | Primer mapa de caza, estilo pradera con ruinas. Portal desde la ciudad. Mini-boss al fondo. |
| **Catacumbas de Óxido** | Mazmorra | 13–25 | Corredores y salas, más oscura, densidad de mobs mayor, boss final. |

- Portal = trigger cilíndrico + confirmación de nivel mínimo.
- Respawn al morir: siempre ciudad (MVP, sin puntos de guardado).
- Tamaño MVP: cada mapa ~120x120 m.

## 8. Social

- **Chat**: canales `Global` (naranja), `Susurro` (rosa, `/w nombre`), `Party` (celeste). Sin canales de guild en MVP.
- **Party**: hasta 5 jugadores. Invitación por clic en el personaje. XP se reparte: `xp_por_jugador = xp_mob * (0.6 + 0.4/n_miembros)` si el nivel de cada miembro está a ≤8 del mob (si no, ese miembro recibe la XP atenuada). Sin split de oro/drops: loot "primero que agarra".
- Sin trade entre jugadores en MVP (post-MVP; los vendedores cubren la economía temprana).

## 9. Muerte y respawn

- HP llega a 0 → muerte: animación, pantalla con botón "Revivir en ciudad", -3% XP del nivel actual, se revive en el punto de respawn de la ciudad con HP/MP al 50%.

## 10. Economía v1

| Fuente de oro | Sink de oro |
|---|---|
| Drops de mobs (60% de kills dan 1–20 oro según nivel) | Pociones (maná/HP) |
| Vender ítems a vendedor (30% de su valor) | Núcleos de Mejora en vendedor (caro: incentiva farmearlos) |
| Quest recompensas (post-MVP, no en MVP) | Costo de oro por intento de mejora +N |
| — | Muerte: pequeño costo de "sangría" (post-MVP) |

Meta de balance F6: una hora de farm nivel 15 ≈ 12 intentos de mejora comprando la mitad de los núcleos.
