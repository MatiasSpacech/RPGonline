extends Node3D
## F3 — combate: auth/lobby (F2) + mobs, auto-ataque, XP, pociones y muerte.
## El servidor es la autoridad: acá se pide y se renderiza.

const INTERP_DELAY_MS := 100
const MAP_HALF := 19.0
const CAMERA_OFFSET := Vector3(0, 8, 10)
const NetClientScript := preload("res://scripts/net_client.gd")
const AuthUIScript := preload("res://scripts/auth_ui.gd")
const GameHUDScript := preload("res://scripts/hud.gd")
const InventoryUIScript := preload("res://scripts/inventory_ui.gd")
const PlayerEntityScript := preload("res://scripts/player_entity.gd")
const MobEntityScript := preload("res://scripts/mob_entity.gd")
const DropEntityScript := preload("res://scripts/drop_entity.gd")
const MapViewScript := preload("res://scripts/map_view.gd")
const PLAYER_SCENE := preload("res://scenes/player_entity.tscn")
const MOB_SCENE := preload("res://scenes/mob_entity.tscn")
const DROP_SCENE := preload("res://scenes/drop_entity.tscn")

const AUTOTEST_USER := "autotest"
const AUTOTEST_PASS := "secreto123"
const AUTOTEST_CHAR := "AutoWarrior"

@onready var _net: NetClientScript = $NetClient
@onready var _ui: AuthUIScript = $UIOverlay
@onready var _hud: GameHUDScript = $UI
@onready var _inventory: InventoryUIScript = $UIInventory
@onready var _map_view: MapViewScript = $MapView
@onready var _camera: Camera3D = $Camera3D
@onready var _entities_root: Node3D = $Entities
@onready var _marker: MeshInstance3D = $MoveMarker

var _entities: Dictionary = {}
var _mobs: Dictionary = {}
var _drops: Dictionary = {}
var _item_templates: Dictionary = {}
var _skills_by_id: Dictionary = {}
var _class_skill_ids: Array = []
var _self_id := 0
var _self_entity: PlayerEntityScript = null
var _character: Dictionary = {}
var _current_map := ""
var _last_target_id := 0
var _last_ground_point := Vector3.ZERO
var _last_ground_valid := false
var _current_tick := 0
var _last_rtt := 0
var _in_game := false
var _marker_tween: Tween

# Modo autotest headless: `godot --headless --path client -- --autotest`
var _autotest := false
var _autotest_state := 0
var _autotest_attack_sent := false
var _autotest_items_ok := false
var _autotest_equip_sent := false
var _autotest_enhance_sent := false
var _autotest_ready_combat := false
var _autotest_portal_sent := false
var _autotest_done := false


func _ready() -> void:
	for arg in OS.get_cmdline_user_args():
		if arg == "--autotest":
			_autotest = true

	_map_view.setup($Ground, $WorldEnvironment, $Sun)

	_net.connected.connect(_on_net_connected)
	_net.disconnected.connect(_on_net_disconnected)
	_net.login_ok.connect(_on_login_ok)
	_net.char_created.connect(_on_char_created)
	_net.char_deleted.connect(_on_char_deleted)
	_net.welcome_received.connect(_on_welcome)
	_net.snapshot_received.connect(_on_snapshot)
	_net.pong_received.connect(_on_pong)
	_net.server_error.connect(_on_server_error)
	_net.self_state_received.connect(_on_self_state)
	_net.damage_received.connect(_on_damage)
	_net.mob_dead_received.connect(_on_mob_dead)
	_net.death_received.connect(_on_death)
	_net.levelup_received.connect(_on_levelup)
	_net.teleport_received.connect(_on_teleport)
	_net.inventory_received.connect(_on_inventory_received)
	_net.enhance_result_received.connect(_on_enhance_result)
	_net.skill_cast_received.connect(_on_skill_cast)
	_net.skill_effect_received.connect(_on_skill_effect)

	_ui.login_requested.connect(_on_ui_login)
	_ui.register_requested.connect(_on_ui_register)
	_ui.create_requested.connect(_on_ui_create)
	_ui.select_requested.connect(_on_ui_select)
	_ui.delete_requested.connect(_on_ui_delete)

	_hud.potion_requested.connect(_on_hud_potion)
	_hud.alloc_stat_requested.connect(_on_hud_alloc_stat)
	_hud.inventory_requested.connect(_toggle_inventory)
	_hud.skill_requested.connect(_cast_skill)
	_hud.reset()

	_inventory.equip_requested.connect(_net.equip_item)
	_inventory.unequip_requested.connect(_net.unequip_item)
	_inventory.use_requested.connect(_net.use_item)
	_inventory.enhance_requested.connect(_net.enhance_item)
	_inventory.sell_requested.connect(_net.shop_sell)
	_inventory.move_requested.connect(_net.move_item)
	_inventory.buy_requested.connect(func(tmpl: String): _net.shop_buy(tmpl))

	_ui.show_auth("Conectando al servidor...")
	if _autotest:
		_autotest_watchdog()


