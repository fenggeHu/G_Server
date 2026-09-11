extends RefCounted
## 服务器权威的世界可拾取物目录：位置/物品/状态由服务器决定，客户端不可自定义。
## 当前为服务器内置默认；待地图内容格式冻结后改为从内容清单同步（见 docs 迭代状态）。

const VALID_STATES := ["available", "taken"]
const PLAYABLE_HALF_SIZE := 100.0

const DEFAULT_ENTRIES := {
	"pickup-1": {"position": Vector3(1.5, 0, 0), "state": "available", "item": "coin"},
	"pickup-far": {"position": Vector3(60.0, 0, 60.0), "state": "available", "item": "coin"},
}

static func build(entries: Dictionary = DEFAULT_ENTRIES) -> Dictionary:
	var out := {}
	for id in entries:
		out[String(id)] = _normalize(String(id), entries[id])
	return out

static func validate(catalog: Dictionary) -> bool:
	for id in catalog:
		var entry = catalog[id]
		if typeof(entry) != TYPE_DICTIONARY or entry.is_empty():
			return false
		if String(entry.get("item", "")).is_empty():
			return false
		if not VALID_STATES.has(String(entry.get("state", ""))):
			return false
		var position: Vector3 = entry.get("position", Vector3.ZERO)
		if absf(position.x) > PLAYABLE_HALF_SIZE or absf(position.z) > PLAYABLE_HALF_SIZE:
			return false
	return true

static func _normalize(id: String, raw) -> Dictionary:
	if typeof(raw) != TYPE_DICTIONARY:
		push_error("world item %s must be a dictionary" % id)
		return {}
	var entry: Dictionary = raw
	var position: Vector3 = entry.get("position", Vector3.ZERO)
	if absf(position.x) > PLAYABLE_HALF_SIZE or absf(position.z) > PLAYABLE_HALF_SIZE:
		push_error("world item %s out of playable bounds" % id)
	var state := String(entry.get("state", "available"))
	if not VALID_STATES.has(state):
		state = "available"
	var item := String(entry.get("item", ""))
	if item.is_empty():
		push_error("world item %s missing item" % id)
	return {"position": position, "state": state, "item": item}
