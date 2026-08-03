package service

import (
	"testing"

	"buildadmin-go/internal/pkg/data_scope"

	cErr "buildadmin-go/internal/pkg/error"
)

func ptr(v int32) *int32 { return &v }

func TestResolveParentIDForAdd(t *testing.T) {
	restricted := data_scope.Actor{AdminID: 5}
	unrestricted := data_scope.Actor{AdminID: 1, Unrestricted: true}

	cases := []struct {
		name    string
		p       ParentSelection
		actor   data_scope.Actor
		want    *int32
		wantErr bool
	}{
		{"restricted omitted defaults to actor", ParentSelection{}, restricted, ptr(5), false},
		{"restricted null defaults to actor", ParentSelection{Set: true}, restricted, ptr(5), false},
		{"restricted 0 defaults to actor", ParentSelection{Set: true, Value: ptr(0)}, restricted, ptr(5), false},
		{"unrestricted omitted is root", ParentSelection{}, unrestricted, nil, false},
		{"unrestricted 0 is root", ParentSelection{Set: true, Value: ptr(0)}, unrestricted, nil, false},
		{"explicit positive kept", ParentSelection{Set: true, Value: ptr(7)}, restricted, ptr(7), false},
		{"negative rejected", ParentSelection{Set: true, Value: ptr(-1)}, restricted, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := (&AdminService{}).ResolveParentIDForAdd(tc.p, tc.actor)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !int32PtrEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveParentIDForEdit(t *testing.T) {
	restricted := data_scope.Actor{AdminID: 5}
	unrestricted := data_scope.Actor{AdminID: 1, Unrestricted: true}

	cases := []struct {
		name        string
		p           ParentSelection
		current     *int32
		actor       data_scope.Actor
		want        *int32
		wantChanged bool
		wantErr     bool
	}{
		{"omitted unchanged", ParentSelection{}, ptr(3), restricted, ptr(3), false, false},
		{"null unchanged", ParentSelection{Set: true}, ptr(3), restricted, ptr(3), false, false},
		{"restricted cannot root", ParentSelection{Set: true, Value: ptr(0)}, ptr(3), restricted, nil, false, true},
		{"unrestricted can root", ParentSelection{Set: true, Value: ptr(0)}, ptr(3), unrestricted, nil, true, false},
		{"unrestricted root no-op", ParentSelection{Set: true, Value: ptr(0)}, nil, unrestricted, nil, false, false},
		{"positive changed", ParentSelection{Set: true, Value: ptr(7)}, ptr(3), restricted, ptr(7), true, false},
		{"positive unchanged", ParentSelection{Set: true, Value: ptr(3)}, ptr(3), restricted, ptr(3), false, false},
		{"negative rejected", ParentSelection{Set: true, Value: ptr(-1)}, ptr(3), restricted, nil, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := (&AdminService{}).ResolveParentIDForEdit(tc.p, tc.current, tc.actor)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !int32PtrEqual(got, tc.want) {
				t.Fatalf("parent got %v, want %v", got, tc.want)
			}
			if changed != tc.wantChanged {
				t.Fatalf("changed=%v, want %v", changed, tc.wantChanged)
			}
		})
	}
}

func TestIsMovingUnderSelf(t *testing.T) {
	svc := AdminService{}
	if !svc.IsMovingUnderSelf(10, ptr(10)) {
		t.Error("moving node 10 under itself should be detected")
	}
	if svc.IsMovingUnderSelf(10, nil) {
		t.Error("nil parent is not a self-move")
	}
	if svc.IsMovingUnderSelf(10, ptr(5)) {
		t.Error("different parent is not a self-move")
	}
	// New administrator has no ID yet; creating under the current actor is valid.
	if svc.IsMovingUnderSelf(0, ptr(7)) {
		t.Error("new admin under actor 7 must not be treated as self-move")
	}
}

func TestValidateAccountStatusValue(t *testing.T) {
	for _, value := range []any{"enable", "disable"} {
		if err := ValidateAccountStatusValue(value); err != nil {
			t.Fatalf("%v should be accepted: %v", value, err)
		}
	}
	for _, value := range []any{"0", "1", "", "bad", 0, 1, nil} {
		err := ValidateAccountStatusValue(value)
		if err == nil {
			t.Fatalf("%v should be rejected", value)
		}
		if _, ok := err.(*cErr.Error); !ok {
			t.Fatalf("%v error type = %T", value, err)
		}
	}
}