func _process(_delta: float) -> void:
	var now := Time.get_ticks_msec()
	for entity in _entities.values():
		entity.render_at(now, INTERP_DELAY_MS)
	for mob in _mobs.values():
		mob.render_at(now, INTERP_DELAY_MS)

	if _self_entity != null and is_instance_valid(_self_entity):
		var target := _self_entity.position + CAMERA_OFFSET
		_camera.position = _camera.position.lerp(target, 0.15)


func _unhandled_input(event: InputEvent) -> void:
	if _inventory.is_open():
		if event is InputEventKey and event.pressed and not event.echo:
			if event.physical_keycode == KEY_I or event.physical_keycode == KEY_ESCAPE:
				_inventory.close()
		return

	if not _in_game:
		return

	if event is InputEventKey and event.pressed and not event.echo:
		match event.physical_keycode:
			KEY_I:
				_inventory.open()
				return
			KEY_Q:
				_cast_skill_index(0)
				return
			KEY_W:
				_cast_skill_index(1)
				return
			KEY_E:
				_cast_skill_index(2)
				return
			KEY_R:
				_cast_skill_index(3)
				return
			KEY_1:
				_net.use_potion("hp")
			KEY_2:
				_net.use_potion("mp")

	if event is InputEventMouseButton and event.button_index == MOUSE_BUTTON_LEFT and event.pressed:
		_handle_click(event.position)


func _handle_click(screen_pos: Vector2) -> void:
	var point := _ground_point(screen_pos)
	if point == Vector3.INF:
		return
	_last_ground_point = point
	_last_ground_valid = true

	# Prioridad: portal → drop → mob → caminar.
	var portal := _map_view.portal_near(point, 1.6)
	if not portal.is_empty():
		var portal_point := Vector3(float(portal.get("x", 0.0)), 0.0, float(portal.get("z", 0.0)))
		if _self_entity == null or portal_point.distance_to(_self_entity.position) > 3.0:
			_last_target_id = 0
			_net.send_move(portal_point)
			_show_marker(portal_point)
		else:
			_last_target_id = 0
			_net.use_portal(str(portal.get("id", "")))
			_show_marker(portal_point)
		return

	var drop_id := _drop_near(point, 1.1)
	if drop_id != 0:
		_last_target_id = 0
		var drop: DropEntityScript = _drops[drop_id]
		var drop_point := Vector3(drop.position.x, 0.0, drop.position.z)
		if _self_entity == null or drop_point.distance_to(_self_entity.position) > 3.0:
			_net.send_move(drop_point)
			_show_marker(drop_point)
		else:
			_net.pickup(drop_id)
			_show_marker(drop_point)
		return

	var mob_id := _mob_near(point, 1.8)
	if mob_id != 0:
		_last_target_id = mob_id
		_net.attack(mob_id)
		var mob: MobEntityScript = _mobs[mob_id]
		_show_marker(mob.position)
	else:
		_last_target_id = 0
		_net.send_move(point)
		_show_marker(point)


