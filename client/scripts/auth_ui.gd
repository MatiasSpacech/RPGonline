extends CanvasLayer
## UI de autenticación y lobby construida por código. Emite señales que
## main.gd traduce a mensajes del NetClient.

signal login_requested(user: String, password: String)
signal register_requested(user: String, password: String)
signal create_requested(character_name: String, class_id: String)
signal select_requested(character_id: int)
signal delete_requested(character_id: int)

var _root: Control
var _auth_panel: PanelContainer
var _lobby_panel: PanelContainer

var _auth_status: Label
var _auth_error: Label
var _username: LineEdit
var _password: LineEdit
var _login_button: Button
var _register_button: Button

var _lobby_session: Label
var _lobby_status: Label
var _lobby_error: Label
var _chars_list: ItemList
var _select_button: Button
var _delete_button: Button
var _char_name: LineEdit
var _class_option: OptionButton
var _create_button: Button


func _ready() -> void:
	_build()


func _build() -> void:
	_root = Control.new()
	_root.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	add_child(_root)

	var bg := ColorRect.new()
	bg.color = Color(0.03, 0.04, 0.06, 0.94)
	bg.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_root.add_child(bg)

	var center := CenterContainer.new()
	center.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	_root.add_child(center)

	_auth_panel = _build_auth_panel()
	_lobby_panel = _build_lobby_panel()
	_lobby_panel.visible = false
	center.add_child(_auth_panel)
	center.add_child(_lobby_panel)


func _build_auth_panel() -> PanelContainer:
	var panel := PanelContainer.new()
	panel.custom_minimum_size = Vector2(460, 0)
	var box := _content(panel)

	var title := Label.new()
	title.text = "RPGonline"
	title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	title.add_theme_font_size_override("font_size", 34)
	box.add_child(title)

	var subtitle := Label.new()
	subtitle.text = "Cuentas y personajes · F2"
	subtitle.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	subtitle.add_theme_color_override("font_color", Color(0.6, 0.68, 0.85))
	box.add_child(subtitle)

	_auth_status = Label.new()
	_auth_status.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_auth_status.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_auth_status.add_theme_color_override("font_color", Color(0.55, 0.75, 0.6))
	_auth_status.text = "Conectando al servidor..."
	box.add_child(_auth_status)

	box.add_child(HSeparator.new())

	var user_label := Label.new()
	user_label.text = "Usuario"
	box.add_child(user_label)

	_username = LineEdit.new()
	_username.placeholder_text = "3-16 caracteres"
	_username.max_length = 16
	_username.custom_minimum_size = Vector2(0, 38)
	box.add_child(_username)

	var pass_label := Label.new()
	pass_label.text = "Contraseña"
	box.add_child(pass_label)

	_password = LineEdit.new()
	_password.secret = true
	_password.placeholder_text = "Mínimo 6 caracteres"
	_password.max_length = 72
	_password.custom_minimum_size = Vector2(0, 38)
	_password.text_submitted.connect(_on_password_submitted)
	box.add_child(_password)

	var buttons := HBoxContainer.new()
	buttons.add_theme_constant_override("separation", 10)
	box.add_child(buttons)

	_login_button = Button.new()
	_login_button.text = "Entrar"
	_login_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_login_button.pressed.connect(_on_login_pressed)
	buttons.add_child(_login_button)

	_register_button = Button.new()
	_register_button.text = "Crear cuenta"
	_register_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_register_button.pressed.connect(_on_register_pressed)
	buttons.add_child(_register_button)

	_auth_error = _make_error_label()
	box.add_child(_auth_error)

	var hint := Label.new()
	hint.text = "Usuario: letras, números, _ o -.\nContraseña: 6-72 caracteres."
	hint.add_theme_font_size_override("font_size", 12)
	hint.add_theme_color_override("font_color", Color(0.5, 0.55, 0.65))
	box.add_child(hint)

	return panel


func _build_lobby_panel() -> PanelContainer:
	var panel := PanelContainer.new()
	panel.custom_minimum_size = Vector2(520, 0)
	var box := _content(panel)

	var title := Label.new()
	title.text = "Tus personajes"
	title.add_theme_font_size_override("font_size", 28)
	box.add_child(title)

	_lobby_session = Label.new()
	_lobby_session.add_theme_color_override("font_color", Color(0.6, 0.68, 0.85))
	box.add_child(_lobby_session)

	_lobby_status = Label.new()
	_lobby_status.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_lobby_status.add_theme_color_override("font_color", Color(0.55, 0.75, 0.6))
	box.add_child(_lobby_status)

	_chars_list = ItemList.new()
	_chars_list.custom_minimum_size = Vector2(0, 140)
	_chars_list.item_selected.connect(_on_char_selected)
	_chars_list.item_activated.connect(_on_char_activated)
	box.add_child(_chars_list)

	var actions := HBoxContainer.new()
	actions.add_theme_constant_override("separation", 10)
	box.add_child(actions)

	_select_button = Button.new()
	_select_button.text = "Entrar al mundo"
	_select_button.disabled = true
	_select_button.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_select_button.pressed.connect(_on_select_pressed)
	actions.add_child(_select_button)

	_delete_button = Button.new()
	_delete_button.text = "Borrar"
	_delete_button.disabled = true
	_delete_button.pressed.connect(_on_delete_pressed)
	actions.add_child(_delete_button)

	box.add_child(HSeparator.new())

	var create_title := Label.new()
	create_title.text = "Crear personaje"
	create_title.add_theme_font_size_override("font_size", 18)
	box.add_child(create_title)

	_char_name = LineEdit.new()
	_char_name.placeholder_text = "Nombre (3-16 letras)"
	_char_name.max_length = 16
	_char_name.custom_minimum_size = Vector2(0, 36)
	box.add_child(_char_name)

	_class_option = OptionButton.new()
	_class_option.add_item("Guerrero")
	_class_option.set_item_metadata(0, "warrior")
	_class_option.add_item("Mago")
	_class_option.set_item_metadata(1, "mage")
	_class_option.add_item("Arquera")
	_class_option.set_item_metadata(2, "archer")
	box.add_child(_class_option)

	_create_button = Button.new()
	_create_button.text = "Crear personaje"
	_create_button.pressed.connect(_on_create_pressed)
	box.add_child(_create_button)

	_lobby_error = _make_error_label()
	box.add_child(_lobby_error)

	var hint := Label.new()
	hint.text = "Máximo 3 personajes por cuenta."
	hint.add_theme_font_size_override("font_size", 12)
	hint.add_theme_color_override("font_color", Color(0.5, 0.55, 0.65))
	box.add_child(hint)

	return panel


