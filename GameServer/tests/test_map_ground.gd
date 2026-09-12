extends SceneTree
## E9：逐点地面高度权威；哈希不符不加载；非飞行移动跟随地面。

const MapAuthority := preload("res://Server/map_authority.gd")
const Movement := preload("res://Server/movement.gd")

func sha256(path: String) -> String:
	var file := FileAccess.open(path, FileAccess.READ)
	if file == null:
		return ""
	var context := HashingContext.new()
	context.start(HashingContext.HASH_SHA256)
	while not file.eof_reached():
		var chunk := file.get_buffer(4096)
		if chunk.size() > 0:
			context.update(chunk)
	file.close()
	return context.finish().hex_encode()

func _initialize() -> void:
	var grid_path := "user://ground_test.json"
	var file := FileAccess.open(grid_path, FileAccess.WRITE)
	file.store_string(JSON.stringify({
		"resolution": 3, "size": 100.0, "min_y": 0.0, "max_y": 5.0,
		"heights": [0.0, 0.0, 0.0, 0.0, 5.0, 0.0, 0.0, 0.0, 0.0]}))
	file.close()
	var digest := sha256(grid_path)

	var authority := MapAuthority.new()
	assert(authority.configure({
		"map_id": "ground_map", "authority_version": "v1",
		"boundary": {"min_x": -50.0, "max_x": 50.0, "min_z": -50.0, "max_z": 50.0},
		"flight": {"enabled": true, "max_height": 30.0},
		"ground": {"min_y": 0.0, "max_y": 5.0, "resolution": 3, "size": 100.0,
			"grid_path": grid_path, "sha256": digest}}))
	assert(authority.has_ground(), "grid loaded")
	assert(absf(authority.ground_height(0.0, 0.0) - 5.0) < 0.001, "center height")
	assert(absf(authority.ground_height(-50.0, -50.0)) < 0.001, "corner height")

	var tampered := MapAuthority.new()
	assert(tampered.configure({
		"map_id": "ground_map", "authority_version": "v1",
		"boundary": {"min_x": -50.0, "max_x": 50.0, "min_z": -50.0, "max_z": 50.0},
		"flight": {"enabled": true, "max_height": 30.0},
		"ground": {"min_y": 0.0, "max_y": 5.0, "resolution": 3, "size": 100.0,
			"grid_path": grid_path, "sha256": "0".repeat(64)}}))
	assert(not tampered.has_ground(), "hash mismatch rejects grid")

	var movement := Movement.new()
	movement.set_map_authority(authority)
	movement.position = Vector3(0.0, 0.0, 0.0)
	movement.step(1)
	assert(absf(movement.position.y - 5.0) < 0.001, "terrain follow")
	movement.free()

	DirAccess.remove_absolute(ProjectSettings.globalize_path(grid_path))
	print("MAP_GROUND_PASS")
	quit(0)
