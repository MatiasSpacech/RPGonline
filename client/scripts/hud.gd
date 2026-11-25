class_name GameHUD
extends CanvasLayer
## HUD de juego: barras HP/MP/XP, pociones, asignación de stats, toasts y
## números flotantes. Construida por código para mantener main.tscn simple.

signal potion_requested(kind: String)
signal alloc_stat_requested(stat: String)

var _root: Control
var _status: Label
var _info: Label
var _level_label: Label

var _hp_bar: ProgressBar
var _hp_label: Label
var _mp_bar: ProgressBar
var _mp_label: Label
var _xp_bar: ProgressBar
var _xp_label: Label

var _hp_potion_button: Button
var _mp_potion_button: Button
var _points_label: Label
var _stat_labels := {}
var _stat_buttons := {}

var _toast: Label
var _toast_token := 0


func _ready() -> void:
	_build()


func _build() -> void:
	_root = Control.new()
	_root.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_root.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(_root)

	_build_top_left()
	_build_bottom_left()
	_build_bottom_right()
	_build_toast()


func _build_top_left() -> void:
	var box := VBoxContainer.new()
	box.position = Vector2(24, 18)
	box.add_theme_constant_override("separation", 2)
	box.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_root.add_child(box)

	_status = Label.new()
	_status.add_theme_font_size_override("font_size", 20)
	_status.add_theme_color_override("font_color", Color(0.85, 0.9, 1.0))
	_status.mouse_filter = Control.MOUSE_FILTER_IGNORE
	box.add_child(_status)

	_info = Label.new()
	_info.add_theme_font_size_override("font_size", 14)
	_info.add_theme_color_override("font_color", Color(0.6, 0.68, 0.82))
	_info.mouse_filter = Control.MOUSE_FILTER_IGNORE
	box.add_child(_info)


func _build_bottom_left() -> void:
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 5)
	box.mouse_filter = Control.MOUSE_FILTER_IGNORE

	_level_label = Label.new()
	_level_label.add_theme_font_size_override("font_size", 18)
	_level_label.add_theme_color_override("font_color", Color(0.95, 0.85, 0.55))
	_level_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	box.add_child(_level_label)

	var hp := _make_bar("HP", Color(0.85, 0.25, 0.25))
	_hp_label = hp.label
	_hp_bar = hp.bar
	box.add_child(hp.box)

	var mp := _make_bar("MP", Color(0.25, 0.45, 0.9))
	_mp_label = mp.label
	_mp_bar = mp.bar
	box.add_child(mp.box)

	var xp := _make_bar("XP", Color(0.85, 0.75, 0.25))
	_xp_label = xp.label
	_xp_bar = xp.bar
	box.add_child(xp.box)

	_root.add_child(box)
	box.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_LEFT, Control.PRESET_MODE_MINSIZE, 24)


func _make_bar(title: String, fill_color: Color) -> Dictionary:
	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 2)
	box.mouse_filter = Control.MOUSE_FILTER_IGNORE

	var label := Label.new()
	label.text = title
	label.add_theme_font_size_override("font_size", 12)
	label.add_theme_color_override("font_color", Color(0.82, 0.86, 0.95))
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	box.add_child(label)

	var bar := ProgressBar.new()
	bar.custom_minimum_size = Vector2(300, 16)
	bar.show_percentage = false
	bar.max_value = 1
	bar.value = 0
	bar.mouse_filter = Control.MOUSE_FILTER_IGNORE

	var background := StyleBoxFlat.new()
	background.bg_color = Color(0.05, 0.06, 0.09, 0.85)
	background.corner_radius_top_left = 4
	background.corner_radius_top_right = 4
	background.corner_radius_bottom_left = 4
	background.corner_radius_bottom_right = 4
	bar.add_theme_stylebox_override("background", background)

	var fill := StyleBoxFlat.new()
	fill.bg_color = fill_color
	fill.corner_radius_top_left = 4
	fill.corner_radius_top_right = 4
	fill.corner_radius_bottom_left = 4
	fill.corner_radius_bottom_right = 4
	bar.add_theme_stylebox_override("fill", fill)

	box.add_child(bar)
	return {"box": box, "bar": bar, "label": label}


