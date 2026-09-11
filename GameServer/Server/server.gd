extends Node

const P = preload("res://Shared/protocol.gd")
const POLICY = preload("res://Shared/network_policy.gd")
const Movement = preload("res://Server/movement.gd")
var entities: Dictionary = {}
var server_tick := 0
var peer := ENetMultiplayerPeer.new()
var pending: Dictionary = {}
var identities: Dictionary = {}
var room_id := "g0-room"
var base := ""
var service_token := ""
var generation := 0
var auth_generation := 0
var leases: Dictionary = {}
var pickup_entities := {"pickup-1": {"position": Vector3.ZERO, "state": "available", "item": "coin"}}
var heartbeat: Timer
var combat_enabled := false
var enemy := {"hp": 30, "revision": 1, "dead": false, "respawn_at": 0}
var attack_seen: Dictionary = {}
var attack_last: Dictionary = {}
var enemy_node: Node3D
var preset_id := "exploration"
var content_hash := P.CONTENT_HASH

func _ready() -> void:
	Engine.physics_ticks_per_second = 60
	_start.call_deferred()

func _start() -> void:
	preset_id = OS.get_environment("ROOM_PRESET")
	if preset_id.is_empty(): preset_id = P.DEFAULT_PRESET
	combat_enabled = preset_id == "coop_combat"
	content_hash = P.content_hash_for(preset_id)
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
	generation = max(1, OS.get_environment("ROOM_GENERATION").to_int())
	if port < 1 or port > 65535:
		_fatal()
		return
	if not await _register_room(port):
		_fatal()
		return
	if combat_enabled:
		enemy_node = Node3D.new(); enemy_node.name = "enemy_1"; enemy_node.position = Vector3(0, 0, 2); add_child(enemy_node)
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
		for existing in identities:
			if existing != id: player_joined.rpc_id(id, identities[existing], existing, identities.size())
		player_joined.rpc(identities[id], id, identities.size())
		if combat_enabled: health_changed.rpc_id(id, "enemy_1", enemy.hp, enemy.revision)
		print("G1 player_joined player_id=" + identities[id])
	)
	api.multiplayer_peer = peer
	heartbeat = Timer.new()
	heartbeat.wait_time = 5.0
	heartbeat.timeout.connect(_heartbeat_room)
	add_child(heartbeat)
	heartbeat.start()
	var lease_timer := Timer.new(); lease_timer.wait_time = 10.0; lease_timer.timeout.connect(_renew_sessions); add_child(lease_timer); lease_timer.start()
	print("G0_SERVER_LISTENING")

func _room_post(path: String, body: Dictionary) -> Array:
	var http := HTTPRequest.new()
	http.timeout = 4.0
	add_child(http)
	var err := http.request(base + path, ["Content-Type: application/json", "Authorization: Bearer " + service_token], HTTPClient.METHOD_POST, JSON.stringify(body))
	if err != OK:
		http.queue_free(); return []
	var result: Array = await http.request_completed
	http.queue_free()
	return result

func _register_room(port: int) -> bool:
	var response := await _room_post("/v1/internal/rooms/register", {"room_id": room_id, "host": OS.get_environment("ROOM_HOST") if not OS.get_environment("ROOM_HOST").is_empty() else "127.0.0.1", "port": port, "protocol_version": P.PROTOCOL_VERSION, "gameplay_content_hash": content_hash, "resolved_config_hash": POLICY.config_hash(), "capacity": 8, "generation": generation, "status": "ready"})
	if response.size() > 1 and response[1] != 200:
		print("G0_ROOM_REGISTER_REJECTED status=" + str(response[1]))
	return response.size() > 1 and response[0] == HTTPRequest.RESULT_SUCCESS and response[1] == 200

func _heartbeat_room() -> void:
	var response := await _room_post("/v1/internal/rooms/heartbeat", {"room_id": room_id, "generation": generation, "capacity": 8, "used_players": identities.size(), "status": "ready"})
	if response.size() <= 1 or response[0] != HTTPRequest.RESULT_SUCCESS or response[1] != 200:
		print("G1_ROOM_HEARTBEAT_FAILED")

func _pending_peer(id: int) -> void:
	if pending.size() >= 16 or identities.size() >= 8:
		_reject(id)
		return
	auth_generation += 1
	pending[id] = {"generation": auth_generation, "submitted": false}

