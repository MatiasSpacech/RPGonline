# 01 — Visión y alcance

## Idea en una frase

> Un RPG online 3D donde la sensación de progreso es adictiva: cada sesión termina con algo ganado — niveles, oro o un ítem que ahora es +7 — y la ciudad es el lugar donde se luce.

## Qué hace "estilo MU Online"

MU Online (2001) sigue siendo recordado por cosas muy concretas. La lista de abajo es la **esencia que replicamos**, no su contenido:

| Esencia de MU | Cómo aparece en MuNext |
|---|---|
| Click-to-move y combate con objetivo (tab/click) | Movimiento por clic con cámara orbital, ataque automático al objetivo |
| Grind satisfactorio con riesgo/recompensa | Mobs que respawnean rápido, XP visible por barra, drops raros brillando en el suelo |
| Mejora de ítems con % de éxito | +0 → +12 con probabilidades decrecientes; fallar cuesta materiales (y a niveles altos, el ítem) |
| Clases arquetípicas nítidas | Guerrero (melé/tanque), Mago (Área/burst), Arquera (rango/soporte híbrido) |
| Ciudades como hubs sociales | Zona segura con NPCs vendedores, jugadores con equipo visible, chat global |
| World PvP/PvE con zonas por nivel | 3 mapas MVP con rangos de nivel (ciudad, campo, mazmorra) |
| Stats simples con builds | STR/AGI/VIT/ENE + puntos por nivel que el jugador asigna |

## Pilares de diseño (en orden)

1. **Progreso siempre visible.** Cualquier sesión de 20 min deja algo: XP, oro, drops o un intento de mejora. Sin parches de "dailies" artificiales: el grind ES el contenido.
2. **El riesgo dulce.** La mejora de ítems es el corazón emocional: tensión al clickear "mejorar", euforia o pérdida. Nada de cash shop para evitar la frustración.
3. **Servidor manda.** Todo lo importante lo calcula el servidor (daño, loot, posición). El cliente es "vista + control". Esto protege el punto 1: el progreso de otros no puede falsificarse.
4. **Legible antes que realista.** Estilo visual claro y semi-estilizado (no realismo): mejor rendimiento, assets libres de calidad, y envejece mejor.
5. **Pequeño pero completo.** 64 jugadores, 3 mapas, 3 clases, un puñado de mobs — pero cada sistema terminado y funcional, no 30 sistemas a medias.

## Alcance MVP (primera versión "completa")

**"Completa" = se puede jugar una sesión real de 1 hora con amigos y querer repetir.**

- Login con usuario/contraseña; hasta 3 personajes por cuenta.
- 3 clases con 3–4 skills cada una.
- 3 mapas (ciudad segura + campo + mazmorra) conectados por portales.
- Combate PvE: auto-ataque con objetivo, skills con CD y maná, mobs con IA básica (agro, persecución, respawn), 1 mini-boss por mazmorra.
- Progresión: niveles 1–30 con curva creciente, puntos de stat, curva de daño/defensa.
- Ítems: ~40 plantillas (armas/armas/arqueros por clase, 5 piezas de armadura), drops con rareza, inventario de grilla, equipamiento visible, mejora +0→+12 con % y fallo.
- Economía mínima: oro, vendedor de pociones, vendedor de material de mejora.
- Social: chat global/susurro/party, party de hasta 5 con XP compartida.
- Muerte: respawn en ciudad con pérdida parcial de XP (sin drop de ítems en MVP).

## Post-MVP (backlog, sin compromiso)

- Guilds con emblema y ranking de mejora.
- PvP opt-in (arenas) y zonas de conflicto.
- Crafting y refinado de gemas.
- Mascotas y monturas.
- Ranking web de niveles y mejoras.
- Más mapas, evento mundial tipo "Blood Castle" (oleadas cronometradas).

## No-objetivos explícitos

- ❌ Miles de jugadores concurrentes / sharding.
- ❌ Motor propio, netcode UDP confiable custom, o física distribuida.
- ❌ Cash shop, pay-to-win ni "time gating".
- ❌ Estilo realista/PBR pesado, voxel o pixel-art.
- ❌ Sandbox con construcción (base building).
- ❌ Content pipeline procedural ("mapas infinitos"). Contenido hecho a mano, poco y bueno.

## Nota legal (importante)

MU Online es una marca y propiedad de Webzen. MuNext debe:

- **No** usar nombres de MU (Lorencia, Devias, Kundun, Dark Knight, "jewels of bless/soul", etc.), logos, modelos, texturas, sonidos ni interfaces copiadas.
- **No** usar el término "MU" en el nombre público del juego.
- **Sí** puede inspirarse en mecánicas y "game feel" (las mecánicas no tienen copyright).
- Assets solo de fuentes con licencia clara (CC0/CC-BY documentadas en [07-flujo-de-trabajo](07-flujo-de-trabajo.md)).

## Criterios de éxito del MVP

1. **Técnico**: 8 jugadores conectados simultáneamente (2 de ellos en la misma WiFi) jugando 1 hora sin caída de servidor ni desync visible de posición.
2. **Social**: en una playtest con amigos, alguien dice espontáneamente "esperá, me falta poco para el +9".
3. **Personal**: puedes explicar cada sistema del juego en 2 minutos porque lo construiste (control de alcance logrado).
