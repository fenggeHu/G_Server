extends RefCounted
## GameServer 侧的最小地图权威数据。只保存可验证规则，不加载客户端视觉资源。

var map_id := ""
var authority_version := ""
var min_x := 0.0
var max_x := 0.0
var min_z := 0.0
var max_z := 0.0
var flight_enabled := false
var max_flight_height := 0.0
var min_ground_y := -INF
var blockers: Array = []
var _enemy_placements: Array = []
var _ground_resolution := 0
var _ground_size := 0.0
var _ground_heights := PackedFloat32Array()


static func load_json(path: String, expected_map_id: String = "", expected_version: String = ""):
	if path.is_empty() or not FileAccess.file_exists(path):
		return null
	var file := FileAccess.open(path, FileAccess.READ)
	if file == null:
		return null
	var parser := JSON.new()
	var result := parser.parse(file.get_as_text())
	file.close()
	if result != OK or not parser.data is Dictionary:
		return null
	var authority = load("res://Server/map_authority.gd").new()
	if not authority.configure(parser.data):
		return null
	if not expected_map_id.is_empty() and authority.map_id != expected_map_id:
		return null
	if not expected_version.is_empty() and authority.authority_version != expected_version:
		return null
	return authority


func configure(data: Dictionary) -> bool:
	map_id = String(data.get("map_id", ""))
	authority_version = String(data.get("authority_version", ""))
	var boundary = data.get("boundary", {})
	var flight = data.get("flight", {})
	if not boundary is Dictionary or not flight is Dictionary:
		return false
	min_x = float(boundary.get("min_x", INF))
	max_x = float(boundary.get("max_x", -INF))
	min_z = float(boundary.get("min_z", INF))
	max_z = float(boundary.get("max_z", -INF))
	flight_enabled = bool(flight.get("enabled", false))
	max_flight_height = float(flight.get("max_height", -1.0))
	var ground = data.get("ground", {})
	min_ground_y = float(ground.get("min_y", -INF)) if ground is Dictionary else -INF
	if ground is Dictionary:
		_load_ground(String(ground.get("grid_path", "")), String(ground.get("sha256", "")))
	blockers.clear()
	var raw_blockers = data.get("blockers", [])
	if raw_blockers is Array:
		for entry in raw_blockers:
			if not entry is Dictionary:
				continue
			var blocker := {
				"min_x": float(entry.get("min_x", INF)), "max_x": float(entry.get("max_x", -INF)),
				"min_z": float(entry.get("min_z", INF)), "max_z": float(entry.get("max_z", -INF)),
				"min_y": float(entry.get("min_y", INF)), "max_y": float(entry.get("max_y", -INF)),
			}
			if blocker.min_x >= blocker.max_x or blocker.min_z >= blocker.max_z or blocker.min_y >= blocker.max_y:
				continue
			blockers.append(blocker)
	_parse_enemies(data)
	return is_valid()


func _parse_enemies(data: Dictionary) -> void:
	_enemy_placements.clear()
	var raw = data.get("enemies", [])
	if not raw is Array:
		return
	var seen := {}
	for entry in raw:
		if not entry is Dictionary:
			continue
		var enemy_id := String(entry.get("id", ""))
		var enemy_type := String(entry.get("type", ""))
		if enemy_id.is_empty() or enemy_type.is_empty() or seen.has(enemy_id):
			continue
		var x := float(entry.get("x", 0.0))
		var z := float(entry.get("z", 0.0))
		if not is_finite(x) or not is_finite(z):
			continue
		seen[enemy_id] = true
		_enemy_placements.append({"id": enemy_id, "type": enemy_type, "x": x, "z": z})


func enemies() -> Array:
	return _enemy_placements


func has_enemy(enemy_id: String) -> bool:
	for entry in _enemy_placements:
		if entry.id == enemy_id:
			return true
	return false


func enemy_type(enemy_id: String) -> String:
	for entry in _enemy_placements:
		if entry.id == enemy_id:
			return entry.type
	return ""


