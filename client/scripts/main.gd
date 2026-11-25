extends Node3D
## F2 — autenticación, lobby de personajes y mundo con click-to-move.
## El servidor es la autoridad: acá se pide y se renderiza.

const INTERP_DELAY_MS := 100
const MAP_HALF := 19.0
const CAMERA_OFFSET := Vector3(0, 8, 10)
const NetClientScript := preload("res://scripts/net_client.gd")
const AuthUIScript := preload("res://scripts/auth_ui.gd")
const PlayerEntityScript := preload("res://scripts/player_entity.gd")
const PLAYER_SCENE := preload("res://scenes/player_entity.tscn")

const AUTOTEST_USER := "autotest"
const AUTOTEST_PASS := "secreto123"
const AUTOTEST_CHAR := "AutoWarrior"

@onready var _net: NetClientScript = $NetClient
@onready var _ui: AuthUIScript = $UIOverlay
@onready var _camera: Camera3D = $Camera3D
@onready var _entities_root: Node3D = $Entities
@onready var _marker: MeshInstance3D = $MoveMarker
@onready var _status: Label = $UI/StatusLabel
@onready var _info: Label = $UI/InfoLabel

var _entities: Dictionary = {}
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

	_ui.login_requested.connect(_on_ui_login)
	_ui.register_requested.connect(_on_ui_register)
	_ui.create_requested.connect(_on_ui_create)
	_ui.select_requested.connect(_on_ui_select)
	_ui.delete_requested.connect(_on_ui_delete)

	_ui.show_auth("Conectando al servidor...")
	if _autotest:
		_autotest_watchdog()


func _process(_delta: float) -> void:
	var now := Time.get_ticks_msec()
	for entity in _entities.values():
		entity.render_at(now, INTERP_DELAY_MS)

	if _self_entity != null and is_instance_valid(_self_entity):
		var target := _self_entity.position + CAMERA_OFFSET
		_camera.position = _camera.position.lerp(target, 0.15)


func _unhandled_input(event: InputEvent) -> void:
	if not _in_game:
		return
	if event is InputEventMouseButton and event.button_index == MOUSE_BUTTON_LEFT and event.pressed:
		_request_move(event.position)


func _request_move(screen_pos: Vector2) -> void:
	if _self_entity == null:
		return

	var origin := _camera.project_ray_origin(screen_pos)
	var dir := _camera.project_ray_normal(screen_pos)
	if absf(dir.y) < 0.0001:
		return
	var t := -origin.y / dir.y
	if t < 0.0:
		return

	var point := origin + dir * t
	point.x = clampf(point.x, -MAP_HALF, MAP_HALF)
	point.z = clampf(point.z, -MAP_HALF, MAP_HALF)
	point.y = 0.0

	_net.send_move(point)
	_show_marker(point)


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
	_ui.show_auth("Desconectado: %s\nReintentando automáticamente..." % reason)


func _on_login_ok(data: Dictionary) -> void:
	_in_game = false
	clear_entities()

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

	var char_variant: Variant = data.get("character", {})
	_character = char_variant if char_variant is Dictionary else {}

	var character_name := str(_character.get("name", "?"))
	var class_id := str(_character.get("class", ""))
	var level := int(_character.get("level", 1))
	_status.text = "%s · %s · Nv %d  ·  mapa: %s" % [
		character_name,
		_ui.class_label(class_id),
		level,
		str(data.get("map", "?")),
	]
	_info.text = ""

	var self_variant: Variant = data.get("self", {})
	if self_variant is Dictionary:
		var self_data: Dictionary = self_variant
		print("[game] posición al entrar: (%.2f, %.2f)" % [
			float(self_data.get("x", 0.0)),
			float(self_data.get("z", 0.0)),
		])

	print("[game] en el mundo como %s (id %d)" % [character_name, int(_character.get("id", 0))])

	if _autotest:
		_autotest_play()


func _on_snapshot(data: Dictionary) -> void:
	if not _in_game:
		return

	var now := Time.get_ticks_msec()
	_current_tick = int(data.get("tick", 0))

	var ents_variant: Variant = data.get("ents", [])
	if not (ents_variant is Array):
		return
	var ents: Array = ents_variant

	var seen := {}
	for entry in ents:
		if not (entry is Dictionary):
			continue
		var e: Dictionary = entry
		var id := int(e.get("id", 0))
		if id == 0:
			continue
		seen[id] = true

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

		var pos := Vector3(float(e.get("x", 0.0)), 0.0, float(e.get("z", 0.0)))
		node.push_state(pos, now, str(e.get("anim", "idle")))

	_update_info()

	for id in _entities.keys():
		if not seen.has(id):
			var dead: PlayerEntityScript = _entities[id] as PlayerEntityScript
			if dead == _self_entity:
				_self_entity = null
			if dead != null:
				dead.queue_free()
			_entities.erase(id)


func _update_info() -> void:
	_info.text = "tick %d  ·  jugadores: %d  ·  RTT: %d ms" % [
		_current_tick,
		_entities.size(),
		_last_rtt,
	]


func _on_pong(rtt_ms: int) -> void:
	_last_rtt = rtt_ms
	if _in_game:
		_update_info()


func _on_server_error(code: String, message: String) -> void:
	_ui.set_error(message)
	if _autotest and code == "auth_failed" and _autotest_state == 1:
		# Primera ejecución: la cuenta no existe todavía.
		_autotest_state = 2
		_net.request_register(AUTOTEST_USER, AUTOTEST_PASS)


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


## --- Utilidades ---

func clear_entities() -> void:
	for entity in _entities.values():
		if entity != null:
			entity.queue_free()
	_entities.clear()
	_self_entity = null
	_self_id = 0


## --- Autotest headless ---

func _autotest_play() -> void:
	_autotest_state = 5
	print("[autotest] personaje en el mundo")
	_net.send_move(Vector3(5, 0, 0))
	await get_tree().create_timer(1.8).timeout
	print("[autotest] OK")
	_autotest_done = true
	get_tree().quit(0)


func _autotest_watchdog() -> void:
	await get_tree().create_timer(20.0).timeout
	if not _autotest_done:
		print("[autotest] TIMEOUT")
		get_tree().quit(1)
