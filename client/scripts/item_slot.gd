class_name ItemSlot
extends Button

signal drop_requested(item_id: int, slot_x: int, slot_y: int)

var slot_x := -1
var slot_y := -1
var inventory_item_id := 0


func _ready() -> void:
	focus_mode = Control.FOCUS_NONE


func _get_drag_data(_at_position: Vector2) -> Variant:
	if inventory_item_id == 0:
		return null
	var preview := Label.new()
	preview.text = text
	preview.add_theme_color_override("font_color", get_theme_color("font_color"))
	preview.modulate = Color(1, 1, 1, 0.9)
	set_drag_preview(preview)
	return {"item_id": inventory_item_id}


func _can_drop_data(_at_position: Vector2, data: Variant) -> bool:
	return data is Dictionary and int(data.get("item_id", 0)) != 0


func _drop_data(_at_position: Vector2, data: Variant) -> void:
	if not (data is Dictionary):
		return
	drop_requested.emit(int(data.get("item_id", 0)), slot_x, slot_y)
