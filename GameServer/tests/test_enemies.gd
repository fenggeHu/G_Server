extends SceneTree
## 服务端敌人目录：加载、权威数值与未知类型拒绝。

const Enemies := preload("res://Server/enemies.gd")


func _initialize() -> void:
	assert(Enemies.is_valid(), "catalog valid")
	assert(Enemies.has_enemy("dummy"), "dummy registered")
	assert(Enemies.has_enemy("stone_golem"), "stone_golem registered")
	assert(Enemies.hp("dummy") == 30, "dummy hp")
	assert(Enemies.damage("dummy") == 10, "dummy damage")
	assert(Enemies.respawn_ms("dummy") == 5000, "dummy respawn")
	assert(Enemies.attack_cooldown_ms("dummy") == 1000, "dummy attack cooldown")
	assert(Enemies.hp("dracling") == 120, "dracling hp")
	assert(Enemies.damage("wisp") == 8, "wisp damage")
	assert(not Enemies.has_enemy("missing"), "unknown rejected")
	assert(Enemies.enemy_types() == ["dracling", "dummy", "stone_golem", "wisp"], "id list")

	print("ENEMIES_PASS")
	quit(0)
