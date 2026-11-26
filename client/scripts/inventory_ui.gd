extends CanvasLayer
## Inventario (8x5), equipo, detalle de ítems y tienda. Construido por código.

signal closed
signal equip_requested(item_id: int)
signal unequip_requested(slot: String)
signal use_requested(item_id: int)
signal enhance_requested(item_id: int)
signal sell_requested(item_id: int)
signal move_requested(item_id: int, x: int, y: int)
signal buy_requested(tmpl: String)

const ItemSlotScript := preload("res://scripts/item_slot.gd")

const SLOTS := ["weapon", "helm", "chest", "gloves", "boots", "shield"]
const SLOT_LABELS := {
	"weapon": "Arma", "helm": "Casco", "chest": "Pecho",
	"gloves": "Guantes", "boots": "Botas", "shield": "Escudo",
}
const RARITY_NAMES := {
	"common": "Común", "good": "Bueno", "rare": "Raro",
	"epic": "Épico", "legendary": "Legendario",
}
const RARITY_COLORS := {
	"common": Color(0.88, 0.9, 0.94),
	"good": Color(0.45, 0.95, 0.5),
	"rare": Color(0.35, 0.65, 1.0),
	"epic": Color(0.75, 0.45, 1.0),
	"legendary": Color(1.0, 0.6, 0.2),
}

var _root: Control
var _templates: Dictionary = {}
var _items: Array = []
var _gold := 0
var _selected_id := 0
var _selected_slot := ""
var _attack_min := 0
var _attack_max := 0
var _defense := 0
var _is_open := false

var _gold_label: Label
var _combat_label: Label
var _inv_slots: Array = []
var _equip_buttons: Dictionary = {}
var _detail_name: Label
var _detail_info: Label
var _action_buttons: Dictionary = {}
var _shop_panel: PanelContainer
var _shop_list: VBoxContainer


func _ready() -> void:
	_build()
	_root.visible = false


func _build() -> void:
	_root = Control.new()
	_root.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_root.mouse_filter = Control.MOUSE_FILTER_STOP
	add_child(_root)

	var bg := ColorRect.new()
	bg.color = Color(0.03, 0.04, 0.06, 0.92)
	bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	bg.mouse_filter = Control.MOUSE_FILTER_STOP
	_root.add_child(bg)

	var center := CenterContainer.new()
	center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_root.add_child(center)

	var panel := PanelContainer.new()
	panel.custom_minimum_size = Vector2(1000, 0)
	center.add_child(panel)

	var margin := MarginContainer.new()
	margin.add_theme_constant_override("margin_left", 24)
	margin.add_theme_constant_override("margin_right", 24)
	margin.add_theme_constant_override("margin_top", 18)
	margin.add_theme_constant_override("margin_bottom", 18)
	panel.add_child(margin)

	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 10)
	margin.add_child(box)

	_build_header(box)

	var main := HBoxContainer.new()
	main.add_theme_constant_override("separation", 18)
	box.add_child(main)

	_build_equipment(main)
	_build_inventory(main)
	_build_detail(box)
	_build_shop(box)


func _build_header(box: VBoxContainer) -> void:
	var header := HBoxContainer.new()
	header.add_theme_constant_override("separation", 12)
	box.add_child(header)

	var title := Label.new()
	title.text = "Inventario"
	title.add_theme_font_size_override("font_size", 26)
	header.add_child(title)

	_gold_label = Label.new()
	_gold_label.text = "Oro: 0"
	_gold_label.add_theme_font_size_override("font_size", 16)
	_gold_label.add_theme_color_override("font_color", Color(0.95, 0.82, 0.35))
	_gold_label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	header.add_child(_gold_label)

	var shop_button := Button.new()
	shop_button.text = "Tienda"
	shop_button.focus_mode = Control.FOCUS_NONE
	shop_button.pressed.connect(_toggle_shop)
	header.add_child(shop_button)

	var close_button := Button.new()
	close_button.text = "Cerrar [I]"
	close_button.focus_mode = Control.FOCUS_NONE
	close_button.pressed.connect(close)
	header.add_child(close_button)


