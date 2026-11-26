class_name MapView
extends Node3D
## Construye la vista temática de cada mapa (ciudad, campo, mazmorra) y sus
## portales. Geometría simple por ahora: el primer pase de arte real queda
## para una iteración posterior.

var _ground: MeshInstance3D
var _environment: WorldEnvironment
var _sun: DirectionalLight3D
var _maps: Dictionary = {}
var _current_map := ""
var _portals: Array = []


func setup(ground: MeshInstance3D, environment: WorldEnvironment, sun: DirectionalLight3D) -> void:
	_ground = ground
	_environment = environment
	_sun = sun


func set_maps(maps: Array) -> void:
	_maps.clear()
	for entry in maps:
		if entry is Dictionary:
			var m: Dictionary = entry
			_maps[str(m.get("id", ""))] = m


func apply_map(map_id: String) -> void:
	_clear()
	_current_map = map_id
	var info: Dictionary = _maps.get(map_id, {})
	var theme := str(info.get("theme", "field"))
	_build_ground(theme)
	_build_environment(theme)
	match theme:
		"town":
			_build_town()
		"dungeon":
			_build_dungeon()
		_:
			_build_field()

	var portals_variant: Variant = info.get("portals", [])
	if portals_variant is Array:
		_build_portals(portals_variant)


func map_name(map_id: String) -> String:
	var info: Dictionary = _maps.get(map_id, {})
	return str(info.get("name", map_id))


func is_safe(map_id: String) -> bool:
	var info: Dictionary = _maps.get(map_id, {})
	return bool(info.get("safe", false))


func current_portals() -> Array:
	return _portals


func portal_towards(target_map: String) -> Dictionary:
	for portal in _portals:
		if str(portal.get("target", "")) == target_map:
			return portal
	return {}


func portal_near(point: Vector3, radius: float) -> Dictionary:
	var best: Dictionary = {}
	var best_dist := radius
	for portal in _portals:
		var dist := Vector2(float(portal.get("x", 0.0)) - point.x, float(portal.get("z", 0.0)) - point.z).length()
		if dist < best_dist:
			best_dist = dist
			best = portal
	return best


func _clear() -> void:
	_portals.clear()
	for child in get_children():
		remove_child(child)
		child.queue_free()


func _build_ground(theme: String) -> void:
	if _ground == null:
		return
	var color := Color(0.18, 0.26, 0.14)
	match theme:
		"town":
			color = Color(0.30, 0.27, 0.22)
		"dungeon":
			color = Color(0.07, 0.07, 0.09)
	var mat := StandardMaterial3D.new()
	mat.albedo_color = color
	mat.roughness = 1.0
	_ground.material_override = mat


func _build_environment(theme: String) -> void:
	if _environment != null and _environment.environment != null and _sun != null:
		var env := _environment.environment
		match theme:
			"town":
				env.background_color = Color(0.12, 0.11, 0.12, 1)
				env.ambient_light_color = Color(0.75, 0.66, 0.55, 1)
				env.ambient_light_energy = 0.75
				_sun.rotation_degrees = Vector3(-55, 25, 0)
				_sun.light_color = Color(1.0, 0.9, 0.75)
				_sun.light_energy = 1.2
			"dungeon":
				env.background_color = Color(0.02, 0.02, 0.04, 1)
				env.ambient_light_color = Color(0.25, 0.22, 0.35, 1)
				env.ambient_light_energy = 0.45
				_sun.rotation_degrees = Vector3(-65, 10, 0)
				_sun.light_color = Color(0.5, 0.55, 0.8)
				_sun.light_energy = 0.4
			_:
				env.background_color = Color(0.09, 0.1, 0.13, 1)
				env.ambient_light_color = Color(0.65, 0.68, 0.75, 1)
				env.ambient_light_energy = 0.65
				_sun.rotation_degrees = Vector3(-50, 30, 0)
				_sun.light_color = Color(1.0, 0.97, 0.9)
				_sun.light_energy = 1.15


