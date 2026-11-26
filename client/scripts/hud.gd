class_name GameHUD
extends CanvasLayer
## HUD de juego: barras HP/MP/XP, pociones, asignación de stats, toasts y
## números flotantes. Construida por código para mantener main.tscn simple.

signal potion_requested(kind: String)
signal alloc_stat_requested(stat: String)
signal inventory_requested
signal skill_requested(skill_id: String)

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
var _combat_label: Label
var _stat_labels := {}
var _stat_buttons := {}

var _toast: Label
var _toast_token := 0

var _skill_bar: HBoxContainer
var _skill_buttons := {}
var _skill_defs := {}
var _skill_base_text := {}
var _skill_ready_ms := {}
var _skill_ids: Array = []
var _player_level := 1
var _player_mp := 0


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
	_build_skill_bar()
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

	var inventory_button := Button.new()
	inventory_button.text = "Inventario [I]"
	inventory_button.focus_mode = Control.FOCUS_NONE
	inventory_button.pressed.connect(func(): inventory_requested.emit())
	box.add_child(inventory_button)

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

	_combat_label = Label.new()
	_combat_label.text = "ATK — · DEF —"
	_combat_label.add_theme_font_size_override("font_size", 12)
	_combat_label.add_theme_color_override("font_color", Color(0.8, 0.85, 0.95))
	_combat_label.mouse_filter = Control.MOUSE_FILTER_IGNORE
	box.add_child(_combat_label)

	_root.add_child(panel)
	panel.set_anchors_and_offsets_preset(Control.PRESET_BOTTOM_RIGHT, Control.PRESET_MODE_MINSIZE, 24)


func _build_skill_bar() -> void:
	_skill_bar = HBoxContainer.new()
	_skill_bar.add_theme_constant_override("separation", 8)
	_skill_bar.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_root.add_child(_skill_bar)


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


func set_skills(skills: Array) -> void:
	for child in _skill_bar.get_children():
		child.queue_free()
	_skill_buttons.clear()
	_skill_defs.clear()
	_skill_base_text.clear()
	_skill_ready_ms.clear()
	_skill_ids.clear()

	var keys := ["Q", "W", "E", "R"]
	var index := 0
	for entry in skills:
		if not (entry is Dictionary):
			continue
		var def: Dictionary = entry
		var skill_id := str(def.get("id", ""))
		if skill_id == "":
			continue
		var key: String = keys[index] if index < keys.size() else str(index + 1)
		index += 1

		var button := Button.new()
		button.focus_mode = Control.FOCUS_NONE
		button.custom_minimum_size = Vector2(150, 54)
		button.add_theme_font_size_override("font_size", 11)
		button.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		button.tooltip_text = _skill_tooltip(def)
		button.pressed.connect(_on_skill_button.bind(skill_id))
		_skill_bar.add_child(button)

		_skill_buttons[skill_id] = button
		_skill_defs[skill_id] = def
		_skill_base_text[skill_id] = "%s · %s" % [key, str(def.get("name", "Skill"))]
		_skill_ids.append(skill_id)

	_skill_bar.set_anchors_and_offsets_preset(Control.PRESET_CENTER_BOTTOM, Control.PRESET_MODE_MINSIZE, 18)


func skill_ids() -> Array:
	return _skill_ids


func start_skill_cooldown(skill_id: String, cooldown_ms: int) -> void:
	_skill_ready_ms[skill_id] = Time.get_ticks_msec() + cooldown_ms


func _process(_delta: float) -> void:
	if _skill_buttons.is_empty():
		return
	var now := Time.get_ticks_msec()
	for skill_id in _skill_buttons:
		var button: Button = _skill_buttons[skill_id]
		var def: Dictionary = _skill_defs[skill_id]
		var base: String = _skill_base_text[skill_id]
		var ready_ms := int(_skill_ready_ms.get(skill_id, 0))
		var remaining := ready_ms - now
		var required_level := int(def.get("level_req", 1))

		if remaining > 0:
			button.disabled = true
			button.text = "%s\n%.1fs" % [base, remaining / 1000.0]
		elif _player_level < required_level:
			button.disabled = true
			button.text = "%s\nNv %d" % [base, required_level]
		else:
			button.disabled = false
			button.text = "%s\n%d MP" % [base, int(def.get("mana_cost", 0))]


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
	_combat_label.text = "ATK — · DEF —"
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
	_player_level = level
	_player_mp = mp

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
	_combat_label.text = "ATK %d-%d · DEF %d" % [
		int(state.get("attack_min", 0)),
		int(state.get("attack_max", 0)),
		int(state.get("defense", 0)),
	]


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


func _on_skill_button(skill_id: String) -> void:
	skill_requested.emit(skill_id)


func _skill_tooltip(def: Dictionary) -> String:
	var lines: Array = []
	lines.append(str(def.get("name", "Skill")))
	lines.append("Maná: %d · CD: %.1fs" % [
		int(def.get("mana_cost", 0)), int(def.get("cooldown_ms", 0)) / 1000.0,
	])
	lines.append("Nivel requerido: %d" % int(def.get("level_req", 1)))
	var kind := str(def.get("kind", ""))
	match kind:
		"single":
			lines.append("Daño x%.1f en un objetivo" % float(def.get("multiplier", 1.0)))
		"aoe":
			lines.append("Daño x%.1f en área (radio %.1f m)" % [
				float(def.get("multiplier", 1.0)), float(def.get("radius", 0.0)),
			])
		"heal":
			lines.append("Cura %d%% del HP máximo" % int(def.get("heal_percent", 0)))
		"buff":
			lines.append("+%d%% daño por %.0fs" % [
				int(def.get("buff_percent", 0)), int(def.get("buff_duration_ms", 0)) / 1000.0,
			])
		"teleport":
			lines.append("Teletransporte hasta %.1f m" % float(def.get("teleport_distance", 0.0)))
	return "\n".join(PackedStringArray(lines))
