extends SceneTree

func _initialize() -> void:
	assert(FileAccess.file_exists("res://Server/server.gd"))
	var source := FileAccess.get_file_as_string("res://Server/server.gd")
	assert(source.contains("/v1/internal/rooms/register"))
	assert(source.contains("G1_ROOM_HEARTBEAT_FAILED"))
	print("G1_TEST_PASS game server contract")
	quit()
