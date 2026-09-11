package transport

import "testing"

func TestEquipmentGodotNumbers(t *testing.T) {
	var request equipmentReq
	if !decode([]byte(`{"fencing_token":1.0,"expected_revision":2.0}`), &request) || !integer(request.FencingToken) || !integer(request.ExpectedRevision) {
		t.Fatal("integral Godot JSON numbers rejected")
	}
	if !decode([]byte(`{"fencing_token":1.5}`), &request) || integer(request.FencingToken) {
		t.Fatal("fractional fencing token accepted")
	}
}
