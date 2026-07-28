package handler

import (
	"net/http"
	"testing"

	"go-build-admin/app/middleware"
)

func TestCRUDCapabilitiesNormalizeRouteAndMethods(t *testing.T) {
	adminCapabilities := CRUDCapabilities("auth.Admin")
	if len(adminCapabilities) != 3 {
		t.Fatalf("expected 3 capabilities, got %d", len(adminCapabilities))
	}
	if adminCapabilities[0].Route != "auth/admin" {
		t.Fatalf("expected normalized route auth/admin, got %q", adminCapabilities[0].Route)
	}

	want := []middleware.AtomicRoute{
		{Route: "countrylanguage", Action: "add", Method: http.MethodPost},
		{Route: "countrylanguage", Action: "edit", Method: http.MethodPost},
		{Route: "countrylanguage", Action: "del", Method: http.MethodDelete},
	}
	got := CRUDCapabilities("countryLanguage")
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("capability %d: expected %#v, got %#v", i, want[i], got[i])
		}
	}
}
