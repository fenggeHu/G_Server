extends RefCounted
class_name G0Protocol
const PROTOCOL_VERSION := 1
const DEFAULT_PRESET := "exploration"
const PRESET_ID := DEFAULT_PRESET
const CONTENT_HASH := "g0-empty-v1"
const COMBAT_CONTENT_HASH := "g4-coop-combat-v1"
const ATTACK_COOLDOWN_MS := 500
static func supports_preset(value: String) -> bool: return value == "exploration" or value == "coop_combat"
static func content_hash_for(value: String) -> String: return COMBAT_CONTENT_HASH if value == "coop_combat" else CONTENT_HASH
const MAX_AUTH_PAYLOAD := 4096
const RECONNECT_GRACE_MS := 30000
const RECONNECT_TOKEN_LENGTH := 64