func _ground_point(screen_pos: Vector2) -> Vector3:
	var origin := _camera.project_ray_origin(screen_pos)
	var dir := _camera.project_ray_normal(screen_pos)
	if absf(dir.y) < 0.0001:
		return Vector3.INF
	var t := -origin.y / dir.y
	if t < 0.0:
		return Vector3.INF
	var point := origin + dir * t
	point.x = clampf(point.x, -MAP_HALF, MAP_HALF)
	point.z = clampf(point.z, -MAP_HALF, MAP_HALF)
	point.y = 0.0
	return point


func _mob_near(point: Vector3, radius: float) -> int:
	var best_id := 0
	var best_dist := radius
	for id in _mobs.keys():
		var mob: MobEntityScript = _mobs[id]
		var dist := Vector2(mob.position.x - point.x, mob.position.z - point.z).length()
		if dist < best_dist:
			best_dist = dist
			best_id = int(id)
	return best_id


func _drop_near(point: Vector3, radius: float) -> int:
	var best_id := 0
	var best_dist := radius
	for id in _drops.keys():
		var drop: DropEntityScript = _drops[id]
		var dist := Vector2(drop.position.x - point.x, drop.position.z - point.z).length()
		if dist < best_dist:
			best_dist = dist
			best_id = int(id)
	return best_id


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
	_hud.reset()
	_inventory.set_inventory({"gold": 0, "items": []})
	if _inventory.is_open():
		_inventory.close()
	_ui.show_auth("Desconectado: %s\nReintentando automáticamente..." % reason)


func _on_login_ok(data: Dictionary) -> void:
	_in_game = false
	clear_entities()
	_hud.reset()
	_inventory.set_inventory({"gold": 0, "items": []})
	if _inventory.is_open():
		_inventory.close()

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
	_hud.reset()
	if _inventory.is_open():
		_inventory.close()

	var game_data_variant: Variant = data.get("game_data", {})
	if game_data_variant is Dictionary:
		var game_data: Dictionary = game_data_variant

		var items_variant: Variant = game_data.get("items", [])
		if items_variant is Array:
			var items: Array = items_variant
			_item_templates.clear()
			for entry in items:
				if entry is Dictionary:
					var t: Dictionary = entry
					_item_templates[str(t.get("id", ""))] = t
			_inventory.set_game_data(items)

		var maps_variant: Variant = game_data.get("maps", [])
		if maps_variant is Array:
			_map_view.set_maps(maps_variant)

	var char_variant: Variant = data.get("character", {})
	_character = char_variant if char_variant is Dictionary else {}

	var character_name := str(_character.get("name", "?"))
	var class_id := str(_character.get("class", ""))
	_last_target_id = 0

	_skills_by_id.clear()
	_class_skill_ids.clear()
	var class_skills: Array = []
	if game_data_variant is Dictionary:
		var game_data: Dictionary = game_data_variant
		var skills_variant: Variant = game_data.get("skills", [])
		if skills_variant is Array:
			for entry in skills_variant:
				if not (entry is Dictionary):
					continue
				var s: Dictionary = entry
				if str(s.get("class", "")) != class_id:
					continue
				var skill_id := str(s.get("id", ""))
				_skills_by_id[skill_id] = s
				_class_skill_ids.append(skill_id)
				class_skills.append(s)
	_hud.set_skills(class_skills)

	_current_map = str(data.get("map", ""))
	_last_ground_valid = false
	_map_view.apply_map(_current_map)
	_update_status()

	var self_variant: Variant = data.get("self", {})
	if self_variant is Dictionary:
		var self_data: Dictionary = self_variant
		_self_id = int(self_data.get("id", 0))
		print("[game] posición al entrar: (%.2f, %.2f)" % [
			float(self_data.get("x", 0.0)),
			float(self_data.get("z", 0.0)),
		])

	print("[game] en el mundo como %s (id %d) en mapa %s" % [
		character_name, int(_character.get("id", 0)), _current_map,
	])

	if _autotest:
		_autotest_state = 5
		_autotest_attack_sent = false
		_autotest_items_ok = false
		_autotest_equip_sent = false
		_autotest_enhance_sent = false
		_autotest_ready_combat = false
		_autotest_portal_sent = false


