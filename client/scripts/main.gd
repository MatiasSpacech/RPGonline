extends Node3D
## F3 — combate: auth/lobby (F2) + mobs, auto-ataque, XP, pociones y muerte.
## El servidor es la autoridad: acá se pide y se renderiza.

const INTERP_DELAY_MS := 100
const MAP_HALF := 19.0
const CAMERA_OFFSET := Vector3(0, 8, 10)
const NetClientScript := preload("res://scripts/net_client.gd")
const AuthUIScript := preload("res://scripts/auth_ui.gd")
const GameHUDScript := preload("res://scripts/hud.gd")
const PlayerEntityScript := preload("res://scripts/player_entity.gd")
const MobEntityScript := preload("res://scripts/mob_entity.gd")
const PLAYER_SCENE := preload("res://scenes/player_entity.tscn")
const MOB_SCENE := preload("res://scenes/mob_entity.tscn")

const AUTOTEST_USER := "autotest"
const AUTOTEST_PASS := "secreto123"
const AUTOTEST_CHAR := "AutoWarrior"

@onready var _net: NetClientScript = $NetClient
@onready var _ui: AuthUIScript = $UIOverlay
@onready var _hud: GameHUDScript = $UI
@onready var _camera: Camera3D = $Camera3D
@onready var _entities_root: Node3D = $Entities
@onready var _marker: MeshInstance3D = $MoveMarker

var _entities: Dictionary = {}
var _mobs: Dictionary = {}
var _self_id := 0
var _self_entity: PlayerEntityScript = null
var _character: Dictionary = {}
var _current_tick := 0
var _last_rtt := 0
var _in_game := false
var _marker_tween: Tween

# Modo autotest headless: `godot --headless --path client -- --autotest`
var _autotest := false
var _autotest_state := 0
var _autotest_attack_sent := false
var _autotest_done := false


func _ready() -> void:
	for arg in OS.get_cmdline_user_args():
		if arg == "--autotest":
			_autotest = true

	_net.connected.connect(_on_net_connected)
	_net.disconnected.connect(_on_net_disconnected)
	_net.login_ok.connect(_on_login_ok)
	_net.char_created.connect(_on_char_created)
	_net.char_deleted.connect(_on_char_deleted)
	_net.welcome_received.connect(_on_welcome)
	_net.snapshot_received.connect(_on_snapshot)
	_net.pong_received.connect(_on_pong)
	_net.server_error.connect(_on_server_error)
	_net.self_state_received.connect(_on_self_state)
	_net.damage_received.connect(_on_damage)
	_net.mob_dead_received.connect(_on_mob_dead)
	_net.death_received.connect(_on_death)
	_net.levelup_received.connect(_on_levelup)
	_net.teleport_received.connect(_on_teleport)

	_ui.login_requested.connect(_on_ui_login)
	_ui.register_requested.connect(_on_ui_register)
	_ui.create_requested.connect(_on_ui_create)
	_ui.select_requested.connect(_on_ui_select)
	_ui.delete_requested.connect(_on_ui_delete)

	_hud.potion_requested.connect(_on_hud_potion)
	_hud.alloc_stat_requested.connect(_on_hud_alloc_stat)
	_hud.reset()

	_ui.show_auth("Conectando al servidor...")
	if _autotest:
		_autotest_watchdog()


func _process(_delta: float) -> void:
	var now := Time.get_ticks_msec()
	for entity in _entities.values():
		entity.render_at(now, INTERP_DELAY_MS)
	for mob in _mobs.values():
		mob.render_at(now, INTERP_DELAY_MS)

	if _self_entity != null and is_instance_valid(_self_entity):
		var target := _self_entity.position + CAMERA_OFFSET
		_camera.position = _camera.position.lerp(target, 0.15)


func _unhandled_input(event: InputEvent) -> void:
	if not _in_game:
		return

	if event is InputEventMouseButton and event.button_index == MOUSE_BUTTON_LEFT and event.pressed:
		_handle_click(event.position)
		return

	if event is InputEventKey and event.pressed and not event.echo:
		match event.physical_keycode:
			KEY_1:
				_net.use_potion("hp")
			KEY_2:
				_net.use_potion("mp")