func _build_bottom_right() -> void:
	var panel := PanelContainer.new()
	panel.mouse_filter = Control.MOUSE_FILTER_STOP

	var margin := MarginContainer.new()
	margin.add_theme_constant_override("margin_left", 14)
	margin.add_theme_constant_override("margin_right", 14)
	margin.add_theme_constant_override("margin_top", 10)
	margin.add_theme_constant_override("margin_bottom", 10)
	panel.add_child(margin)

	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 7)
	margin.add_child(box)

	var potions := HBoxContainer.new()
	potions.add_theme_constant_override("separation", 8)
	box.add_child(potions)

	_hp_potion_button = Button.new()
	_hp_potion_button.text = "HP (5) [1]"
	_hp_potion_button.focus_mode = Control.FOCUS_NONE
	_hp_potion_button.pressed.connect(_on_hp_potion)
	potions.add_child(_hp_potion_button)

	_mp_potion_button = Button.new()
	_mp_potion_button.text = "MP (5) [2]"
	_mp_potion_button.focus_mode = Control.FOCUS_NONE
	_mp_potion_button.pressed.connect(_on_mp_potion)
	potions.add_child(_mp_potion_button)

	_points_label = Label.new()
	_points_label.text = "Puntos: 0"
	_points_label.add_theme_font_size_override("font_size", 13)
	_points_label.add_theme_color_override("font_color", Color(0.95, 0.85, 0.55))
	_points_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	box.add_child(_points_label)

	var grid := GridContainer.new()
	grid.columns = 4
	grid.add_theme_constant_override("h_separation", 6)
	grid.add_theme_constant_override("v_separation", 4)
	box.add_child(grid)

	var stats := [
		["str", "STR"],
		["agi", "AGI"],
		["vit", "VIT"],
		["ene", "ENE"],
	]
	for entry in stats:
		var stat_id := str(entry[0])
		var stat_label := Label.new()
		stat_label.text = "%s 0" % str(entry[1])
		stat_label.add_theme_font_size_override("font_size", 12)
		stat_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
		grid.add_child(stat_label)
		_stat_labels[stat_id] = stat_label

		var plus := Button.new()
		plus.text = "+"
		plus.focus_mode = Control.FOCUS_NONE
		plus.custom_minimum_size = Vector2(28, 0)
		plus.disabled = true
		plus.pressed.connect(_on_stat_plus.bind(stat_id))
		grid.add_child(plus)
		_stat_buttons[stat_id] = plus

	_root.add_child(panel)
	panel.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_RIGHT, Control.PRESET_MODE_MINSIZE, 24)


func _build_toast() -> void:
	_toast = Label.new()
	_toast.set_anchors_preset(Control.PRESET_CENTER_TOP)
	_toast.offset_left = -420.0
	_toast.offset_right = 420.0
	_toast.offset_top = 84.0
	_toast.offset_bottom = 124.0
	_toast.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_toast.add_theme_font_size_override("font_size", 24)
	_toast.add_theme_constant_override("outline_size", 6)
	_toast.add_theme_color_override("font_outline_color", Color(0.02, 0.02, 0.03, 1))
	_toast.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_toast.visible = false
	_root.add_child(_toast)


## --- API de main.gd ---

func set_status(text: String) -> void:
	_status.text = text


func set_info(text: String) -> void:
	_info.text = text


func reset() -> void:
	_status.text = ""
	_info.text = ""
	_level_label.text = ""
	_set_bar(_hp_bar, _hp_label, "HP", 0, 0)
	_set_bar(_mp_bar, _mp_label, "MP", 0, 0)
	_set_bar(_xp_bar, _xp_label, "XP", 0, 0)
	_hp_potion_button.text = "HP (0) [1]"
	_mp_potion_button.text = "MP (0) [2]"
	_points_label.text = "Puntos: 0"
	for stat_id in _stat_labels:
		_stat_labels[stat_id].text = "%s 0" % str(stat_id).to_upper()
		_stat_buttons[stat_id].disabled = true
	_toast_token += 1
	_toast.visible = false


