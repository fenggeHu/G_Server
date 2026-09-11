extends SceneTree

const MapAuthority := preload("res://Server/map_authority.gd")

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
	var invalid := MapAuthority.new()
	assert(not invalid.configure({
		"map_id": "broken", "authority_version": "v1",
		"boundary": {"min_x": 1.0, "max_x": -1.0},
		"flight": {"enabled": true, "max_height": 10.0}}))
	print("MAP_AUTHORITY_PASS")
	quit(0)
