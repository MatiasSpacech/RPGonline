class_name PlayerEntity
extends Node3D
## Representación de un jugador. Interpola entre los dos últimos snapshots
## para que el movimiento se vea fluido aunque el server mande a 10 Hz.

const BASE_CAPSULE_Y := 0.9

@onready var _capsule: MeshInstance3D = $Capsule
@onready var _label: Label3D = $Name

var entity_id := 0
var is_self := false
var display_name := "Player"

var _prev_pos := Vector3.ZERO
var _curr_pos := Vector3.ZERO
var _prev_t := 0
var _curr_t := 0
var _has_state := false
var _moving := false


func setup(id: int, entity_name: String, self_flag: bool) -> void:
	entity_id = id
	display_name = entity_name
	is_self = self_flag
	_label.text = entity_name

	if is_self:
		_label.modulate = Color(1.0, 0.85, 0.4)
		_set_color(Color(0.95, 0.72, 0.25))
	else:
		var hue := fposmod(float(id) * 0.37, 1.0)
		_set_color(Color.from_hsv(hue, 0.55, 0.9))
		_label.modulate = Color(0.82, 0.9, 1.0)

	print("[game] entidad #%d '%s'%s" % [id, entity_name, " (yo)" if is_self else ""])


func _set_color(color: Color) -> void:
	if _capsule.material_override == null:
		return
	var mat := _capsule.material_override.duplicate() as StandardMaterial3D
	if mat == null:
		return
	mat.albedo_color = color
	_capsule.material_override = mat


## Guarda un estado recibido en snapshot. t_ms es el reloj local de recepción.
func push_state(pos: Vector3, t_ms: int, anim: String) -> void:
	_moving = anim == "run"

	if not _has_state:
		_prev_pos = pos
		_curr_pos = pos
		_prev_t = t_ms
		_curr_t = t_ms
		_has_state = true
		position = pos
		return

	_prev_pos = _curr_pos
	_prev_t = _curr_t
	_curr_pos = pos
	_curr_t = t_ms


## Renderiza el estado correspondiente a (now - delay): interpolación con
## ~100 ms de buffer para suavizar el jitter de la red.
func render_at(now_ms: int, delay_ms: int) -> void:
	if not _has_state:
		return

	var render_time := float(now_ms - delay_ms)
	var alpha := 1.0
	if _curr_t > _prev_t:
		alpha = clampf((render_time - float(_prev_t)) / float(_curr_t - _prev_t), 0.0, 1.0)
	position = _prev_pos.lerp(_curr_pos, alpha)

	var delta := _curr_pos - _prev_pos
	if delta.length_squared() > 0.0001:
		var target_rotation := atan2(delta.x, delta.z)
		rotation.y = lerp_angle(rotation.y, target_rotation, 0.25)

	if _moving:
		_capsule.position.y = BASE_CAPSULE_Y + absf(sin(Time.get_ticks_msec() / 90.0)) * 0.07
	else:
		_capsule.position.y = lerpf(_capsule.position.y, BASE_CAPSULE_Y, 0.2)
