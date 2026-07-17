package local

import (
	"go-build-admin/database/migrations/internal/core"
)

func officialKeysThrough(official []core.OfficialMigration, version int64) []core.OfficialKey {
	keys := make([]core.OfficialKey, 0, len(official))
	for _, migration := range official {
		if migration.Key.Version <= version {
			keys = append(keys, migration.Key)
		}
	}
	return keys
}

func Migrations(official []core.OfficialMigration) []core.LocalMigration {
	postSeed := localPostSeedVerify
	return []core.LocalMigration{
		{Sequence: 1, ID: "account-status-protocol", Revision: 1, RequiresOfficial: officialKeysThrough(official, 20250412134127), LegacyAliases: []core.OfficialKey{{20260714120000, "Version223"}}, Up: local0001Up, VerifySchema: verifyStatusContract, VerifyUpgradeData: verifyStatusContract, PostSeedVerify: postSeed},
		{Sequence: 2, ID: "admin-hierarchy", Revision: 1, RequiresOfficial: officialKeysThrough(official, 20250412134127), LegacyAliases: []core.OfficialKey{{20260714130000, "Version224"}}, Up: version224, VerifySchema: verifyHierarchyContract, VerifyUpgradeData: verifyHierarchyContract, PostSeedVerify: postSeed},
		{Sequence: 3, ID: "attachment-owner-index", Revision: 1, RequiresOfficial: officialKeysThrough(official, 20250412134127), LegacyAliases: []core.OfficialKey{{20260715000000, "Version225"}}, Up: version225, VerifySchema: verifyAttachmentContract, VerifyUpgradeData: verifyAttachmentContract, PostSeedVerify: postSeed},
		{Sequence: 4, ID: "user-ownership", Revision: 1, RequiresOfficial: officialKeysThrough(official, 20250412134127), LegacyAliases: []core.OfficialKey{{20260716000000, "Version226"}}, Up: version226, VerifySchema: verifyUserOwnerContract, VerifyUpgradeData: verifyUserOwnerContract, PostSeedVerify: postSeed},
		{Sequence: 5, ID: "security-ownership", Revision: 1, RequiresOfficial: officialKeysThrough(official, 20250412134127), LegacyAliases: []core.OfficialKey{{20260717000000, "Version227"}}, Up: version227, VerifySchema: verifySecurityOwnerContract, VerifyUpgradeData: verifySecurityOwnerContract, PostSeedVerify: postSeed},
		{Sequence: 6, ID: "signed-balance-deltas", Revision: 1, RequiresOfficial: officialKeysThrough(official, 20250412134127), LegacyAliases: []core.OfficialKey{{20260718000000, "Version228"}}, Up: version228, VerifySchema: verifySignedDeltaContract, VerifyUpgradeData: verifySignedDeltaContract, PostSeedVerify: postSeed},
		{Sequence: 7, ID: "security-target-owner", Revision: 1, RequiresOfficial: officialKeysThrough(official, 20250412134127), LegacyAliases: []core.OfficialKey{{20260719000000, "Version229"}}, Up: version229, VerifySchema: verifyTargetContract, VerifyUpgradeData: verifyTargetContract, PostSeedVerify: postSeed},
		{Sequence: 8, ID: "legacy-target-state", Revision: 1, RequiresOfficial: officialKeysThrough(official, 20250412134127), LegacyAliases: []core.OfficialKey{{20260720000000, "Version230"}}, Up: version230, VerifySchema: verifyLegacyTargetContract, VerifyUpgradeData: verifyLegacyTargetContract, PostSeedVerify: postSeed},
		{Sequence: 9, ID: "security-commit-state", Revision: 1, RequiresOfficial: officialKeysThrough(official, 20250412134127), LegacyAliases: []core.OfficialKey{{20260721000000, "Version231"}}, Up: version231, VerifySchema: verifyCommitContract, VerifyUpgradeData: verifyCommitContract, PostSeedVerify: postSeed},
		{Sequence: 10, ID: "security-rule-normalization", Revision: 1, RequiresOfficial: officialKeysThrough(official, 20250412134127), LegacyAliases: []core.OfficialKey{{20260722000000, "Version232"}}, Up: version232, VerifySchema: verifySecurityRuleContract, VerifyUpgradeData: verifySecurityRuleContract, PostSeedVerify: postSeed},
	}
}
