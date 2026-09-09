extends Node3D

const STEP := 1.0 / 60.0
var velocity := Vector3.ZERO
var server_tick := 0
var connection_epoch := 1
var last_processed_input := 0
var inputs: Dictionary = {}
var missing_ticks := 0

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
	position += velocity * STEP
