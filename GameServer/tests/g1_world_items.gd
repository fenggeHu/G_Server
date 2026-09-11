extends SceneTree
## 服务器世界可拾取物目录：默认目录完整性、边界校验与状态规范化。

const WorldItems := preload("res://Server/world_items.gd")

func _initialize() -> void:
	var catalog := WorldItems.build()
	assert(catalog.has("pickup-1") and catalog.has("pickup-far"))
	assert(WorldItems.validate(catalog))
	assert(String(catalog["pickup-1"]["item"]) == "coin")
	assert(catalog["pickup-1"]["position"] is Vector3)
	assert(String(catalog["pickup-1"]["state"]) == "available")

	var out_of_bounds := WorldItems.build({"far": {"position": Vector3(500, 0, 0), "state": "available", "item": "coin"}})
	assert(not WorldItems.validate(out_of_bounds), "out-of-bounds item must fail validation")

	var bad_state := WorldItems.build({"x": {"position": Vector3.ZERO, "state": "weird", "item": "coin"}})
	assert(String(bad_state["x"]["state"]) == "available", "unknown state must normalize")

	var empty_item := WorldItems.build({"x": {"position": Vector3.ZERO, "state": "available", "item": ""}})
	assert(not WorldItems.validate(empty_item), "empty item must fail validation")

	print("G1_WORLD_ITEMS_PASS")
	quit()
