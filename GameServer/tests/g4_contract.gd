extends SceneTree

const P = preload("res://Shared/protocol.gd")

func _init() -> void:
	assert(P.supports_preset("coop_combat"))
	assert(P.content_hash_for("coop_combat") == "g4-coop-combat-v1")
	assert(P.ATTACK_COOLDOWN_MS == 500)
	quit(0)