func _build_equipment(parent: HBoxContainer) -> void:
	var box := VBoxContainer.new()
	box.custom_minimum_size = Vector2(250, 0)
	box.add_theme_constant_override("separation", 6)
	parent.add_child(box)

	var title := Label.new()
	title.text = "Equipo"
	title.add_theme_font_size_override("font_size", 18)
	box.add_child(title)

	var grid := GridContainer.new()
	grid.columns = 2
	grid.add_theme_constant_override("h_separation", 6)
	grid.add_theme_constant_override("v_separation", 6)
	box.add_child(grid)

	for slot in SLOTS:
		var button := Button.new()
		button.text = "%s\n—" % SLOT_LABELS[slot]
		button.custom_minimum_size = Vector2(118, 52)
		button.focus_mode = Control.FOCUS_NONE
		button.add_theme_font_size_override("font_size", 11)
		button.pressed.connect(_on_equip_slot_pressed.bind(slot))
		grid.add_child(button)
		_equip_buttons[slot] = button

	_combat_label = Label.new()
	_combat_label.text = "ATK — · DEF —"
	_combat_label.add_theme_font_size_override("font_size", 13)
	_combat_label.add_theme_color_override("font_color", Color(0.8, 0.85, 0.95))
	box.add_child(_combat_label)

	var hint := Label.new()
	hint.text = "Click: ver detalle.\nArrastrá: mover en la mochila."
	hint.add_theme_font_size_override("font_size", 11)
	hint.add_theme_color_override("font_color", Color(0.55, 0.6, 0.72))
	box.add_child(hint)


func _build_inventory(parent: HBoxContainer) -> void:
	var box := VBoxContainer.new()
	box.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	box.add_theme_constant_override("separation", 6)
	parent.add_child(box)

	var title := Label.new()
	title.text = "Mochila 8×5"
	title.add_theme_font_size_override("font_size", 18)
	box.add_child(title)

	var grid := GridContainer.new()
	grid.columns = 8
	grid.add_theme_constant_override("h_separation", 5)
	grid.add_theme_constant_override("v_separation", 5)
	box.add_child(grid)

	for y in 5:
		for x in 8:
			var slot = ItemSlotScript.new()
			slot.slot_x = x
			slot.slot_y = y
			slot.custom_minimum_size = Vector2(62, 58)
			slot.add_theme_font_size_override("font_size", 10)
			slot.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
			slot.pressed.connect(_on_inventory_slot_pressed.bind(slot))
			slot.drop_requested.connect(_on_drop_requested)
			grid.add_child(slot)
			_inv_slots.append(slot)


func _build_detail(box: VBoxContainer) -> void:
	var panel := PanelContainer.new()
	box.add_child(panel)

	var margin := MarginContainer.new()
	margin.add_theme_constant_override("margin_left", 12)
	margin.add_theme_constant_override("margin_right", 12)
	margin.add_theme_constant_override("margin_top", 8)
	margin.add_theme_constant_override("margin_bottom", 8)
	panel.add_child(margin)

	var inner := VBoxContainer.new()
	inner.add_theme_constant_override("separation", 4)
	margin.add_child(inner)

	_detail_name = Label.new()
	_detail_name.text = "Seleccioná un ítem"
	_detail_name.add_theme_font_size_override("font_size", 18)
	inner.add_child(_detail_name)

	_detail_info = Label.new()
	_detail_info.text = ""
	_detail_info.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_detail_info.add_theme_font_size_override("font_size", 12)
	_detail_info.add_theme_color_override("font_color", Color(0.78, 0.82, 0.9))
	inner.add_child(_detail_info)

	var actions := HBoxContainer.new()
	actions.add_theme_constant_override("separation", 8)
	inner.add_child(actions)

	var defs := [
		["equip", "Equipar"],
		["unequip", "Desequipar"],
		["use", "Usar"],
		["enhance", "Mejorar +"],
		["sell", "Vender"],
	]
	for entry in defs:
		var button := Button.new()
		button.text = str(entry[1])
		button.focus_mode = Control.FOCUS_NONE
		button.visible = false
		button.pressed.connect(_on_action_pressed.bind(str(entry[0])))
		actions.add_child(button)
		_action_buttons[str(entry[0])] = button


func _build_shop(box: VBoxContainer) -> void:
	_shop_panel = PanelContainer.new()
	_shop_panel.visible = false
	box.add_child(_shop_panel)

	var margin := MarginContainer.new()
	margin.add_theme_constant_override("margin_left", 14)
	margin.add_theme_constant_override("margin_right", 14)
	margin.add_theme_constant_override("margin_top", 10)
	margin.add_theme_constant_override("margin_bottom", 10)
	_shop_panel.add_child(margin)

	var inner := VBoxContainer.new()
	inner.add_theme_constant_override("separation", 6)
	margin.add_child(inner)

	var title := Label.new()
	title.text = "Vendedor"
	title.add_theme_font_size_override("font_size", 18)
	inner.add_child(title)

	_shop_list = VBoxContainer.new()
	_shop_list.add_theme_constant_override("separation", 4)
	inner.add_child(_shop_list)


## --- API ---

func set_game_data(items: Array) -> void:
	_templates.clear()
	for item in items:
		if item is Dictionary:
			var t: Dictionary = item
			_templates[str(t.get("id", ""))] = t
	_rebuild_shop()