func _on_snapshot(data: Dictionary) -> void:
	if not _in_game:
		return

	var now := Time.get_ticks_msec()
	_current_tick = int(data.get("tick", 0))

	var ents_variant: Variant = data.get("ents", [])
	if not (ents_variant is Array):
		return
	var ents: Array = ents_variant

	var seen_pcs := {}
	var seen_mobs := {}
	var seen_drops := {}

	for entry in ents:
		if not (entry is Dictionary):
			continue
		var e: Dictionary = entry
		var id := int(e.get("id", 0))
		if id == 0:
			continue

		var pos := Vector3(float(e.get("x", 0.0)), 0.0, float(e.get("z", 0.0)))
		var kind := str(e.get("k", "pc"))

		if kind == "drop":
			seen_drops[id] = true
			var drop: DropEntityScript = _drops.get(id) as DropEntityScript
			if drop == null:
				drop = DROP_SCENE.instantiate() as DropEntityScript
				if drop == null:
					continue
				_entities_root.add_child(drop)
				drop.setup(
					id,
					str(e.get("tmpl", "")),
					str(e.get("name", "Ítem")),
					str(e.get("rarity", "common"))
				)
				_drops[id] = drop
			drop.position = pos
			continue

		if kind == "mob":
			seen_mobs[id] = true
			var mob: MobEntityScript = _mobs.get(id) as MobEntityScript
			if mob == null:
				mob = MOB_SCENE.instantiate() as MobEntityScript
				if mob == null:
					continue
				_entities_root.add_child(mob)
				mob.setup(
					id,
					str(e.get("name", "Mob")),
					int(e.get("level", 1)),
					str(e.get("tmpl", "")),
					int(e.get("hp", 1)),
					int(e.get("hp_max", 1))
				)
				_mobs[id] = mob
			mob.push_state(
				pos, now,
				str(e.get("anim", "idle")),
				int(e.get("hp", 1)),
				int(e.get("hp_max", 1))
			)
		else:
			seen_pcs[id] = true
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
			node.push_state(pos, now, str(e.get("anim", "idle")))

	_remove_missing(_entities, seen_pcs, true)
	_remove_missing(_mobs, seen_mobs, false)
	_remove_missing(_drops, seen_drops, false)
	_update_info()

	if _autotest and _autotest_items_ok and _autotest_ready_combat and not _autotest_attack_sent:
		var target := _nearest_mob_for_autotest()
		if target != 0:
			_autotest_attack_sent = true
			_autotest_state = 7
			print("[autotest] atacando mob %d" % target)
			_net.attack(target)

	if _last_target_id != 0 and not _mobs.has(_last_target_id):
		_last_target_id = 0


func _remove_missing(collection: Dictionary, seen: Dictionary, is_player: bool) -> void:
	for id in collection.keys():
		if seen.has(id):
			continue
		if is_player:
			var dead_player: PlayerEntityScript = collection[id] as PlayerEntityScript
			if dead_player == _self_entity:
				_self_entity = null
		var node: Node3D = collection[id] as Node3D
		if node != null:
			node.queue_free()
		collection.erase(id)


func _nearest_mob_for_autotest() -> int:
	var origin := Vector3.ZERO
	if _self_entity != null:
		origin = _self_entity.position

	# Preferir babosas (pasivas) para que el smoke test sea seguro.
	for preferred_passive in [true, false]:
		var best_id := 0
		var best_dist := INF
		for id in _mobs.keys():
			var mob: MobEntityScript = _mobs[id]
			if preferred_passive and mob.template_id != "babosa":
				continue
			var d := mob.position.distance_to(origin)
			if d < best_dist:
				best_dist = d
				best_id = int(id)
		if best_id != 0:
			return best_id
	return 0