func _remove_peer(id: int) -> void:
	var was_authenticated := identities.has(id)
	pending.erase(id)
	var player_id: String = identities.get(id, "")
	identities.erase(id)
	entities.erase(id)
	var players := get_node_or_null("Players")
	if was_authenticated and players:
		var node := players.get_node_or_null(_safe_name(player_id))
		if node: node.queue_free()
		player_left.rpc(player_id, id, identities.size())
		print("G1 player_left player_id=" + player_id)
		await _room_post("/v1/internal/rooms/release", {"room_id": room_id, "player_id": player_id})

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
	if body.size() != 6 or not body.get("ticket") is String or body.ticket.is_empty() or body.ticket.length() > 256 or body.get("room_id") != room_id or body.get("protocol_version") != P.PROTOCOL_VERSION or body.get("gameplay_content_hash") != content_hash or body.get("resolved_config_hash") != POLICY.config_hash():
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
	if not result.get("player_id") is String or result.get("room_id") != room_id or result.get("preset_id") != preset_id or result.get("resolved_config_hash") != POLICY.config_hash() or identities.values().has(result.player_id):
		_reject(id)
		return
	var lease := await _progress_post("acquire", {"player_id": result.player_id, "room_id": room_id})
	if lease.is_empty() or int(lease.get("fencing_token", 0)) < 1:
		_reject(id); return
	leases[id] = lease
	identities[id] = result.player_id
	var players := get_node_or_null("Players")
	if not players:
		players = Node.new(); players.name = "Players"; add_child(players)
	var player := Movement.new(); player.name = _safe_name(result.player_id); player.set_meta("player_id", result.player_id); player.set_meta("connection_id", id); players.add_child(player)
	entities[id] = player
	multiplayer.send_auth(id, JSON.stringify({"status": "authenticated", "room_id": room_id, "resolved_config_hash": POLICY.config_hash(), "player_id": result.player_id}).to_utf8_buffer())
	multiplayer.complete_auth(id)

func _safe_name(value: String) -> String:
	var result := "player_"
	for c in value:
		if c.to_lower() in "abcdefghijklmnopqrstuvwxyz0123456789_": result += c
	return result

func _progress_post(action: String, body: Dictionary) -> Dictionary:
	var response := await _room_post("/v1/internal/progress/" + action, body)
	if response.size() <= 3 or response[0] != HTTPRequest.RESULT_SUCCESS or response[1] != 200:
		return {}
	var p := JSON.new()
	if p.parse(response[3].get_string_from_utf8()) != OK or not p.data is Dictionary:
		return {}
	return p.data

func _renew_sessions() -> void:
	for id in leases.keys():
		var x: Dictionary = leases[id]; var out := await _progress_post("renew", {"player_id": x.player_id, "room_id": room_id, "fencing_token": x.fencing_token})
		if out.is_empty():
			leases[id]["uncertain"] = true
		else: leases[id] = out

func _try_pickup(id: int, entity_id: String, request_id: String) -> void:
	if not identities.has(id) or not leases.has(id) or leases[id].get("uncertain", false):
		return
	if not pickup_entities.has(entity_id):
		pickup_result.rpc_id(id, request_id, {"status": "rejected", "code": "unknown_entity"})
		return
	var entity: Dictionary = pickup_entities[entity_id]
	if entity.state != "available":
		pickup_result.rpc_id(id, request_id, {"status": "rejected", "code": "already_consumed"})
		return
	entity.state = "reserved"; entity.operation_id = identities[id] + ":" + request_id
	pickup_entities[entity_id] = entity
	var x: Dictionary = leases[id]
	var out := await _progress_post("commit", {"player_id": identities[id], "room_id": room_id, "fencing_token": x.fencing_token, "expected_revision": 0, "operation_id": entity.operation_id, "payload": {"item": entity.item, "amount": 1}})
	if out.is_empty():
		return
	if out.get("status") == "succeeded":
		entity.state = "consumed"; pickup_entities[entity_id] = entity; pickup_result.rpc_id(id, request_id, out)
	else:
		entity.state = "available"; pickup_entities[entity_id] = entity; pickup_result.rpc_id(id, request_id, out)