func _handle_click(screen_pos: Vector2) -> void:
	var point := _ground_point(screen_pos)
	if point == Vector3.INF:
		return

	# Prioridad: si el clic cae cerca de un mob, atacarlo (el servidor se
	# encarga de acercarse). Si no, caminar.
	var mob_id := _mob_near(point, 1.8)
	if mob_id != 0:
		_net.attack(mob_id)
		var mob: MobEntityScript = _mobs[mob_id]
		_show_marker(mob.position)
	else:
		_net.send_move(point)
		_show_marker(point)


func _ground_point(screen_pos: Vector2) -> Vector3:
	var origin := _camera.project_ray_origin(screen_pos)
	var dir := _camera.project_ray_normal(screen_pos)
	if absf(dir.y) < 0.0001:
		return Vector3.INF
	var t := -origin.y / dir.y
	if t < 0.0:
		return Vector3.INF
	var point := origin + dir * t
	point.x = clampf(point.x, -MAP_HALF, MAP_HALF)
	point.z = clampf(point.z, -MAP_HALF, MAP_HALF)
	point.y = 0.0
	return point


func _mob_near(point: Vector3, radius: float) -> int:
	var best_id := 0
	var best_dist := radius
	for id in _mobs.keys():
		var mob: MobEntityScript = _mobs[id]
		var dist := Vector2(mob.position.x - point.x, mob.position.z - point.z).length()
		if dist < best_dist:
			best_dist = dist
			best_id = int(id)
	return best_id


func _show_marker(point: Vector3) -> void:
	if _marker_tween != null and _marker_tween.is_valid():
		_marker_tween.kill()
	_marker.position = point + Vector3(0, 0.03, 0)
	_marker.scale = Vector3.ONE
	_marker.transparency = 0.0
	_marker.visible = true
	_marker_tween = create_tween().set_parallel(true)
	_marker_tween.tween_property(_marker, "scale", Vector3(1.7, 1.0, 1.7), 0.45)
	_marker_tween.tween_property(_marker, "transparency", 1.0, 0.45)
	_marker_tween.chain().tween_callback(_hide_marker)


func _hide_marker() -> void:
	_marker.visible = false


## --- Señales del NetClient ---

func _on_net_connected() -> void:
	if not _in_game:
		_ui.set_status("Conectado al servidor.")
	if _autotest and _autotest_state == 0:
		_autotest_state = 1
		_net.request_login(AUTOTEST_USER, AUTOTEST_PASS)


func _on_net_disconnected(reason: String) -> void:
	_in_game = false
	clear_entities()
	_hud.reset()
	_ui.show_auth("Desconectado: %s\nReintentando automáticamente..." % reason)


func _on_login_ok(data: Dictionary) -> void:
	_in_game = false
	clear_entities()
	_hud.reset()

	var user := str(data.get("user", "?"))
	var chars_variant: Variant = data.get("chars", [])
	var chars: Array = chars_variant if chars_variant is Array else []
	_ui.show_lobby(user, chars)
	print("[game] sesión iniciada como %s (%d personajes)" % [user, chars.size()])

	if _autotest:
		if chars.is_empty():
			_autotest_state = 3
			_net.create_character(AUTOTEST_CHAR, "warrior")
		else:
			_autotest_state = 4
			var first: Dictionary = chars[0]
			_net.select_character(int(first.get("id", 0)))


func _on_char_created(data: Dictionary) -> void:
	var char_variant: Variant = data.get("char", {})
	if not (char_variant is Dictionary):
		return
	var character: Dictionary = char_variant
	_ui.add_character(character)
	if _autotest:
		_autotest_state = 4
		_net.select_character(int(character.get("id", 0)))


func _on_char_deleted(char_id: int) -> void:
	_ui.remove_character(char_id)