func _on_self_state(data: Dictionary) -> void:
	_hud.update_self_state(data)
	_inventory.set_stats(
		int(data.get("attack_min", 0)),
		int(data.get("attack_max", 0)),
		int(data.get("defense", 0))
	)
	_character["level"] = int(data.get("level", 1))
	_update_status()


func _on_inventory_received(data: Dictionary) -> void:
	_inventory.set_inventory(data)
	var items_variant: Variant = data.get("items", [])
	var count := 0
	if items_variant is Array:
		count = (items_variant as Array).size()
	print("[game] inventario: %d ítems · %d oro" % [count, int(data.get("gold", 0))])
	if _autotest:
		_autotest_handle_inventory(data)
		if _autotest_items_ok:
			_autotest_go_to_field()


func _on_enhance_result(data: Dictionary) -> void:
	var success := bool(data.get("success", false))
	var destroyed := bool(data.get("destroyed", false))
	if destroyed:
		_hud.toast("¡El ítem se destruyó!", Color(1.0, 0.35, 0.3))
	elif success:
		_hud.toast("¡Mejora exitosa! +%d" % int(data.get("new_enhance", 0)), Color(1.0, 0.85, 0.35))
	else:
		_hud.toast("Falló la mejora", Color(0.75, 0.78, 0.85))

	if _autotest:
		_autotest_items_ok = true
		_autotest_state = 6
		print("[autotest] enhance: success=%s destroyed=%s" % [success, destroyed])


func _on_skill_cast(data: Dictionary) -> void:
	_hud.start_skill_cooldown(
		str(data.get("skill_id", "")),
		int(data.get("cooldown_ms", 0))
	)


func _on_skill_effect(data: Dictionary) -> void:
	var skill_id := str(data.get("skill_id", ""))
	var kind := str(data.get("kind", ""))
	var pos := Vector3(float(data.get("x", 0.0)), 0.6, float(data.get("z", 0.0)))
	var radius := maxf(0.8, float(data.get("radius", 1.2)))
	var color := Color(0.6, 0.8, 1.0)
	match kind:
		"aoe":
			color = Color(1.0, 0.55, 0.2)
		"heal":
			color = Color(0.4, 1.0, 0.5)
		"buff":
			color = Color(1.0, 0.85, 0.35)
		"teleport":
			color = Color(0.75, 0.45, 1.0)
		_:
			color = Color(0.55, 0.8, 1.0)

	var mesh := MeshInstance3D.new()
	var sphere := SphereMesh.new()
	sphere.radius = 1.0
	sphere.height = 2.0
	mesh.mesh = sphere
	var mat := StandardMaterial3D.new()
	mat.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
	mat.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
	mat.albedo_color = Color(color.r, color.g, color.b, 0.45)
	mat.emission_enabled = true
	mat.emission = color
	mat.emission_energy_multiplier = 2.0
	mesh.material_override = mat
	mesh.position = pos
	mesh.scale = Vector3.ONE * 0.35
	add_child(mesh)

	var tween := create_tween().set_parallel(true)
	tween.tween_property(mesh, "scale", Vector3.ONE * radius * 2.0, 0.35)
	tween.tween_property(mesh, "transparency", 1.0, 0.35)
	tween.chain().tween_callback(mesh.queue_free)

	if kind == "buff" or kind == "heal":
		var def: Dictionary = _skills_by_id.get(skill_id, {})
		_hud.toast(str(def.get("name", "Skill")), color)


func _cast_skill(skill_id: String) -> void:
	if not _in_game or skill_id.is_empty() or _inventory.is_open():
		return
	var def: Dictionary = _skills_by_id.get(skill_id, {})
	var x := 0.0
	var z := 0.0
	if str(def.get("kind", "")) == "teleport":
		if _last_ground_valid:
			x = _last_ground_point.x
			z = _last_ground_point.z
		else:
			var point := _ground_point(get_viewport().get_mouse_position())
			if point != Vector3.INF:
				x = point.x
				z = point.z
	_net.use_skill(skill_id, _last_target_id, x, z)


