package data_scope

import "testing"

func TestValidateBusinessIdentifierRejectsInjection(t *testing.T) {
	for _, value := range []string{"user.id", "user`id", "user id", "user/*x*/", "user#x", ""} {
		if err := ValidateBusinessIdentifier(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestValidateBusinessIdentifierAcceptsPlainName(t *testing.T) {
	for _, value := range []string{"user", "orders_2026", "_tenant_data"} {
		if err := ValidateBusinessIdentifier(value); err != nil {
			t.Fatalf("expected %q to be accepted: %v", value, err)
		}
	}
}

func TestStaticPolicyRejectsSensitiveFields(t *testing.T) {
	for _, field := range []string{"admin_id", "password", "salt", "token", "secret"} {
		if err := ValidateSecurityField(field); err == nil {
			t.Fatalf("expected sensitive field %q to be rejected", field)
		}
	}
}

func TestCustomPrimaryKeyIdentifierIsAllowed(t *testing.T) {
	for _, field := range []string{"order_no", "uuid"} {
		if err := ValidateSecurityField(field); err != nil {
			t.Fatalf("custom primary key %q rejected: %v", field, err)
		}
	}
}
