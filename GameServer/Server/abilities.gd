extends RefCounted
## 服务端权威技能目录：只有登记的技能可被客户端请求。
## 客户端内容目录（Content/world/abilities.json）必须与这里的 id/数值保持一致。

const ABILITIES := {
	"heavy_strike": {"damage": 25, "cooldown_ms": 1500, "max_range": 3.5},
}


static func has_ability(ability_id: String) -> bool:
	return ABILITIES.has(ability_id)


static func definition(ability_id: String) -> Dictionary:
	var value = ABILITIES.get(ability_id, {})
	return value.duplicate() if value is Dictionary else {}


static func ability_ids() -> Array:
	var ids: Array = ABILITIES.keys()
	ids.sort()
	return ids
