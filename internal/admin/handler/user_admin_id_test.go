package handler

import "testing"

func TestRequestedAdminIDProtocol(t *testing.T) {
	if id, present, err := requestedAdminID([]byte(`{"username":"u"}`)); err != nil || present || id != 0 {
		t.Fatalf("missing admin_id = %d/%v/%v", id, present, err)
	}
	if id, present, err := requestedAdminID([]byte(`{"admin_id":12}`)); err != nil || !present || id != 12 {
		t.Fatalf("valid admin_id = %d/%v/%v", id, present, err)
	}
	for _, body := range []string{`{"admin_id":0}`, `{"admin_id":-1}`, `{"admin_id":null}`, `{"admin_id":1.5}`, `{"admin_id":"12"}`} {
		if _, _, err := requestedAdminID([]byte(body)); err == nil {
			t.Fatalf("invalid admin_id accepted: %s", body)
		}
	}
}
