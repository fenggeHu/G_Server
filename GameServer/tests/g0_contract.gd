extends SceneTree

func _initialize() -> void:
	assert(FileAccess.file_exists("res://Bootstrap/Main.tscn"))
	assert(FileAccess.file_exists("res://Server/server.gd"))
	assert(FileAccess.file_exists("res://Server/world_items.gd"))
	assert(FileAccess.file_exists("res://Shared/protocol.gd"))
	print("G0_TEST_PASS game server contract")
	quit()
