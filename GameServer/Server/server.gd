extends Node

const P = preload("res://Shared/protocol.gd")
const POLICY = preload("res://Shared/network_policy.gd")
var peer := ENetMultiplayerPeer.new()
var pending: Dictionary = {}
var identities: Dictionary = {}
var room_id := "g0-room"
var base := ""
var service_token := ""
var generation := 0

func _ready() -> void:
	_start.call_deferred()

func _start() -> void:
	base = OS.get_environment("G0_BACKEND_URL").trim_suffix("/")
	service_token = OS.get_environment("ROOM_SERVICE_TOKEN")
	if not POLICY.backend_allowed(base) or service_token.length() < 32:
		_fatal()
		return
	var loopback := OS.get_environment("G0_LOOPBACK_TEST") == "1"
	if not loopback and OS.get_environment("G0_PROTECTED_NETWORK") != "1":
		_fatal()
		return
	var port := 7000 if OS.get_environment("ROOM_PORT").is_empty() else OS.get_environment("ROOM_PORT").to_int()
	if not OS.get_environment("ROOM_ID").is_empty():
		room_id = OS.get_environment("ROOM_ID")
	if port < 1 or port > 65535:
		_fatal()
		return
	if loopback:
		peer.set_bind_ip("127.0.0.1")
	if peer.create_server(port, 16) != OK:
		_fatal()
		return
	var api := multiplayer as SceneMultiplayer
	api.auth_callback = _authenticate
	api.auth_timeout = 5.0
	api.allow_object_decoding = false
	api.peer_authenticating.connect(_pending_peer)
	api.peer_authentication_failed.connect(_remove_peer)
	api.peer_disconnected.connect(_remove_peer)
	api.peer_connected.connect(func(id):
		pending.erase(id)
		print("G0_SERVER_AUTHENTICATED")
	)
	api.multiplayer_peer = peer
	print("G0_SERVER_LISTENING")

func _pending_peer(id: int) -> void:
	if pending.size() >= 16 or identities.size() >= 8:
		_reject(id)
		return
	generation += 1
	pending[id] = {"generation": generation, "submitted": false}

func _remove_peer(id: int) -> void:
	pending.erase(id)
	identities.erase(id)

func _reject(id: int) -> void:
	_remove_peer(id)
	peer.disconnect_peer(id)
	print("G0_AUTH_REJECTED")

func _authenticate(id: int, data: PackedByteArray) -> void:
	if not pending.has(id) or pending[id].submitted or data.size() > P.MAX_AUTH_PAYLOAD:
		_reject(id)
		return
	pending[id].submitted = true
	var parser := JSON.new()
	if parser.parse(data.get_string_from_utf8()) != OK or not parser.data is Dictionary:
		_reject(id)
		return
	var body: Dictionary = parser.data
	if body.size() != 5 or not body.get("ticket") is String or body.ticket.is_empty() or body.ticket.length() > 256 or body.get("room_id") != room_id or body.get("protocol_version") != P.PROTOCOL_VERSION or body.get("gameplay_content_hash") != P.CONTENT_HASH or body.get("resolved_config_hash") != POLICY.config_hash():
		_reject(id)
		return
	var request_generation: int = pending[id].generation
	body["protocol_version"] = P.PROTOCOL_VERSION
	var http := HTTPRequest.new()
	http.timeout = 4.0
	http.body_size_limit = 4096
	http.max_redirects = 0
	add_child(http)
	var err := http.request(base + "/v1/internal/tickets/redeem", ["Content-Type: application/json", "Authorization: Bearer " + service_token], HTTPClient.METHOD_POST, JSON.stringify(body))
	body.clear()
	if err != OK:
		http.queue_free()
		_reject(id)
		return
	var response: Array = await http.request_completed
	http.queue_free()
	if not pending.has(id) or pending[id].generation != request_generation:
		return
	if response[0] != HTTPRequest.RESULT_SUCCESS or response[1] != 200:
		_reject(id)
		return
	if parser.parse(response[3].get_string_from_utf8()) != OK or not parser.data is Dictionary:
		_reject(id)
		return
	var result: Dictionary = parser.data
	if not result.get("player_id") is String or result.get("room_id") != room_id or result.get("preset_id") != P.PRESET_ID or result.get("resolved_config_hash") != POLICY.config_hash() or identities.values().has(result.player_id):
		_reject(id)
		return
	identities[id] = result.player_id
	multiplayer.send_auth(id, JSON.stringify({"status": "authenticated", "room_id": room_id, "resolved_config_hash": POLICY.config_hash()}).to_utf8_buffer())
	multiplayer.complete_auth(id)

func _fatal() -> void:
	print("G0_SERVER_START_FAILED")
	get_tree().quit(1)