func _on_welcome(data: Dictionary) -> void:
	_in_game = true
	_ui.show_game()
	clear_entities()
	_hud.reset()

	var char_variant: Variant = data.get("character", {})
	_character = char_variant if char_variant is Dictionary else {}

	var character_name := str(_character.get("name", "?"))
	var class_id := str(_character.get("class", ""))
	var level := int(_character.get("level", 1))
	_hud.set_status("%s · %s · Nv %d  ·  mapa: %s" % [
		character_name,
		_ui.class_label(class_id),
		level,
		str(data.get("map", "?")),
	])

	var self_variant: Variant = data.get("self", {})
	if self_variant is Dictionary:
		var self_data: Dictionary = self_variant
		_self_id = int(self_data.get("id", 0))
		print("[game] posición al entrar: (%.2f, %.2f)" % [
			float(self_data.get("x", 0.0)),
			float(self_data.get("z", 0.0)),
		])

	print("[game] en el mundo como %s (id %d)" % [character_name, int(_character.get("id", 0))])

	if _autotest:
		_autotest_state = 5
		_autotest_attack_sent = false


func _on_snapshot(data: Dictionary) -> void:
	if not _in_game:
		return

	var now := Time.get_ticks_msec()
	_current_tick = int(data.get("tick", 0))

	var ents_variant: Variant = data.get("ents", [])
	if not (ents_variant is Array):
		return
	var ents: Array = ents_variant

	var seen_pcs := {}
	var seen_mobs := {}

	for entry in ents:
		if not (entry is Dictionary):
			continue
		var e: Dictionary = entry
		var id := int(e.get("id", 0))
		if id == 0:
			continue

		var pos := Vector3(float(e.get("x", 0.0)), 0.0, float(e.get("z", 0.0)))
		var kind := str(e.get("k", "pc"))

		if kind == "mob":
			seen_mobs[id] = true
			var mob: MobEntityScript = _mobs.get(id) as MobEntityScript
			if mob == null:
				mob = MOB_SCENE.instantiate() as MobEntityScript
				if mob == null:
					continue
				_entities_root.add_child(mob)
				mob.setup(
					id,
					str(e.get("name", "Mob")),
					int(e.get("level", 1)),
					str(e.get("tmpl", "")),
					int(e.get("hp", 1)),
					int(e.get("hp_max", 1))
				)
				_mobs[id] = mob
			mob.push_state(
				pos, now,
				str(e.get("anim", "idle")),
				int(e.get("hp", 1)),
				int(e.get("hp_max", 1))
			)
		else:
			seen_pcs[id] = true
			var node: PlayerEntityScript = _entities.get(id) as PlayerEntityScript
			if node == null:
				node = PLAYER_SCENE.instantiate() as PlayerEntityScript
				if node == null:
					continue
				_entities_root.add_child(node)
				var is_self := id == _self_id
				node.setup(id, str(e.get("name", "Player")), is_self)
				_entities[id] = node
				if is_self:
					_self_entity = node
					_camera.position = node.position + CAMERA_OFFSET
			node.push_state(pos, now, str(e.get("anim", "idle")))

	_remove_missing(_entities, seen_pcs, true)
	_remove_missing(_mobs, seen_mobs, false)
	_update_info()

	if _autotest and _autotest_state == 5 and not _autotest_attack_sent:
		var target := _nearest_mob_for_autotest()
		if target != 0:
			_autotest_attack_sent = true
			_autotest_state = 6
			print("[autotest] atacando mob %d" % target)
			_net.attack(target)


func _remove_missing(collection: Dictionary, seen: Dictionary, is_player: bool) -> void:
	for id in collection.keys():
		if seen.has(id):
			continue
		if is_player:
			var dead_player: PlayerEntityScript = collection[id] as PlayerEntityScript
			if dead_player == _self_entity:
				_self_entity = null
		var node: Node3D = collection[id] as Node3D
		if node != null:
			node.queue_free()
		collection.erase(id)


func _nearest_mob_for_autotest() -> int:
	var origin := Vector3.ZERO
	if _self_entity != null:
		origin = _self_entity.position

	# Preferir babosas (pasivas) para que el smoke test sea seguro.
	for preferred_passive in [true, false]:
		var best_id := 0
		var best_dist := INF
		for id in _mobs.keys():
			var mob: MobEntityScript = _mobs[id]
			if preferred_passive and mob.template_id != "babosa":
				continue
			var d := mob.position.distance_to(origin)
			if d < best_dist:
				best_dist = d
				best_id = int(id)
		if best_id != 0:
			return best_id
	return 0


