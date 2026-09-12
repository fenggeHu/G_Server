extends Node3D

const STEP := 1.0 / 60.0
var velocity := Vector3.ZERO
var server_tick := 0
var connection_epoch := 1
var last_processed_input := 0
var inputs: Dictionary = {}
var missing_ticks := 0
var map_authority = null
var flight_mode := false

func set_map_authority(value) -> void:
	map_authority = value

func set_flight_mode(value: bool) -> void:
	flight_mode = value

func enqueue(sequence: int, direction: Vector2) -> bool:
	if sequence <= last_processed_input or inputs.has(sequence): return false
	if sequence > 4294967295 or sequence - last_processed_input > 120 or inputs.size() >= 120:
		return false
	if not direction.is_finite() or direction.length_squared() > 1.0: return false
	inputs[sequence] = direction
	return true

func step(tick: int) -> void:
	server_tick = tick
	if not inputs.is_empty():
		var sequences := inputs.keys()
		sequences.sort()
		last_processed_input = sequences[0]
		var direction: Vector2 = inputs[last_processed_input]
		inputs.erase(last_processed_input)
		velocity = Vector3(direction.x, 0, direction.y) * 5.0
		missing_ticks = 0
	else:
		missing_ticks += 1
		if missing_ticks > 6: velocity = Vector3.ZERO
	# G2 basic plane only: no collision, gravity or client-supplied delta.
	var next_position := position + velocity * STEP
	if map_authority != null:
		if not map_authority.contains_horizontal(next_position):
			next_position.x = clampf(next_position.x, map_authority.min_x, map_authority.max_x)
			next_position.z = clampf(next_position.z, map_authority.min_z, map_authority.max_z)
			velocity.x = 0.0
			velocity.z = 0.0
		if flight_mode and not map_authority.allows_flight_at(next_position):
			next_position.y = minf(next_position.y, map_authority.max_flight_height)
			velocity.y = 0.0
		if not map_authority.above_floor(next_position):
			next_position.y = map_authority.min_ground_y
			velocity.y = 0.0
		if map_authority.has_method("is_blocked") and map_authority.is_blocked(next_position):
			next_position = position
			velocity = Vector3.ZERO
		if not flight_mode and map_authority.has_method("has_ground") and map_authority.has_ground():
			next_position.y = map_authority.ground_height(next_position.x, next_position.z)
			velocity.y = 0.0
	position = next_position
