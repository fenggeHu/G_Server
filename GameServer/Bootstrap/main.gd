extends Node

func _ready() -> void:
	var args := OS.get_cmdline_user_args()
	if not args.has("--server") and not OS.has_feature("dedicated_server"):
		print("G0 GameServer requires --server")
		get_tree().quit(2)
		return
	var session: Node = load("res://Server/server.gd").new()
	session.name = "Session"
	$NetworkRoot.add_child(session)