func update_self_state(state: Dictionary) -> void:
	var hp := int(state.get("hp", 0))
	var hp_max := int(state.get("hp_max", 0))
	var mp := int(state.get("mp", 0))
	var mp_max := int(state.get("mp_max", 0))
	var xp := int(state.get("xp", 0))
	var xp_next := int(state.get("xp_next", 0))
	var level := int(state.get("level", 1))
	var points := int(state.get("stat_points", 0))

	_level_label.text = "Nivel %d" % level
	_set_bar(_hp_bar, _hp_label, "HP", hp, hp_max)
	_set_bar(_mp_bar, _mp_label, "MP", mp, mp_max)
	_set_bar(_xp_bar, _xp_label, "XP", xp, xp_next)

	_hp_potion_button.text = "HP (%d) [1]" % int(state.get("hp_potions", 0))
	_mp_potion_button.text = "MP (%d) [2]" % int(state.get("mp_potions", 0))
	_hp_potion_button.disabled = int(state.get("hp_potions", 0)) <= 0
	_mp_potion_button.disabled = int(state.get("mp_potions", 0)) <= 0

	_points_label.text = "Puntos: %d" % points
	_stat_labels["str"].text = "STR %d" % int(state.get("str", 0))
	_stat_labels["agi"].text = "AGI %d" % int(state.get("agi", 0))
	_stat_labels["vit"].text = "VIT %d" % int(state.get("vit", 0))
	_stat_labels["ene"].text = "ENE %d" % int(state.get("ene", 0))
	for stat_id in _stat_buttons:
		_stat_buttons[stat_id].disabled = points <= 0


func toast(text: String, color: Color = Color.WHITE) -> void:
	_toast_token += 1
	var token := _toast_token
	_toast.text = text
	_toast.add_theme_color_override("font_color", color)
	_toast.modulate = Color.WHITE
	_toast.visible = true

	await get_tree().create_timer(1.8).timeout
	if token != _toast_token:
		return
	var tween := create_tween()
	tween.tween_property(_toast, "modulate:a", 0.0, 0.4)
	await tween.finished
	if token == _toast_token:
		_toast.visible = false


func float_text(screen_pos: Vector2, text: String, color: Color, big: bool = false) -> void:
	var label := Label.new()
	label.text = text
	label.position = screen_pos - Vector2(24, 10)
	label.z_index = 100
	label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	label.add_theme_font_size_override("font_size", 28 if big else 18)
	label.add_theme_color_override("font_color", color)
	label.add_theme_constant_override("outline_size", 5)
	label.add_theme_color_override("font_outline_color", Color(0.02, 0.02, 0.03, 1))
	_root.add_child(label)

	var tween := create_tween().set_parallel(true)
	tween.tween_property(label, "position", label.position + Vector2(0, -58), 0.8)
	tween.tween_property(label, "modulate:a", 0.0, 0.8)
	tween.chain().tween_callback(label.queue_free)


func float_damage(screen_pos: Vector2, amount: int, crit: bool, kind: String) -> void:
	var text := str(amount)
	var color := Color(1.0, 0.92, 0.6) # daño que hacés
	match kind:
		"in":
			color = Color(1.0, 0.45, 0.35)
		"dodge":
			text = "Miss"
			color = Color(0.75, 0.8, 0.9)
	if crit:
		text += "!"
		color = Color(1.0, 0.75, 0.2)
	float_text(screen_pos, text, color, crit)


## --- Interno ---

func _set_bar(bar: ProgressBar, label: Label, title: String, value: int, maximum: int) -> void:
	var max_value := maxi(1, maximum)
	bar.max_value = max_value
	bar.value = clampi(value, 0, max_value)
	if maximum > 0:
		label.text = "%s %d / %d" % [title, value, maximum]
	else:
		label.text = title


func _on_hp_potion() -> void:
	potion_requested.emit("hp")


func _on_mp_potion() -> void:
	potion_requested.emit("mp")


func _on_stat_plus(stat_id: String) -> void:
	alloc_stat_requested.emit(stat_id)
