class_name MobEntity
extends Node3D
## Representación de un mob: interpolación entre snapshots + nombre/nivel/HP.

const BASE_CAPSULE_Y := 0.9

@onready var _capsule: MeshInstance3D = $Capsule
@onready var _label: Label3D = $Name

var mob_id := 0
var template_id := ""
var display_name := "Mob"
var level := 1
var hp := 1
var hp_max := 1

var _prev_pos := Vector3.ZERO
var _curr_pos := Vector3.ZERO
var _prev_t := 0
var _curr_t := 0
var _has_state := false
var _anim := "idle"
var _label_hp := -1


func setup(id: int, mob_name: String, mob_level: int, tmpl: String, hp_value: int, hp_max_value: int) -> void:
	mob_id = id
	display_name = mob_name
	level = mob_level
	template_id = tmpl
	hp_max = max(1, hp_max_value)
	hp = clamp(hp_value, 0, hp_max)

	_label.text = "%s · Nv %d" % [display_name, level]
	_apply_look()
	_update_hp_label()
	print("[game] mob #%d '%s' (%s)" % [mob_id, display_name, template_id])


func push_state(pos: Vector3, t_ms: int, anim: String, hp_value: int, hp_max_value: int) -> void:
	_anim = anim
	hp_max = max(1, hp_max_value)
	hp = clamp(hp_value, 0, hp_max)
	_update_hp_label()

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
		rotation.y = lerp_angle(rotation.y, atan2(delta.x, delta.z), 0.25)

	if _anim == "chase":
		_capsule.position.y = BASE_CAPSULE_Y + absf(sin(Time.get_ticks_msec() / 110.0)) * 0.05
	else:
		_capsule.position.y = lerpf(_capsule.position.y, BASE_CAPSULE_Y, 0.2)


func _apply_look() -> void:
	var color := Color(0.6, 0.6, 0.6)
	var root_scale := 1.0
	match template_id:
		"babosa":
			color = Color(0.35, 0.75, 0.35)
			root_scale = 0.7
		"lobo":
			color = Color(0.55, 0.55, 0.62)
			root_scale = 0.9
		"bandido":
			color = Color(0.7, 0.5, 0.3)
			root_scale = 1.0
		"escorpion":
			color = Color(0.85, 0.55, 0.2)
			root_scale = 1.0
		"golem":
			color = Color(0.35, 0.4, 0.5)
			root_scale = 1.5
	scale = Vector3.ONE * root_scale

	if _capsule.material_override == null:
		return
	var mat := _capsule.material_override.duplicate() as StandardMaterial3D
	if mat != null:
		mat.albedo_color = color
		_capsule.material_override = mat


func _update_hp_label() -> void:
	if hp == _label_hp:
		return
	_label_hp = hp
	var pct := int(round(100.0 * float(hp) / float(max(1, hp_max))))
	_label.text = "%s · Nv %d  ·  %d%%" % [display_name, level, pct]
	if pct > 50:
		_label.modulate = Color(0.75, 1.0, 0.75)
	elif pct > 25:
		_label.modulate = Color(1.0, 0.9, 0.55)
	else:
		_label.modulate = Color(1.0, 0.6, 0.6)
