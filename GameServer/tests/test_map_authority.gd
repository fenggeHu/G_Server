extends SceneTree

const MapAuthority := preload("res://Server/map_authority.gd")

func _initialize() -> void:
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
