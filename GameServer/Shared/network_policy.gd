extends RefCounted
const P = preload("res://Shared/protocol.gd")
static func backend_allowed(url: String) -> bool:
	if url.begins_with("https://") and url.length() > 8 and not "@" in url: return true
	if OS.get_environment("G0_LOOPBACK_TEST") != "1": return false
	var r := RegEx.new(); r.compile("^http://127\\.0\\.0\\.1(:[0-9]{1,5})?$"); return r.search(url) != null
static func room_allowed(host: String) -> bool:
	return host == "127.0.0.1" if OS.get_environment("G0_LOOPBACK_TEST") == "1" else OS.get_environment("G0_PROTECTED_NETWORK") == "1" and not host.is_empty()
static func config_hash() -> String:
	return '{"gameplay_content_hash":"g0-empty-v1","max_players":8,"preset_id":"exploration","protocol_version":1}'.sha256_text()
