extends Node3D
## F1 — "Caminar juntos": click-to-move, snapshots e interpolación.
## El servidor es la autoridad: acá solo se pide moverse y se renderiza.

const INTERP_DELAY_MS := 100
const MAP_HALF := 19.0
const CAMERA_OFFSET := Vector3(0, 8, 10)
const NetClientScript := preload("res://scripts/net_client.gd")
const PlayerEntityScript := preload("res://scripts/player_entity.gd")
const PLAYER_SCENE := preload("res://scenes/player_entity.tscn")

@onready var _net: NetClientScript = $NetClient
@onready var _camera: Camera3D = $Camera3D
@onready var _entities_root: Node3D = $Entities
@onready var _marker: MeshInstance3D = $MoveMarker
@onready var _status: Label = $UI/StatusLabel
@onready var _info: Label = $UI/InfoLabel

var _entities: Dictionary = {}
var _self_id := 0
var _self_entity: PlayerEntityScript = null
var _current_tick := 0
var _last_rtt := 0
var _marker_tween: Tween


func _ready() -> void:
	_net.connected.connect(_on_net_connected)
	_net.disconnected.connect(_on_net_disconnected)
	_net.welcome_received.connect(_on_welcome)
	_net.snapshot_received.connect(_on_snapshot)
	_net.pong_received.connect(_on_pong)

	_status.text = "Conectando a ws://127.0.0.1:8080/ws ..."
	_net.login("dev")


func _process(_delta: float) -> void:
	var now := Time.get_ticks_msec()
	for entity in _entities.values():
		entity.render_at(now, INTERP_DELAY_MS)

	if _self_entity != null and is_instance_valid(_self_entity):
		var target := _self_entity.position + CAMERA_OFFSET
		_camera.position = _camera.position.lerp(target, 0.15)


func _unhandled_input(event: InputEvent) -> void:
	if event is InputEventMouseButton and event.button_index == MOUSE_BUTTON_LEFT and event.pressed:
		_request_move(event.position)


func _request_move(screen_pos: Vector2) -> void:
	if _self_entity == null:
		return

	# Rayo desde la cámara al plano del suelo (y = 0).
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


func _on_net_connected() -> void:
	clear_entities()
	_status.text = "Conectado — autenticando..."


func _on_net_disconnected(reason: String) -> void:
	clear_entities()
	_status.text = "Desconectado: %s\nReintentando automáticamente..." % reason
	_info.text = ""


func _on_welcome(data: Dictionary) -> void:
	var self_variant: Variant = data.get("self", {})
	if not (self_variant is Dictionary):
		return
	var self_data: Dictionary = self_variant
	_self_id = int(self_data.get("id", 0))
	_status.text = "Conectado ✓  ·  %s  ·  mapa: %s" % [
		str(self_data.get("name", "?")),
		str(data.get("map", "?")),
	]
	print("[game] welcome: id=%d nombre=%s" % [_self_id, self_data.get("name", "?")])


func _on_snapshot(data: Dictionary) -> void:
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

	# Los snapshots son completos: lo que no aparece, se fue del mundo.
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
	_update_info()


func clear_entities() -> void:
	for entity in _entities.values():
		if entity != null:
			entity.queue_free()
	_entities.clear()
	_self_entity = null
	_self_id = 0
