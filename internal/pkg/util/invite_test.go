package util

import (
	"strings"
	"testing"
)

func TestInviteCodeDeterministic(t *testing.T) {
	secret := "test-secret"
	id := int32(42)
	first := InviteCode(secret, id)
	second := InviteCode(secret, id)
	if first != second {
		t.Fatalf("invite code must be deterministic: got %q then %q", first, second)
	}
	if len(first) != 6 {
		t.Fatalf("invite code must be 6 chars, got %d (%q)", len(first), first)
	}
}

func TestInviteCodeDistinctPerAdmin(t *testing.T) {
	secret := "test-secret"
	seen := make(map[string]struct{})
	for id := int32(1); id <= 100; id++ {
		code := InviteCode(secret, id)
		if _, dup := seen[code]; dup {
			t.Fatalf("collision for admin id %d: %q", id, code)
		}
		seen[code] = struct{}{}
	}
}

func TestInviteCodeAlphabet(t *testing.T) {
	secret := "test-secret"
	for id := int32(1); id <= 50; id++ {
		code := InviteCode(secret, id)
		for _, c := range code {
			if !strings.ContainsRune(inviteAlphabet, c) {
				t.Fatalf("invite code %q contains invalid char %q", code, c)
			}
		}
	}
}

func TestInviteCodeSecretMatters(t *testing.T) {
	a := InviteCode("secret-a", 1)
	b := InviteCode("secret-b", 1)
	if a == b {
		t.Fatalf("different secrets must yield different codes, both %q", a)
	}
}
