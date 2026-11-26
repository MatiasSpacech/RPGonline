class_name DropEntity
extends Node3D
## Ítem en el suelo: caja brillante + nombre. No se interpola (es estático).

@onready var _mesh: MeshInstance3D = $Mesh
@onready var _label: Label3D = $Name

var drop_id := 0
var template_id := ""
var rarity := "common"


func setup(id: int, tmpl: String, display_name: String, item_rarity: String) -> void:
	drop_id = id
	template_id = tmpl
	rarity = item_rarity
	_label.text = display_name
	var color := _rarity_color(rarity)
	_label.modulate = color
	if _mesh.material_override != null:
		var mat := _mesh.material_override.duplicate() as StandardMaterial3D
		if mat != null:
			mat.albedo_color = color
			mat.emission = color
			_mesh.material_override = mat


func _process(delta: float) -> void:
	rotate_y(delta * 1.6)
	_mesh.position.y = 0.35 + sin(Time.get_ticks_msec() / 300.0) * 0.06


func _rarity_color(item_rarity: String) -> Color:
	match item_rarity:
		"good":
			return Color(0.45, 0.95, 0.5)
		"rare":
			return Color(0.35, 0.65, 1.0)
		"epic":
			return Color(0.75, 0.45, 1.0)
		"legendary":
			return Color(1.0, 0.6, 0.2)
		_:
			return Color(0.9, 0.92, 0.95)