func enemy_position(enemy_id: String) -> Vector3:
	for entry in _enemy_placements:
		if entry.id == enemy_id:
			return Vector3(entry.x, ground_height(entry.x, entry.z), entry.z)
	return Vector3.ZERO


func has_ground() -> bool:
	return _ground_resolution >= 2 and _ground_heights.size() == _ground_resolution * _ground_resolution


func ground_height(x: float, z: float) -> float:
	if not has_ground():
		return min_ground_y
	var n := _ground_resolution
	var fx := clampf((x / _ground_size + 0.5) * float(n - 1), 0.0, float(n - 1))
	var fz := clampf((z / _ground_size + 0.5) * float(n - 1), 0.0, float(n - 1))
	var x0 := int(fx)
	var z0 := int(fz)
	var x1 := mini(x0 + 1, n - 1)
	var z1 := mini(z0 + 1, n - 1)
	var tx := fx - float(x0)
	var tz := fz - float(z0)
	var top := lerpf(_ground_heights[z0 * n + x0], _ground_heights[z0 * n + x1], tx)
	var bottom := lerpf(_ground_heights[z1 * n + x0], _ground_heights[z1 * n + x1], tx)
	return lerpf(top, bottom, tz)


func _load_ground(grid_path: String, expected_sha256: String) -> void:
	_ground_resolution = 0
	_ground_size = 0.0
	_ground_heights = PackedFloat32Array()
	if grid_path.is_empty() or not FileAccess.file_exists(grid_path):
		return
	if _file_sha256(grid_path) != expected_sha256:
		return
	var file := FileAccess.open(grid_path, FileAccess.READ)
	if file == null:
		return
	var parser := JSON.new()
	var parsed := parser.parse(file.get_as_text())
	file.close()
	if parsed != OK or not parser.data is Dictionary:
		return
	var data: Dictionary = parser.data
	var heights = data.get("heights", [])
	if not heights is Array or heights.size() < 4:
		return
	var resolution := int(data.get("resolution", 0))
	if resolution < 2 or heights.size() != resolution * resolution:
		return
	var size := float(data.get("size", 0.0))
	if not is_finite(size) or size <= 0.0:
		return
	var packed := PackedFloat32Array()
	for value in heights:
		packed.append(float(value))
	_ground_resolution = resolution
	_ground_size = size
	_ground_heights = packed


func _file_sha256(path: String) -> String:
	var file := FileAccess.open(path, FileAccess.READ)
	if file == null:
		return ""
	var context := HashingContext.new()
	if context.start(HashingContext.HASH_SHA256) != OK:
		file.close()
		return ""
	while not file.eof_reached():
		var chunk := file.get_buffer(4096)
		if chunk.size() > 0:
			context.update(chunk)
	file.close()
	return context.finish().hex_encode()


func is_valid() -> bool:
	return not map_id.is_empty() and not authority_version.is_empty() \
			and is_finite(min_x) and is_finite(max_x) \
			and is_finite(min_z) and is_finite(max_z) \
			and min_x < max_x and min_z < max_z \
			and is_finite(max_flight_height) and max_flight_height >= 0.0


func contains_horizontal(position: Vector3) -> bool:
	return is_valid() and position.x >= min_x and position.x <= max_x \
			and position.z >= min_z and position.z <= max_z


func allows_flight_at(position: Vector3) -> bool:
	return is_valid() and flight_enabled and contains_horizontal(position) \
			and position.y <= max_flight_height


func above_floor(position: Vector3) -> bool:
	return is_valid() and position.y >= min_ground_y


func is_blocked(position: Vector3, radius: float = 0.35) -> bool:
	if not is_valid():
		return false
	for blocker in blockers:
		if position.x + radius < blocker.min_x or position.x - radius > blocker.max_x:
			continue
		if position.z + radius < blocker.min_z or position.z - radius > blocker.max_z:
			continue
		if position.y + radius < blocker.min_y or position.y - radius > blocker.max_y:
			continue
		return true
	return false
