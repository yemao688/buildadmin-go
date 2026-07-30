package model

import "fmt"

func ValidateRuleIdentityChange(hasLogs bool, currentPK, currentOwner, newPK, newOwner string) error {
	if currentPK == "" {
		currentPK = "id"
	}
	if currentOwner == "" {
		currentOwner = "admin_id"
	}
	if newPK == "" {
		newPK = "id"
	}
	if newOwner == "" {
		newOwner = "admin_id"
	}
	if hasLogs && (currentPK != newPK || currentOwner != newOwner) {
		return fmt.Errorf("cannot change owner_column or primary_key after security logs exist")
	}
	return nil
}

func validateRuleIdentityChange(hasLogs bool, currentPK, currentOwner, newPK, newOwner string) error {
	return ValidateRuleIdentityChange(hasLogs, currentPK, currentOwner, newPK, newOwner)
}
