extends RefCounted
## 服务端权威敌人目录：目录数据与客户端内容同源（Content/world/enemies.json）。
## 只读取权威数值（生命/伤害/重生/攻击节奏），不加载客户端视觉资源。

const PATH := "res://Content/world/enemies.json"

static var _enemies: Dictionary = {}
static var _loaded := false


static func _ensure() -> void:
	if _loaded:
		return
	_loaded = true
	if not FileAccess.file_exists(PATH):
		push_error("Enemies: missing %s" % PATH)
		return
	var file := FileAccess.open(PATH, FileAccess.READ)
	if file == null:
		push_error("Enemies: unreadable %s" % PATH)
		return
	var parser := JSON.new()
	var parsed := parser.parse(file.get_as_text())
	file.close()
	if parsed != OK or not parser.data is Dictionary:
		push_error("Enemies: invalid JSON %s" % PATH)
		return
	var data: Dictionary = parser.data
	if int(data.get("version", 0)) != 1:
		push_error("Enemies: unsupported version")
		return
	var entries = data.get("enemies", {})
	if not entries is Dictionary:
		push_error("Enemies: enemies must be an object")
		return
	var loaded := {}
	for enemy_type in entries.keys():
		var definition = entries[enemy_type]
		if not _valid_definition(String(enemy_type), definition):
			push_error("Enemies: invalid enemy '%s'" % enemy_type)
			return
		loaded[String(enemy_type)] = definition
	_enemies = loaded


static func is_valid() -> bool:
	_ensure()
	return not _enemies.is_empty()


static func has_enemy(enemy_type: String) -> bool:
	_ensure()
	return _enemies.has(enemy_type)


static func definition(enemy_type: String) -> Dictionary:
	_ensure()
	var value = _enemies.get(enemy_type, {})
	return value.duplicate() if value is Dictionary else {}


static func enemy_types() -> Array:
	_ensure()
	var ids: Array = _enemies.keys()
	ids.sort()
	return ids


static func hp(enemy_type: String) -> int:
	return int(definition(enemy_type).get("hp", 1))


static func damage(enemy_type: String) -> int:
	return int(definition(enemy_type).get("damage", 0))


static func respawn_ms(enemy_type: String) -> int:
	return int(definition(enemy_type).get("respawn_ms", 0))


static func attack_cooldown_ms(enemy_type: String) -> int:
	return int(definition(enemy_type).get("attack_cooldown_ms", 0))


static func _valid_definition(enemy_type: String, definition) -> bool:
	if not _valid_id(enemy_type) or not definition is Dictionary:
		return false
	if String(definition.get("display_key", "")).is_empty():
		return false
	var height = definition.get("height", 0.0)
	if not (height is int or height is float) or not is_finite(float(height)) or float(height) <= 0.0:
		return false
	if not _valid_int(definition, "hp", 1):
		return false
	if not _valid_int(definition, "damage", 0):
		return false
	if not _valid_int(definition, "respawn_ms", 0):
		return false
	if not _valid_int(definition, "attack_cooldown_ms", 0):
		return false
	return true


static func _valid_int(definition: Dictionary, key: String, minimum: int) -> bool:
	var value = definition.get(key, null)
	if not (value is int or value is float):
		return false
	if not is_finite(float(value)) or float(value) < float(minimum):
		return false
	return absf(float(value) - float(int(value))) < 0.0001


static func _valid_id(value: String) -> bool:
	if value.is_empty() or value.length() > 64:
		return false
	for character in value:
		if not (character in "abcdefghijklmnopqrstuvwxyz0123456789_"):
			return false
	return true