func set_inventory(data: Dictionary) -> void:
	_gold = int(data.get("gold", 0))
	var items_variant: Variant = data.get("items", [])
	_items = items_variant if items_variant is Array else []
	_gold_label.text = "Oro: %d" % _gold
	_refresh()


func set_stats(attack_min: int, attack_max: int, defense: int) -> void:
	_attack_min = attack_min
	_attack_max = attack_max
	_defense = defense
	if _combat_label != null:
		_combat_label.text = "ATK %d-%d · DEF %d" % [_attack_min, _attack_max, _defense]


func open() -> void:
	_root.visible = true
	_is_open = true
	_refresh()


func close() -> void:
	_root.visible = false
	_is_open = false
	_shop_panel.visible = false
	closed.emit()


func toggle() -> void:
	if _is_open:
		close()
	else:
		open()


func is_open() -> bool:
	return _is_open


## --- Refresh ---

func _refresh() -> void:
	for slot_button in _equip_buttons.values():
		var sb: Button = slot_button
		sb.text = "%s\n—" % SLOT_LABELS[_slot_for_button(sb)]
		sb.add_theme_color_override("font_color", Color(0.75, 0.8, 0.9))
		sb.set_meta("item_id", 0)
		sb.tooltip_text = ""

	for slot in _inv_slots:
		slot.inventory_item_id = 0
		slot.text = ""
		slot.tooltip_text = ""
		slot.add_theme_color_override("font_color", Color(0.8, 0.85, 0.95))

	var pending: Array = []
	for item in _items:
		if not (item is Dictionary):
			continue
		var it: Dictionary = item
		var slot: String = str(it.get("slot", ""))
		if slot != "":
			if _equip_buttons.has(slot):
				var button: Button = _equip_buttons[slot]
				button.text = "%s\n%s" % [SLOT_LABELS[slot], _short_item(it)]
				button.add_theme_color_override("font_color", _rarity_color(str(it.get("rarity", "common"))))
				button.set_meta("item_id", int(it.get("id", 0)))
				button.tooltip_text = _tooltip(it)
			continue
		pending.append(it)

	var used := {}
	for it in pending:
		var x := int(it.get("x", -1))
		var y := int(it.get("y", -1))
		var index := -1
		if x >= 0 and x < 8 and y >= 0 and y < 5:
			index = y * 8 + x
		if index < 0 or used.has(index):
			for i in range(_inv_slots.size()):
				if not used.has(i):
					index = i
					break
		if index < 0:
			continue
		used[index] = true
		var slot_button = _inv_slots[index]
		slot_button.inventory_item_id = int(it.get("id", 0))
		slot_button.text = _short_item(it)
		slot_button.tooltip_text = _tooltip(it)
		slot_button.add_theme_color_override("font_color", _rarity_color(str(it.get("rarity", "common"))))

	_update_detail()


func _update_detail() -> void:
	for button in _action_buttons.values():
		button.visible = false

	var item := _find_item(_selected_id)
	_selected_slot = ""
	if item.is_empty():
		_detail_name.text = "Seleccioná un ítem"
		_detail_name.add_theme_color_override("font_color", Color.WHITE)
		_detail_info.text = ""
		return

	var tmpl := _find_template(str(item.get("tmpl", "")))
	_detail_name.text = _item_title(item, tmpl)
	_detail_name.add_theme_color_override("font_color", _rarity_color(str(item.get("rarity", "common"))))
	_detail_info.text = _tooltip(item)

	var equipped := str(item.get("slot", "")) != ""
	_selected_slot = str(item.get("slot", ""))
	var kind := str(tmpl.get("kind", ""))

	if equipped:
		_action_buttons["unequip"].visible = true
	else:
		if kind == "weapon" or kind == "armor":
			_action_buttons["equip"].visible = true
		if kind == "potion":
			_action_buttons["use"].visible = true
		_action_buttons["sell"].visible = true

	if (kind == "weapon" or kind == "armor") and int(item.get("enhance", 0)) < 12:
		_action_buttons["enhance"].visible = true


func _rebuild_shop() -> void:
	for child in _shop_list.get_children():
		child.queue_free()

	var buyables: Array = []
	for id in _templates:
		var t: Dictionary = _templates[id]
		if int(t.get("buy_price", 0)) > 0:
			buyables.append(t)
	buyables.sort_custom(func(a, b): return str(a.get("name", "")) < str(b.get("name", "")))

	for t in buyables:
		var row := HBoxContainer.new()
		var label := Label.new()
		label.text = "%s — %d oro" % [str(t.get("name", "?")), int(t.get("buy_price", 0))]
		label.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		row.add_child(label)

		var button := Button.new()
		button.text = "Comprar"
		button.focus_mode = Control.FOCUS_NONE
		var tmpl_id := str(t.get("id", ""))
		button.pressed.connect(func(): buy_requested.emit(tmpl_id))
		row.add_child(button)
		_shop_list.add_child(row)


