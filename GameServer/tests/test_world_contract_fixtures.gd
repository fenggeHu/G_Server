extends SceneTree
## E8 契约夹具（服务端侧）：权威数据对夹具的接受/拒绝语义。

const MapAuthority := preload("res://Server/map_authority.gd")

const FIXTURE := "res://tests/fixtures/world_contract.json"

func _initialize() -> void:
	var file := FileAccess.open(FIXTURE, FileAccess.READ)
	assert(file != null, "fixture readable")
	var parser := JSON.new()
	assert(parser.parse(file.get_as_text()) == OK and parser.data is Dictionary, "fixture parses")
	file.close()
	var fixture: Dictionary = parser.data
	assert(int(fixture.get("version", 0)) == 1, "fixture version")
	var cases: Array = fixture.get("authority_cases", [])
	assert(cases.size() > 0, "authority cases present")

	for case in cases:
		var expect_accept: bool = String(case.get("expect", "")) == "accept"
		var authority := MapAuthority.new()
		var accepted := authority.configure(case.get("authority", {}))
		assert(accepted == expect_accept, "authority case " + String(case.get("name", "")))

	print("SERVER_CONTRACT_FIXTURES_PASS")
	quit(0)
