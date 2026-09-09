extends SceneTree

var failures := 0

func check(ok: bool, label: String) -> void:
	if not ok:
		failures += 1
		printerr("G2_FAIL " + label)

func _initialize() -> void:
	var script = load("res://Server/server.gd")
	var server = script.new()
	check(server.has_method("move_input"), "native move_input RPC missing")
	check(server.has_method("snapshot"), "native snapshot RPC missing")
	server.free()
	if not FileAccess.file_exists("res://Server/movement.gd"):
		check(false, "authoritative movement runtime missing")
	else:
		var movement = load("res://Server/movement.gd")
		var player = movement.new()
		check(player is Node3D, "runtime entity is Node3D")
		for i in range(1, 61):
			check(player.enqueue(i, Vector2.RIGHT), "valid input")
			player.step(i)
		check(player.position.distance_to(Vector3(5, 0, 0)) < 0.0001, "60 steps = 5m")
		check(player.velocity == Vector3(5, 0, 0), "velocity")
		check(player.last_processed_input == 60 and player.connection_epoch == 1, "ack identity")
		check(not player.enqueue(60, Vector2.RIGHT), "old sequence")
		check(not player.enqueue(181, Vector2.RIGHT), "future window")
		for direction in [Vector2(NAN, 0), Vector2(INF, 0), Vector2(1, 1), Vector2(-1.01, 0)]:
			check(not player.enqueue(61, direction), "invalid direction")
		check(player.enqueue(180, Vector2.ZERO), "window inclusive")
		check(not player.enqueue(180, Vector2.ZERO), "duplicate")
		player.step(61)
		check(player.last_processed_input == 180, "one input per tick")
		check(player.position.distance_to(Vector3(5, 0, 0)) < 0.0001, "neutral input")
		player.free()
		player = movement.new()
		for i in range(1, 121): check(player.enqueue(i, Vector2(0.6, 0.8)), "bounded queue")
		player.step(1)
		check(player.last_processed_input == 1, "burst cannot accelerate time")
		check(player.position.distance_to(Vector3(0.05, 0, 1.0 / 15.0)) < 0.00001, "diagonal vector")
		player.free()
		player = movement.new()
		player.enqueue(1, Vector2.LEFT)
		for i in range(1, 10): player.step(i)
		check(player.velocity == Vector3.ZERO, "missing input stops after 100ms")
		check(player.position.x >= -7.0 / 12.0 - 0.00001, "bounded input hold")
		player.free()
	if failures == 0: print("G2_CONTRACT_PASS")
	quit(1 if failures else 0)
