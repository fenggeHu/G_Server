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
	return is_valid()


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
