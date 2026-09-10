extends RefCounted
const P = preload("res://Shared/protocol.gd")
static func backend_allowed(url: String) -> bool:
	if url.begins_with("https://") and url.length() > 8 and not "@" in url: return true
	if OS.get_environment("G0_PROTECTED_NETWORK") == "1" and url.begins_with("http://") and not "@" in url: return true
	if OS.get_environment("G0_LOOPBACK_TEST") != "1": return false
	var r := RegEx.new(); r.compile("^http://127\\.0\\.0\\.1(:[0-9]{1,5})?$"); return r.search(url) != null
static func room_allowed(host: String) -> bool:
	return host == "127.0.0.1" if OS.get_environment("G0_LOOPBACK_TEST") == "1" else OS.get_environment("G0_PROTECTED_NETWORK") == "1" and not host.is_empty()
static func config_hash() -> String:
	var preset := OS.get_environment("ROOM_PRESET")
	if preset.is_empty(): preset = "exploration"
	var content := "g4-coop-combat-v1" if preset == "coop_combat" else "g0-empty-v1"
	return ('{"gameplay_content_hash":"' + content + '","max_players":8,"preset_id":"' + preset + '","protocol_version":1}').sha256_text()