func _content(panel: PanelContainer) -> VBoxContainer:
	var margin := MarginContainer.new()
	margin.add_theme_constant_override("margin_left", 28)
	margin.add_theme_constant_override("margin_right", 28)
	margin.add_theme_constant_override("margin_top", 22)
	margin.add_theme_constant_override("margin_bottom", 22)
	panel.add_child(margin)

	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 9)
	margin.add_child(box)
	return box


func _make_error_label() -> Label:
	var label := Label.new()
	label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	label.add_theme_color_override("font_color", Color(1.0, 0.45, 0.45))
	return label


## --- API que usa main.gd ---

func show_auth(message: String = "") -> void:
	_root.visible = true
	_auth_panel.visible = true
	_lobby_panel.visible = false
	_auth_error.text = ""
	if not message.is_empty():
		_auth_status.text = message
	_username.grab_focus()


func show_lobby(user: String, chars: Array) -> void:
	_root.visible = true
	_auth_panel.visible = false
	_lobby_panel.visible = true
	_lobby_error.text = ""
	_lobby_status.text = ""
	_lobby_session.text = "Sesión: %s" % user
	_password.text = ""
	_rebuild_chars(chars)
	_char_name.grab_focus()


func show_game() -> void:
	_root.visible = false


func set_error(message: String) -> void:
	if _auth_panel.visible:
		_auth_status.text = ""
		_auth_error.text = message
	else:
		_lobby_error.text = message
	push_warning("[ui] %s" % message)


func set_status(message: String) -> void:
	if _auth_panel.visible:
		_auth_status.text = message
	else:
		_lobby_status.text = message


func add_character(char: Dictionary) -> void:
	var index := _chars_list.add_item(_char_line(char))
	_chars_list.set_item_metadata(index, int(char.get("id", 0)))
	_lobby_status.text = "Personaje creado."
	_update_buttons()


func remove_character(char_id: int) -> void:
	for i in range(_chars_list.item_count):
		if int(_chars_list.get_item_metadata(i)) == char_id:
			_chars_list.remove_item(i)
			break
	_lobby_status.text = "Personaje borrado."
	_update_buttons()


func class_label(class_id: String) -> String:
	match class_id:
		"warrior":
			return "Guerrero"
		"mage":
			return "Mago"
		"archer":
			return "Arquera"
		_:
			return class_id


## --- Interno ---

func _rebuild_chars(chars: Array) -> void:
	_chars_list.clear()
	for char in chars:
		if char is Dictionary:
			var c: Dictionary = char
			var index := _chars_list.add_item(_char_line(c))
			_chars_list.set_item_metadata(index, int(c.get("id", 0)))
	_update_buttons()


func _char_line(char: Dictionary) -> String:
	return "%s — %s · Nv %d" % [
		str(char.get("name", "?")),
		class_label(str(char.get("class", ""))),
		int(char.get("level", 1)),
	]


func _update_buttons() -> void:
	var has_selection := _chars_list.get_selected_items().size() > 0
	_select_button.disabled = not has_selection
	_delete_button.disabled = not has_selection
	_create_button.disabled = _chars_list.item_count >= 3


func _selected_character_id() -> int:
	var selected := _chars_list.get_selected_items()
	if selected.is_empty():
		return 0
	return int(_chars_list.get_item_metadata(selected[0]))


func _on_password_submitted(_text: String) -> void:
	_on_login_pressed()


func _on_login_pressed() -> void:
	_auth_error.text = ""
	_auth_status.text = "Verificando..."
	login_requested.emit(_username.text.strip_edges(), _password.text)


func _on_register_pressed() -> void:
	_auth_error.text = ""
	_auth_status.text = "Creando cuenta..."
	register_requested.emit(_username.text.strip_edges(), _password.text)


func _on_char_selected(_index: int) -> void:
	_update_buttons()


func _on_char_activated(_index: int) -> void:
	_on_select_pressed()


func _on_select_pressed() -> void:
	var id := _selected_character_id()
	if id == 0:
		return
	_lobby_error.text = ""
	_lobby_status.text = "Entrando al mundo..."
	select_requested.emit(id)


func _on_delete_pressed() -> void:
	var id := _selected_character_id()
	if id == 0:
		return
	_lobby_error.text = ""
	_lobby_status.text = "Borrando..."
	delete_requested.emit(id)


func _on_create_pressed() -> void:
	var character_name := _char_name.text.strip_edges()
	if character_name.is_empty():
		_lobby_error.text = "Escribí un nombre para el personaje."
		return
	var class_id := str(_class_option.get_item_metadata(_class_option.selected))
	_lobby_error.text = ""
	_lobby_status.text = "Creando personaje..."
	create_requested.emit(character_name, class_id)