func _build_town() -> void:
	# Edificios alrededor de la plaza.
	var buildings := [
		[Vector3(-13, 2.0, -10), Vector3(6, 4, 6), 0.0],
		[Vector3(13, 2.5, -11), Vector3(5, 5, 6), -0.2],
		[Vector3(-13, 2.0, 10), Vector3(6, 4, 5), 0.15],
		[Vector3(12, 2.0, 9), Vector3(5, 4, 6), -0.1],
		[Vector3(0, 2.5, -13), Vector3(7, 5, 4), 0.0],
	]
	for b in buildings:
		var pos: Vector3 = b[0]
		var size: Vector3 = b[1]
		var rot: float = b[2]
		_box(pos, size, Color(0.42, 0.37, 0.31), rot)
		_box(pos + Vector3(0, size.y / 2 + 0.35, 0), Vector3(size.x + 0.6, 0.7, size.z + 0.6), Color(0.36, 0.2, 0.17), rot)

	# Fuente central.
	_cylinder(Vector3(0, 0.3, 0), 2.2, 0.6, Color(0.38, 0.36, 0.33))
	_cylinder(Vector3(0, 0.65, 0), 1.6, 0.15, Color(0.25, 0.45, 0.7), true)
	_cylinder(Vector3(0, 1.4, 0), 0.35, 2.2, Color(0.5, 0.5, 0.52))

	# Lámparas.
	for pos in [Vector3(-4, 0, -4), Vector3(4, 0, -4), Vector3(-4, 0, 4), Vector3(4, 0, 4)]:
		_box(pos + Vector3(0, 1.4, 0), Vector3(0.25, 2.8, 0.25), Color(0.25, 0.23, 0.2))
		_light(pos + Vector3(0, 2.9, 0), Color(1.0, 0.75, 0.4), 1.3, 7.0)


func _build_field() -> void:
	# Rocas, ruinas y árboles.
	_box(Vector3(-8, 2.5, -6), Vector3(1.6, 5, 1.6), Color(0.3, 0.3, 0.33), 0.35)
	_box(Vector3(6, 1, -4), Vector3(2, 2, 2), Color(0.33, 0.3, 0.26), 0.6)
	_box(Vector3(9, 1.5, 6), Vector3(5, 3, 0.8), Color(0.32, 0.31, 0.3), 0.1)
	_box(Vector3(-10, 1.25, 7), Vector3(3.2, 2.5, 3.2), Color(0.3, 0.32, 0.28), 0.25)
	_box(Vector3(2, 0.6, 9), Vector3(2.8, 1.2, 2), Color(0.31, 0.29, 0.26), 1.2)
	_box(Vector3(-3, 2, -12), Vector3(1.4, 4, 1.4), Color(0.34, 0.32, 0.3), 0.5)

	if _ground != null:
		for pos in [Vector3(-14, 0, -14), Vector3(15, 0, -7), Vector3(-16, 0, 3), Vector3(14, 0, 14)]:
			_cylinder(pos + Vector3(0, 1.4, 0), 0.5, 2.8, Color(0.28, 0.2, 0.13))
			_box(pos + Vector3(0, 3.4, 0), Vector3(3.2, 2.6, 3.2), Color(0.16, 0.36, 0.16), 0.4)


func _build_dungeon() -> void:
	# Muros perimetrales y columnas.
	_box(Vector3(0, 2.5, -19.5), Vector3(40, 5, 1), Color(0.13, 0.13, 0.16))
	_box(Vector3(0, 2.5, 19.5), Vector3(40, 5, 1), Color(0.13, 0.13, 0.16))
	_box(Vector3(-19.5, 2.5, 0), Vector3(1, 5, 40), Color(0.13, 0.13, 0.16))
	_box(Vector3(19.5, 2.5, 0), Vector3(1, 5, 40), Color(0.13, 0.13, 0.16))

	var walls := [
		[Vector3(-8, 2, -6), Vector3(1, 4, 10)],
		[Vector3(8, 2, 5), Vector3(1, 4, 10)],
		[Vector3(-2, 2, 12), Vector3(8, 4, 1)],
		[Vector3(3, 2, -13), Vector3(10, 4, 1)],
	]
	for wall in walls:
		_box(wall[0], wall[1], Color(0.15, 0.15, 0.19))

	for pos in [Vector3(-12, 0, -12), Vector3(12, 0, -12), Vector3(-12, 0, 12), Vector3(12, 0, 12), Vector3(0, 0, 0)]:
		_box(pos + Vector3(0, 3, 0), Vector3(1.4, 6, 1.4), Color(0.18, 0.18, 0.22))

	# Antorchas.
	for pos in [Vector3(-6, 0, -2), Vector3(6, 0, 2), Vector3(0, 0, -10), Vector3(0, 0, 10)]:
		_box(pos + Vector3(0, 2.2, 0), Vector3(0.3, 0.8, 0.3), Color(0.9, 0.45, 0.15), 0, true)
		_light(pos + Vector3(0, 2.6, 0), Color(1.0, 0.5, 0.2), 1.6, 9.0)