func _cast_skill_index(index: int) -> void:
	if index >= 0 and index < _class_skill_ids.size():
		_cast_skill(str(_class_skill_ids[index]))


func _update_status() -> void:
	if not _in_game:
		return
	var character_name := str(_character.get("name", "?"))
	var class_id := str(_character.get("class", ""))
	var level := int(_character.get("level", 1))
	var map_name := _map_view.map_name(_current_map)
	var safe_suffix := "  ·  Zona segura" if _map_view.is_safe(_current_map) else ""
	_hud.set_status("%s · %s · Nv %d  ·  %s%s" % [
		character_name, _ui.class_label(class_id), level, map_name, safe_suffix,
	])


func _toggle_inventory() -> void:
	if not _in_game:
		return
	_inventory.toggle()


func _on_damage(data: Dictionary) -> void:
	var dst := int(data.get("dst", 0))
	var node: Node3D = _entities.get(dst) as Node3D
	if node == null:
		node = _mobs.get(dst) as Node3D
	if node == null:
		return
	var screen := _camera.unproject_position(node.position + Vector3(0, 2.0, 0))
	_hud.float_damage(
		screen,
		int(data.get("amount", 0)),
		bool(data.get("crit", false)),
		str(data.get("kind", "out"))
	)


func _on_mob_dead(data: Dictionary) -> void:
	var mob_id := int(data.get("id", 0))
	var xp := int(data.get("xp", 0))
	var gold := int(data.get("gold", 0))
	var node: MobEntityScript = _mobs.get(mob_id) as MobEntityScript
	if node != null:
		var screen := _camera.unproject_position(node.position + Vector3(0, 2.2, 0))
		var text := "+%d XP" % xp
		if gold > 0:
			text += "\n+%d oro" % gold
		_hud.float_text(screen, text, Color(0.55, 1.0, 0.6))

	if _autotest and not _autotest_done:
		_autotest_done = true
		print("[autotest] combate OK: +%d XP" % xp)
		_on_skill_effect({
			"skill_id": "", "kind": "aoe",
			"x": 0.0, "z": 0.0, "radius": 2.0,
		})
		await get_tree().create_timer(0.4).timeout
		get_tree().quit(0)


func _on_death(data: Dictionary) -> void:
	var lost := int(data.get("xp_lost", 0))
	_hud.toast("Moriste · -%d XP" % lost, Color(1.0, 0.45, 0.4))
	if _inventory.is_open():
		_inventory.close()


func _on_levelup(data: Dictionary) -> void:
	var level := int(data.get("level", 1))
	_hud.toast("¡Subiste a nivel %d! +5 puntos" % level, Color(1.0, 0.85, 0.35))


func _on_teleport(data: Dictionary) -> void:
	var pos := Vector3(float(data.get("x", 0.0)), 0.0, float(data.get("z", 0.0)))
	var map_id := str(data.get("map", ""))
	if not map_id.is_empty():
		_current_map = map_id
		_map_view.apply_map(map_id)
	_clear_map_entities()
	if _self_entity != null and is_instance_valid(_self_entity):
		_self_entity.snap_to(pos)
	_camera.position = pos + CAMERA_OFFSET
	_marker.visible = false
	if _inventory.is_open():
		_inventory.close()
	_last_target_id = 0
	_last_ground_valid = false
	_update_status()

	if _autotest and not _map_view.is_safe(_current_map):
		_autotest_ready_combat = true
		print("[autotest] en zona de combate: %s" % _current_map)


func _on_pong(rtt_ms: int) -> void:
	_last_rtt = rtt_ms
	if _in_game:
		_update_info()


func _on_server_error(code: String, message: String) -> void:
	_ui.set_error(message)
	if _autotest and code == "auth_failed" and _autotest_state == 1:
		_autotest_state = 2
		_net.request_register(AUTOTEST_USER, AUTOTEST_PASS)