@rpc("any_peer", "call_remote", "reliable")
func try_pickup(entity_id: String, request_id: String) -> void:
	_try_pickup(multiplayer.get_remote_sender_id(), entity_id, request_id)

@rpc("authority", "call_remote", "reliable")
func pickup_result(_request_id: String, _result: Dictionary) -> void: pass

@rpc("authority", "call_remote", "reliable")
func player_joined(_player_id: String, _connection_id: int, _count: int) -> void: pass

@rpc("authority", "call_remote", "reliable")
func player_left(_player_id: String, _connection_id: int, _count: int) -> void: pass

func _fatal() -> void:
	print("G0_SERVER_START_FAILED")
	get_tree().quit(1)

func _physics_process(_delta: float) -> void:
	if combat_enabled and enemy.dead and Time.get_ticks_msec() >= enemy.respawn_at:
		enemy = {"hp": 30, "revision": enemy.revision + 1, "dead": false, "respawn_at": 0}; entity_spawned.rpc("enemy_1", enemy.revision)
	server_tick = (server_tick + 1) & 0xffffffff
	if peer.get_connection_status() != MultiplayerPeer.CONNECTION_CONNECTED: return
	for id in entities:
		if not multiplayer.get_peers().has(id): continue
		var entity = entities[id]
		entity.step(server_tick)
		for recipient in multiplayer.get_peers():
			if identities.has(recipient):
				snapshot.rpc_id(recipient, identities[id], entity.position, entity.velocity, server_tick, entity.connection_epoch, entity.last_processed_input)

@rpc("any_peer", "call_remote", "unreliable_ordered", 0)
func move_input(sequence: int, direction: Vector2) -> void:
	var id := multiplayer.get_remote_sender_id()
	if not identities.has(id) or not entities.has(id):
		print("G2_INPUT_REJECTED unauthorized")
		return
	var entity = entities[id]
	if entity.enqueue(sequence, direction): return
	if not direction.is_finite() or direction.length_squared() > 1.0:
		print("G2_INPUT_REJECTED direction")
	elif sequence - entity.last_processed_input > 120:
		print("G2_INPUT_REJECTED future")

@rpc("authority", "call_remote", "unreliable_ordered", 0)
func snapshot(_player_id: String, _position: Vector3, _velocity: Vector3, _server_tick: int, _epoch: int, _last_seq: int) -> void: pass

@rpc("any_peer", "call_remote", "reliable")
func request_attack(target_id: String, attack_id: String) -> void:
	var id := multiplayer.get_remote_sender_id()
	var now := Time.get_ticks_msec()
	if not _can_attack(id, target_id, attack_id, now): return
	_record_attack(id, attack_id, now)
	_apply_attack(now)

func _can_attack(id: int, target_id: String, attack_id: String, now: int) -> bool:
	if not combat_enabled or not identities.has(id) or not entities.has(id) or not enemy_node or target_id != "enemy_1": return false
	if attack_id.is_empty() or attack_id.length() > 128: return false
	if attack_seen.has(identities[id]) and attack_seen[identities[id]].has(attack_id): return false
	if now - int(attack_last.get(id, -1000000)) < P.ATTACK_COOLDOWN_MS: return false
	return entities[id].global_position.distance_squared_to(enemy_node.global_position) <= 9.0

func _record_attack(id: int, attack_id: String, now: int) -> void:
	attack_seen[identities[id]] = attack_seen.get(identities[id], {})
	attack_seen[identities[id]][attack_id] = true
	attack_last[id] = now

func _apply_attack(now: int) -> void:
	if enemy.dead: return
	enemy.hp -= 10
	enemy.revision += 1
	health_changed.rpc("enemy_1", enemy.hp, enemy.revision)
	if enemy.hp == 0:
		enemy.dead = true
		enemy.respawn_at = now + 5000
		entity_died.rpc("enemy_1", enemy.revision)
@rpc("authority", "call_remote", "reliable") func health_changed(_entity_id: String, _hp: int, _revision: int) -> void: pass
@rpc("authority", "call_remote", "reliable") func entity_died(_entity_id: String, _revision: int) -> void: pass
@rpc("authority", "call_remote", "reliable") func entity_spawned(_entity_id: String, _revision: int) -> void: pass
@rpc("authority", "call_remote", "reliable") func player_health_changed(_entity_id: String, _hp: int, _max_hp: int, _revision: int) -> void: pass
