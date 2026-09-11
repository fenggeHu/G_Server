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
	return is_valid() and flight_enabled and position.y <= max_flight_height