func _update_info() -> void:
	_hud.set_info("tick %d  ·  jugadores: %d  ·  mobs: %d  ·  drops: %d  ·  RTT: %d ms" % [
		_current_tick,
		_entities.size(),
		_mobs.size(),
		_drops.size(),
		_last_rtt,
	])


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


func _on_hud_potion(kind: String) -> void:
	_net.use_potion(kind)


func _on_hud_alloc_stat(stat: String) -> void:
	_net.alloc_stat(stat)


## --- Utilidades ---

func clear_entities() -> void:
	for entity in _entities.values():
		if entity != null:
			entity.queue_free()
	_entities.clear()
	for mob in _mobs.values():
		if mob != null:
			mob.queue_free()
	_mobs.clear()
	for drop in _drops.values():
		if drop != null:
			drop.queue_free()
	_drops.clear()
	_self_entity = null
	_self_id = 0
	_last_target_id = 0


func _clear_map_entities() -> void:
	for id in _entities.keys():
		var entity: PlayerEntityScript = _entities[id] as PlayerEntityScript
		if entity == _self_entity:
			continue
		if entity != null:
			entity.queue_free()
		_entities.erase(id)
	for mob in _mobs.values():
		if mob != null:
			mob.queue_free()
	_mobs.clear()
	for drop in _drops.values():
		if drop != null:
			drop.queue_free()
	_drops.clear()
	_last_target_id = 0


## --- Autotest headless ---

func _autotest_handle_inventory(data: Dictionary) -> void:
	if not _autotest or _autotest_items_ok:
		return

	var items: Array = []
	var items_variant: Variant = data.get("items", [])
	if items_variant is Array:
		items = items_variant

	var weapon: Dictionary = {}
	var has_core := false
	for entry in items:
		if not (entry is Dictionary):
			continue
		var it: Dictionary = entry
		var tmpl: Dictionary = _item_templates.get(str(it.get("tmpl", "")), {})
		if str(tmpl.get("kind", "")) == "core":
			has_core = true
		if str(it.get("slot", "")) == "weapon":
			weapon = it
		elif weapon.is_empty() and str(tmpl.get("kind", "")) == "weapon":
			weapon = it

	if not _autotest_equip_sent and not weapon.is_empty() and str(weapon.get("slot", "")) == "":
		_autotest_equip_sent = true
		print("[autotest] equipando arma %d" % int(weapon.get("id", 0)))
		_net.equip_item(int(weapon.get("id", 0)))
		return

	var gold := int(data.get("gold", 0))
	if _autotest_equip_sent and not _autotest_enhance_sent and has_core and not weapon.is_empty() and gold >= 50:
		_autotest_enhance_sent = true
		print("[autotest] mejorando arma %d" % int(weapon.get("id", 0)))
		_net.enhance_item(int(weapon.get("id", 0)))
		return

	_autotest_items_ok = true
	_autotest_state = 6
	if weapon.is_empty():
		print("[autotest] sin kit inicial; sigo a combate")
	else:
		print("[autotest] items OK")


func _autotest_go_to_field() -> void:
	if _autotest_ready_combat or _autotest_portal_sent:
		return
	if not _map_view.is_safe(_current_map):
		_autotest_ready_combat = true
		print("[autotest] ya en zona de combate: %s" % _current_map)
		return

	var portal := _map_view.portal_towards("llanuras")
	if portal.is_empty():
		_autotest_ready_combat = true
		print("[autotest] sin portal al campo; sigo")
		return

	_autotest_portal_sent = true
	var target := Vector3(float(portal.get("x", 0.0)), 0.0, float(portal.get("z", 0.0)))
	print("[autotest] caminando al portal %s" % str(portal.get("id", "")))
	_net.send_move(target)
	await get_tree().create_timer(2.0).timeout
	_net.use_portal(str(portal.get("id", "")))


func _autotest_watchdog() -> void:
	await get_tree().create_timer(45.0).timeout
	if not _autotest_done:
		print("[autotest] TIMEOUT")
		get_tree().quit(1)