func _on_self_state(data: Dictionary) -> void:
	_hud.update_self_state(data)


func _on_damage(data: Dictionary) -> void:
	var dst := int(data.get("dst", 0))
	var node: Node3D = _entities.get(dst) as Node3D
	if node == null:
		node = _mobs.get(dst) as Node3D
	if node == null:
		return
	var screen := _camera.unproject_position(node.position + Vector3(0, 2.0, 0))
	_hud.float_damage(
		screen,
		int(data.get("amount", 0)),
		bool(data.get("crit", false)),
		str(data.get("kind", "out"))
	)


func _on_mob_dead(data: Dictionary) -> void:
	var mob_id := int(data.get("id", 0))
	var xp := int(data.get("xp", 0))
	var node: MobEntityScript = _mobs.get(mob_id) as MobEntityScript
	if node != null:
		var screen := _camera.unproject_position(node.position + Vector3(0, 2.2, 0))
		_hud.float_text(screen, "+%d XP" % xp, Color(0.55, 1.0, 0.6))

	if _autotest and not _autotest_done:
		_autotest_done = true
		print("[autotest] combate OK: +%d XP" % xp)
		get_tree().quit(0)


func _on_death(data: Dictionary) -> void:
	var lost := int(data.get("xp_lost", 0))
	_hud.toast("Moriste · -%d XP" % lost, Color(1.0, 0.45, 0.4))


func _on_levelup(data: Dictionary) -> void:
	var level := int(data.get("level", 1))
	_hud.toast("¡Subiste a nivel %d! +5 puntos" % level, Color(1.0, 0.85, 0.35))


func _on_teleport(data: Dictionary) -> void:
	var pos := Vector3(float(data.get("x", 0.0)), 0.0, float(data.get("z", 0.0)))
	if _self_entity != null and is_instance_valid(_self_entity):
		_self_entity.snap_to(pos)
	_camera.position = pos + CAMERA_OFFSET
	_marker.visible = false


func _on_pong(rtt_ms: int) -> void:
	_last_rtt = rtt_ms
	if _in_game:
		_update_info()


func _on_server_error(code: String, message: String) -> void:
	_ui.set_error(message)
	if _autotest and code == "auth_failed" and _autotest_state == 1:
		_autotest_state = 2
		_net.request_register(AUTOTEST_USER, AUTOTEST_PASS)


func _update_info() -> void:
	_hud.set_info("tick %d  ·  jugadores: %d  ·  mobs: %d  ·  RTT: %d ms" % [
		_current_tick,
		_entities.size(),
		_mobs.size(),
		_last_rtt,
	])


## --- Señales de la UI ---

func _on_ui_login(user: String, password: String) -> void:
	_net.request_login(user, password)


func _on_ui_register(user: String, password: String) -> void:
	_net.request_register(user, password)


func _on_ui_create(character_name: String, class_id: String) -> void:
	_net.create_character(character_name, class_id)


func _on_ui_select(character_id: int) -> void:
	_net.select_character(character_id)


func _on_ui_delete(character_id: int) -> void:
	_net.delete_character(character_id)


func _on_hud_potion(kind: String) -> void:
	_net.use_potion(kind)


func _on_hud_alloc_stat(stat: String) -> void:
	_net.alloc_stat(stat)


## --- Utilidades ---

func clear_entities() -> void:
	for entity in _entities.values():
		if entity != null:
			entity.queue_free()
	_entities.clear()
	for mob in _mobs.values():
		if mob != null:
			mob.queue_free()
	_mobs.clear()
	_self_entity = null
	_self_id = 0


## --- Autotest headless ---

func _autotest_watchdog() -> void:
	await get_tree().create_timer(30.0).timeout
	if not _autotest_done:
		print("[autotest] TIMEOUT")
		get_tree().quit(1)
