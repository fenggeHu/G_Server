extends SceneTree
## 服务端多敌人注册表与数据驱动战斗：按 id 受击、死亡/重生、玩家被打。

const MapAuthority := preload("res://Server/map_authority.gd")
const ServerScript := preload("res://Server/server.gd")


func _authority() -> RefCounted:
	var authority := MapAuthority.new()
	authority.configure({
		"map_id": "test_combat", "authority_version": "test-1",
		"boundary": {"min_x": -60.0, "max_x": 60.0, "min_z": -60.0, "max_z": 60.0},
		"flight": {"enabled": false, "max_height": 10.0},
		"ground": {"min_y": 0.0},
		"enemies": [
			{"id": "golem", "type": "stone_golem", "x": 0.0, "z": 0.0},
			{"id": "wisp", "type": "wisp", "x": 20.0, "z": 0.0},
			{"id": "bad", "type": "missing_type", "x": 0.0, "z": 0.0},
		]})
	return authority


func _initialize() -> void:
	var server = ServerScript.new()
	server.map_authority = _authority()
	server.combat_enabled = true
	server._build_enemies()

	assert(server.enemies.size() == 2, "unknown enemy type skipped")
	assert(server.enemies["golem"].hp == 80, "golem hp from catalog")
	assert(server.enemies["golem"].damage == 16, "golem damage")
	assert(server.enemies["golem"].respawn_ms == 8000, "golem respawn")
	assert(server.enemies["golem"].attack_cooldown_ms == 1200, "golem attack cooldown")
	assert(server.enemies["wisp"].hp == 40, "wisp hp")
	assert(server.enemies["golem"].position == Vector3(0.0, 0.0, 0.0), "ground position")

	var node := Node3D.new()
	root.add_child(node)
	await process_frame
	node.global_position = Vector3(0.0, 0.0, 0.0)
	server.entities[7] = node
	server.identities[7] = "p1"
	assert(server._can_attack(7, "golem", "a1", 1000), "attack in range")
	assert(not server._can_attack(7, "wisp", "a2", 1000), "attack out of range")
	assert(not server._can_attack(7, "missing", "a3", 1000), "unknown target rejected")

	var now := Time.get_ticks_msec()
	server._apply_enemy_damage("golem", 30, now)
	assert(server.enemies["golem"].hp == 50, "damage applied by id")
	assert(server.enemies["wisp"].hp == 40, "other enemy untouched")
	server._apply_enemy_damage("golem", 50, now)
	assert(server.enemies["golem"].hp == 0 and server.enemies["golem"].dead, "death by id")
	assert(server.enemies["golem"].respawn_at == now + 8000, "respawn scheduled from type")
	server._apply_enemy_damage("golem", 10, now)
	assert(server.enemies["golem"].hp == 0, "dead enemy ignores damage")

	server._tick_enemy_respawns(now + 7999)
	assert(server.enemies["golem"].dead, "not respawned early")
	server._tick_enemy_respawns(now + 8000)
	assert(not server.enemies["golem"].dead and server.enemies["golem"].hp == 80, "respawn restores hp")
	assert(server.enemies["golem"].revision == 4, "revision monotonic")

	server.player_health[7] = {"hp": 100, "max_hp": 100, "revision": 1}
	server._tick_enemy_attacks()
	assert(server.player_health[7].hp == 84, "player damaged by golem")
	server.player_health[7] = {"hp": 100, "max_hp": 100, "revision": 1}
	node.global_position = Vector3(20.0, 0.0, 0.0)
	server._tick_enemy_attacks()
	assert(server.player_health[7].hp == 92, "player damaged by wisp in range")

	node.queue_free()
	server.free()
	print("COMBAT_ENEMIES_PASS")
	quit(0)
