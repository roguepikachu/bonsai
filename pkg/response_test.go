package pkg

import "testing"

func TestNewResponse(t *testing.T) {
	r := NewResponse(201, map[string]string{"ok": "y"}, "created")
	if r.Code != 201 || r.Message != "created" {
		t.Fatalf("mismatch: %+v", r)
	}
	m := r.Data.(map[string]string)
	if m["ok"] != "y" {
		t.Fatalf("data mismatch: %+v", r.Data)
	}
}