## --- Callbacks ---

func _toggle_shop() -> void:
	_shop_panel.visible = not _shop_panel.visible


func _on_inventory_slot_pressed(slot) -> void:
	if slot.inventory_item_id == 0:
		return
	_selected_id = int(slot.inventory_item_id)
	_update_detail()


func _on_equip_slot_pressed(slot: String) -> void:
	var button: Button = _equip_buttons[slot]
	var item_id := int(button.get_meta("item_id", 0))
	if item_id == 0:
		return
	_selected_id = item_id
	_update_detail()


func _on_drop_requested(item_id: int, x: int, y: int) -> void:
	if item_id != 0:
		move_requested.emit(item_id, x, y)


func _on_action_pressed(action: String) -> void:
	if _selected_id == 0:
		return
	match action:
		"equip":
			equip_requested.emit(_selected_id)
		"unequip":
			unequip_requested.emit(_selected_slot)
		"use":
			use_requested.emit(_selected_id)
		"enhance":
			enhance_requested.emit(_selected_id)
		"sell":
			sell_requested.emit(_selected_id)


## --- Helpers ---

func _slot_for_button(button: Button) -> String:
	for slot in _equip_buttons:
		if _equip_buttons[slot] == button:
			return slot
	return ""


func _find_item(item_id: int) -> Dictionary:
	for item in _items:
		if item is Dictionary and int(item.get("id", 0)) == item_id:
			return item
	return {}


func _find_template(tmpl_id: String) -> Dictionary:
	if _templates.has(tmpl_id):
		return _templates[tmpl_id]
	return {}


func _short_item(item: Dictionary) -> String:
	var tmpl := _find_template(str(item.get("tmpl", "")))
	var name := str(tmpl.get("name", str(item.get("tmpl", "?"))))
	if name.length() > 9:
		name = name.substr(0, 9) + "…"
	var enhance := int(item.get("enhance", 0))
	if enhance > 0:
		name += " +%d" % enhance
	var qty := int(item.get("qty", 1))
	if qty > 1:
		name += "\nx%d" % qty
	return name


func _item_title(item: Dictionary, tmpl: Dictionary) -> String:
	var name := str(tmpl.get("name", str(item.get("tmpl", "?"))))
	var enhance := int(item.get("enhance", 0))
	if enhance > 0:
		name += " +%d" % enhance
	var rarity := str(item.get("rarity", "common"))
	return "%s  ·  %s" % [name, RARITY_NAMES.get(rarity, rarity)]


func _tooltip(item: Dictionary) -> String:
	var tmpl := _find_template(str(item.get("tmpl", "")))
	if tmpl.is_empty():
		return str(item.get("tmpl", "?"))
	var lines: Array = []
	var kind := str(tmpl.get("kind", ""))
	if kind == "weapon":
		lines.append("Daño: %d-%d" % [int(tmpl.get("min_damage", 0)), int(tmpl.get("max_damage", 0))])
		lines.append("Rango: %.1f m · Intervalo: %d ms" % [
			float(tmpl.get("range", 0.0)), int(tmpl.get("interval_ms", 0))])
	elif kind == "armor":
		lines.append("Defensa: %d" % int(tmpl.get("defense", 0)))
	elif kind == "potion":
		lines.append("Cura %d%% de %s" % [
			int(tmpl.get("heal_percent", 0)),
			"HP" if str(tmpl.get("heal_kind", "")) == "hp" else "MP",
		])
	elif kind == "core":
		lines.append("Material para mejorar ítems (+N)")

	var reqs: Array = []
	for pair in [["req_str", "STR"], ["req_agi", "AGI"], ["req_vit", "VIT"], ["req_ene", "ENE"]]:
		var need := int(tmpl.get(pair[0], 0))
		if need > 0:
			reqs.append("%s %d" % [pair[1], need])
	if not reqs.is_empty():
		lines.append("Requisitos: " + ", ".join(PackedStringArray(reqs)))
	var class_id := str(tmpl.get("class", ""))
	if class_id != "":
		lines.append("Clase: " + class_id)
	lines.append("Rareza: " + RARITY_NAMES.get(str(item.get("rarity", "common")), "?"))
	var rarity_mult: float = float({
		"good": 1.15, "rare": 1.3, "epic": 1.5, "legendary": 1.8,
	}.get(str(item.get("rarity", "common")), 1.0))
	var sell_value := float(tmpl.get("value", 0)) * 0.3 * float(rarity_mult) * (1.0 + 0.08 * int(item.get("enhance", 0)))
	lines.append("Venta: %d oro c/u" % int(round(sell_value)))
	return "\n".join(PackedStringArray(lines))


func _rarity_color(rarity: String) -> Color:
	return RARITY_COLORS.get(rarity, Color.WHITE)
