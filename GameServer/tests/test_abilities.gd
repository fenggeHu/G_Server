extends SceneTree
## 服务端技能目录与请求入口存在性。

const Abilities := preload("res://Server/abilities.gd")
const ServerScript := preload("res://Server/server.gd")

func _initialize() -> void:
	assert(Abilities.has_ability("heavy_strike"), "heavy_strike registered")
	var definition := Abilities.definition("heavy_strike")
	assert(int(definition.get("damage", 0)) == 25, "damage")
	assert(int(definition.get("cooldown_ms", 0)) == 1500, "cooldown")
	assert(absf(float(definition.get("max_range", 0.0)) - 3.5) < 0.001, "range")
	assert(not Abilities.has_ability("missing"), "unknown rejected")
	assert(Abilities.ability_ids() == ["heavy_strike"], "id list")

	var server = ServerScript.new()
	assert(server.has_method("request_ability"), "server ability RPC")
	server.free()

	print("ABILITIES_PASS")
	quit(0)
