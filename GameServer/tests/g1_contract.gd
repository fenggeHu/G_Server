extends SceneTree

func _initialize() -> void:
	var server = load("res://Server/server.gd").new()
	if not server.has_method("player_joined") or not server.has_method("player_left"):
		server.free()
		quit(1)
		return
	server.free()
	var source := FileAccess.get_file_as_string("res://Server/server.gd")
	if not source.contains('"protocol_version": P.PROTOCOL_VERSION') or not source.contains('"room_id": room_id') or not source.contains("heartbeat.wait_time = 5.0"):
		quit(1)
		return
	print("G1_TEST_PASS game server contract")
	quit()
