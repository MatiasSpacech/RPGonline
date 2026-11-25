# 09 — Glosario

Términos que aparecen en la documentación y conviene dominar (EN → explicación ES).

## Red y servidor

| Término | Significado |
|---|---|
| **Servidor autoritativo** | El servidor es la única fuente de verdad del estado del juego (posición, HP, loot). El cliente solo pide acciones y muestra resultados. |
| **Tick / tick rate** | Un "paso" de la simulación del servidor. Tick rate 20 Hz = el mundo avanza 20 veces por segundo. |
| **Snapshot** | Fotografía del estado de las entidades en un tick, enviada a los clientes. |
| **Interpolación** | Técnica del cliente para suavizar entre dos snapshots: en vez de "saltar" de posición, anima el movimiento intermedio. |
| **Predicción (client-side)** | El cliente mueve a TU personaje de inmediato al input y el servidor corrige si estaba mal. MVP: versión suave (el cliente lidera el movimiento, el server valida). |
| **Latencia / ping** | Tiempo de ida y vuelta de un mensaje. El heartbeat lo mide. |
| **Interest management** | Enviar a cada jugador solo las entidades que le importan (cercanas). MVP: broadcast a todo el mapa; post-MVP: por área. |
| **WebSocket (WS/WSS)** | Canal bidireccional persistente sobre HTTP. `wss://` = cifrado con TLS. |
| **ENet / UDP** | Protocolo de transporte UDP usado en juegos por latencia baja. Alternativa futura a WebSocket (ADR-3). |
| **Goroutine / canal** | En Go: "hilo liviano" y tubería segura para pasar datos entre goroutines. Nuestro game loop es UNA goroutine que posee el mundo. |
| **Race condition** | Bug cuando dos hilos tocan los mismos datos a la vez. Se evita por diseño: solo el game loop toca el estado del mundo. |
| **FSM (máquina de estados finita)** | IA simple: el mob está en un estado (idle/chase/attack/dead) y pasa a otro según reglas. |
| **Navmesh** | Malla de "por dónde se puede caminar" generada sobre la geometría; el pathfinding la usa para calcular rutas. |
| **Broadcast** | Enviar un mensaje a todos los clientes de un mapa/conexión. |

## Persistencia

| Término | Significado |
|---|---|
| **Migración** | Script SQL versionado que modifica el esquema de la BD (golang-migrate lo ejecuta en orden). |
| **Transacción (tx)** | Grupo de operaciones SQL que se aplican todas o ninguna (p. ej. mejorar un ítem sin perder materiales en el intento). |
| **Pool de conexiones** | Conjunto de conexiones reutilizables a Postgres (`pgxpool`). |
| **Argon2id** | Algoritmo moderno para hashear contraseñas (lento a propósito). Nunca MD5/SHA-1. |
| **Token de sesión** | String aleatorio que identifica una sesión ya autenticada (evita mandar la contraseña a cada rato). |
| **Dirty flag** | Marca "este personaje cambió" para saber a quién guardar en el ciclo de persistencia. |

## Diseño de juego

| Término | Significado |
|---|---|
| **MVP** | Producto mínimo viable: la versión más chica que ya "es el juego" (aquí: [01-vision §Alcance](01-vision.md)). |
| **Core loop** | Ciclo de juego que se repite y que debe ser placentero: cazar → botín → mejorar → presumir. |
| **Grind** | Progresión por repetición deliberada (matar muchos mobs). En MuNext es contenido, no castigo. |
| **Loot table** | Tabla ponderada que define qué puede dropear un mob y con qué probabilidad. |
| **Drop / respawn** | Ítem que aparece al matar / reaparición del mob tras un tiempo. |
| **AoE** | *Area of Effect*: habilidad que afecta a varios enemigos en un radio. |
| **CD (cooldown)** | Tiempo de espera antes de reusar una skill. |
| **DPS / burst** | Daño por segundo sostenido / daño explosivo en poco tiempo. |
| **Sink / source (economía)** | Fuentes que crean oro (drops) y pozos que lo destruyen (mejoras, vendedores) — el balance entre ambos controla la inflación. |
| **Vertical slice** | Rebanada delgada pero completa del juego (un mapa, un mob type, todo el loop) — filosofía de este roadmap. |
| **Placeholder** | Asset/valor temporal "honesto" (cápsula con nombre) que permite avanzar sin arte final. |

## Herramientas

| Término | Significado |
|---|---|
| **Docker Compose** | Definir servicios (Postgres, server) con un archivo y levantarlos con un comando. |
| **VPS** | Servidor virtual alquilado (~US$5–15/mes) donde corre producción. |
| **Reverse proxy (Caddy)** | Servidor que recibe el tráfico público (con TLS) y lo reenvía al juego por localhost. |
| **Godot export preset** | Configuración para generar builds ejecutables del cliente (Linux/Windows). |
| **CI** | Integración continua: correr tests automáticamente al hacer push (opcional aquí, `go test` local mínimo). |
