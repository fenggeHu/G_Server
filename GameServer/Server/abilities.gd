extends RefCounted
## 服务端权威技能目录：目录数据与客户端内容同源（Content/world/abilities.json）。
## 只有这里的技能可被客户端请求；数值（伤害/冷却/范围）以服务端为准。

const PATH := "res://Content/world/abilities.json"

static var _abilities: Dictionary = {}
static var _loaded := false


static func _ensure() -> void:
	if _loaded:
		return
	_loaded = true
	if not FileAccess.file_exists(PATH):
		push_error("Abilities: missing %s" % PATH)
		return
	var file := FileAccess.open(PATH, FileAccess.READ)
	if file == null:
		push_error("Abilities: unreadable %s" % PATH)
		return
	var parser := JSON.new()
	var parsed := parser.parse(file.get_as_text())
	file.close()
	if parsed != OK or not parser.data is Dictionary:
		push_error("Abilities: invalid JSON %s" % PATH)
		return
	var data: Dictionary = parser.data
	if int(data.get("version", 0)) != 1:
		push_error("Abilities: unsupported version")
		return
	var entries = data.get("abilities", {})
	if not entries is Dictionary:
		push_error("Abilities: abilities must be an object")
		return
	var loaded := {}
	for ability_id in entries.keys():
		var definition = entries[ability_id]
		if not _valid_definition(String(ability_id), definition):
			push_error("Abilities: invalid ability '%s'" % ability_id)
			return
		loaded[String(ability_id)] = definition
	_abilities = loaded


static func is_valid() -> bool:
	_ensure()
	return not _abilities.is_empty()


static func has_ability(ability_id: String) -> bool:
	_ensure()
	return _abilities.has(ability_id)


static func definition(ability_id: String) -> Dictionary:
	_ensure()
	var value = _abilities.get(ability_id, {})
	return value.duplicate() if value is Dictionary else {}


static func ability_ids() -> Array:
	_ensure()
	var ids: Array = _abilities.keys()
	ids.sort()
	return ids


static func _valid_definition(ability_id: String, definition) -> bool:
	if not _valid_id(ability_id) or not definition is Dictionary:
		return false
	if String(definition.get("display_key", "")).is_empty():
		return false
	if int(definition.get("damage", 0)) < 1:
		return false
	if int(definition.get("cooldown_ms", -1)) < 0:
		return false
	if float(definition.get("max_range", 0.0)) <= 0.0:
		return false
	return true


static func _valid_id(value: String) -> bool:
	if value.is_empty() or value.length() > 64:
		return false
	for character in value:
		if not (character in "abcdefghijklmnopqrstuvwxyz0123456789_"):
			return false
	return true
