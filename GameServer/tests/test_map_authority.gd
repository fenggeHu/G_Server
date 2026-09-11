extends SceneTree

const MapAuthority := preload("res://Server/map_authority.gd")
const Movement := preload("res://Server/movement.gd")

func _initialize() -> void:
	var path := "user://map_authority_test.json"
	var file := FileAccess.open(path, FileAccess.WRITE)
	file.store_string(JSON.stringify({
		"map_id": "file_map", "authority_version": "file-v1",
		"boundary": {"min_x": -10.0, "max_x": 10.0, "min_z": -20.0, "max_z": 20.0},
		"flight": {"enabled": false, "max_height": 15.0}}))
	file.close()
	var loaded = MapAuthority.load_json(path, "file_map", "file-v1")
	assert(loaded != null and loaded.contains_horizontal(Vector3.ZERO))
	assert(MapAuthority.load_json(path, "wrong_map", "file-v1") == null)
	DirAccess.remove_absolute(path)

	var authority := MapAuthority.new()
	assert(authority.configure({
		"map_id": "starter_valley",
		"authority_version": "starter_valley-authority-0",
		"boundary": {"min_x": -140.0, "max_x": 140.0, "min_z": -140.0, "max_z": 140.0},
		"flight": {"enabled": true, "max_height": 28.0}}))
	assert(authority.contains_horizontal(Vector3.ZERO))
	assert(not authority.contains_horizontal(Vector3(141.0, 0.0, 0.0)))
	assert(authority.allows_flight_at(Vector3(0.0, 28.0, 0.0)))
	assert(not authority.allows_flight_at(Vector3(0.0, 28.1, 0.0)))
	assert(not authority.allows_flight_at(Vector3(141.0, 0.0, 0.0)))
	var invalid := MapAuthority.new()
	assert(not invalid.configure({
		"map_id": "broken", "authority_version": "v1",
		"boundary": {"min_x": 1.0, "max_x": -1.0},
		"flight": {"enabled": true, "max_height": 10.0}}))
	var movement := Movement.new()
	movement.set_map_authority(authority)
	movement.position = Vector3(139.99, 0.0, 0.0)
	assert(movement.enqueue(1, Vector2(1.0, 0.0)))
	movement.step(1)
	assert(movement.position.x == 140.0)
	movement.flight_mode = true
	movement.position.y = 28.1
	assert(movement.enqueue(2, Vector2.ZERO))
	movement.step(2)
	assert(movement.position.y <= 28.0)
	movement.free()
	var server = load("res://Server/server.gd").new()
	server.map_authority = authority
	var player = server._create_player("test-player", 42)
	assert(player.map_authority == authority)
	player.position = Vector3(-139.99, 0.0, 139.99)
	assert(player.enqueue(1, Vector2(-1, 1).normalized()))
	player.step(1)
	assert(player.position.x == -140.0 and player.position.z == 140.0)
	assert(player.last_processed_input == 1)
	player.free()
	server.free()
	print("MAP_AUTHORITY_PASS")
	quit(0)