func _build_portals(portals: Array) -> void:
	for entry in portals:
		if not (entry is Dictionary):
			continue
		var p: Dictionary = entry
		var x := float(p.get("x", 0.0))
		var z := float(p.get("z", 0.0))
		var target := str(p.get("target", ""))
		var level_req := int(p.get("level_req", 1))
		var portal_name := str(p.get("name", target))
		var target_info: Dictionary = _maps.get(target, {})
		var color := Color(0.3, 0.8, 1.0)
		match str(target_info.get("theme", "")):
			"field":
				color = Color(0.4, 1.0, 0.55)
			"dungeon":
				color = Color(0.8, 0.4, 1.0)

		var mesh := MeshInstance3D.new()
		var cylinder := CylinderMesh.new()
		cylinder.top_radius = 1.0
		cylinder.bottom_radius = 1.0
		cylinder.height = 3.6
		mesh.mesh = cylinder
		var mat := StandardMaterial3D.new()
		mat.transparency = BaseMaterial3D.TRANSPARENCY_ALPHA
		mat.shading_mode = BaseMaterial3D.SHADING_MODE_UNSHADED
		mat.albedo_color = Color(color.r, color.g, color.b, 0.32)
		mat.emission_enabled = true
		mat.emission = color
		mat.emission_energy_multiplier = 1.6
		mesh.material_override = mat
		mesh.position = Vector3(x, 1.8, z)
		add_child(mesh)

		var base := MeshInstance3D.new()
		var ring := CylinderMesh.new()
		ring.top_radius = 1.35
		ring.bottom_radius = 1.35
		ring.height = 0.12
		base.mesh = ring
		var base_mat := StandardMaterial3D.new()
		base_mat.albedo_color = color
		base_mat.emission_enabled = true
		base_mat.emission = color
		base_mat.emission_energy_multiplier = 1.2
		base.material_override = base_mat
		base.position = Vector3(x, 0.06, z)
		add_child(base)

		var label := Label3D.new()
		label.text = portal_name if level_req <= 1 else "%s\n(Nv %d)" % [portal_name, level_req]
		label.position = Vector3(x, 4.4, z)
		label.billboard = BaseMaterial3D.BILLBOARD_ENABLED
		label.no_depth_test = true
		label.font_size = 42
		label.outline_size = 14
		label.modulate = color
		add_child(label)

		_portals.append({
			"id": str(p.get("id", "")),
			"name": portal_name,
			"target": target,
			"level_req": level_req,
			"x": x,
			"z": z,
		})


func _box(pos: Vector3, size: Vector3, color: Color, rot_y: float = 0.0, emissive: bool = false) -> MeshInstance3D:
	var mesh := MeshInstance3D.new()
	var box := BoxMesh.new()
	box.size = size
	mesh.mesh = box
	var mat := StandardMaterial3D.new()
	mat.albedo_color = color
	mat.roughness = 0.95
	if emissive:
		mat.emission_enabled = true
		mat.emission = color
		mat.emission_energy_multiplier = 2.0
	mesh.material_override = mat
	mesh.position = pos
	mesh.rotation.y = rot_y
	add_child(mesh)
	return mesh


func _cylinder(pos: Vector3, radius: float, height: float, color: Color, emissive: bool = false) -> MeshInstance3D:
	var mesh := MeshInstance3D.new()
	var cylinder := CylinderMesh.new()
	cylinder.top_radius = radius
	cylinder.bottom_radius = radius
	cylinder.height = height
	mesh.mesh = cylinder
	var mat := StandardMaterial3D.new()
	mat.albedo_color = color
	mat.roughness = 0.8
	if emissive:
		mat.emission_enabled = true
		mat.emission = color
		mat.emission_energy_multiplier = 1.2
	mesh.material_override = mat
	mesh.position = pos
	add_child(mesh)
	return mesh


func _light(pos: Vector3, color: Color, energy: float, range_m: float) -> void:
	var light := OmniLight3D.new()
	light.position = pos
	light.light_color = color
	light.light_energy = energy
	light.omni_range = range_m
	add_child(light)
