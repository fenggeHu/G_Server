extends SceneTree

# Exercise the production RPC guard with an ENet-connected but unidentified peer.
# Only transport setup is replaced; the production move_input handler is inherited.
class Guard:
	extends "res://Server/server.gd"
	func _ready() -> void:
		var port := OS.get_environment("G2_GUARD_PORT").to_int()
		if OS.get_environment("G2_GUARD_ROLE") == "server":
			peer.set_bind_ip("127.0.0.1")
			if peer.create_server(port) != OK: get_tree().quit(1)
			multiplayer.multiplayer_peer = peer
			print("G2_GUARD_LISTENING")
		else:
			multiplayer.connected_to_server.connect(func(): move_input.rpc_id(1, 1, Vector2.RIGHT))
			if peer.create_client("127.0.0.1", port) != OK: get_tree().quit(1)
			multiplayer.multiplayer_peer = peer
	func _physics_process(_delta: float) -> void:
		if not entities.is_empty() or not identities.is_empty():
			printerr("G2_FAIL unauthorized created entity")
			get_tree().quit(1)

func _initialize() -> void:
	var main := Node.new()
	main.name = "Main"
	var network := Node.new()
	network.name = "NetworkRoot"
	main.add_child(network)
	var session := Guard.new()
	session.name = "Session"
	network.add_child(session)
	root.add_child.call_deferred(main)
