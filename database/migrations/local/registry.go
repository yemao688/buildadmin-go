package local

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
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
	requiresOfficial := officialKeysThrough(official, 20250412134127)
	return []core.LocalMigration{
		{Sequence: 1, ID: "account-status-protocol", Revision: 1, RequiresOfficial: requiresOfficial, Up: local0001Up, VerifySchema: verifyStatusContract, VerifyUpgradeData: verifyStatusContract},
		{Sequence: 2, ID: "admin-hierarchy", Revision: 1, RequiresOfficial: requiresOfficial, Up: version224, VerifySchema: verifyHierarchyContract, VerifyUpgradeData: verifyHierarchyContract},
		{Sequence: 3, ID: "ownership-and-audit-integrity", Revision: 1, RequiresOfficial: requiresOfficial, Up: ownershipAndAuditIntegrity, VerifySchema: verifyOwnershipAndAuditIntegrity, VerifyUpgradeData: verifyOwnershipAndAuditIntegrity},
		{Sequence: 4, ID: "security-rule-normalization", Revision: 1, RequiresOfficial: requiresOfficial, Up: securityRuleNormalization, VerifySchema: verifySecurityRuleContract, VerifyUpgradeData: verifySecurityRuleContract},
		{Sequence: 5, ID: "country-dictionary", Revision: 1, RequiresOfficial: requiresOfficial, Up: version0013, VerifySchema: verifyCountryDictionaryContract, VerifyUpgradeData: verifyCountryDictionaryContract},
		{Sequence: 6, ID: "upload-config", Revision: 1, RequiresOfficial: requiresOfficial, Up: version0014},
	}
}

func ownershipAndAuditIntegrity(db *gorm.DB, config *conf.Configuration) error {
	for _, migration := range []func(*gorm.DB, *conf.Configuration) error{version225, version226, version227, version228, version229, version230, version231, version0012} {
		if err := migration(db, config); err != nil {
			return err
		}
	}
	return normalizeFreshOwnership(db, config)
}

func verifyOwnershipAndAuditIntegrity(db *gorm.DB, config *conf.Configuration) error {
	for _, verify := range []func(*gorm.DB, *conf.Configuration) error{verifyAttachmentContract, verifyUserOwnerContract, verifySecurityOwnerContract, verifySignedDeltaContract, verifyTargetContract, verifyLegacyTargetContract, verifyCommitContract, verifyOwnerColumnContract} {
		if err := verify(db, config); err != nil {
			return err
		}
	}
	return nil
}

func securityRuleNormalization(db *gorm.DB, config *conf.Configuration) error {
	if err := version232(db, config); err != nil {
		return err
	}
	recycle := core.TableName(config, "security_data_recycle")
	if core.TableExists(db, recycle) {
		if err := convergeSecurityRule(db, recycle, 1, 5, "会员", "user/User.php", "user", "id", "user/user", "auth/user", ""); err != nil {
			return err
		}
	}
	sensitive := core.TableName(config, "security_sensitive_data")
	if core.TableExists(db, sensitive) {
		if err := convergeSecurityRule(db, sensitive, 1, 2, "会员数据", "user/User.php", "user", "id", "user/user", "auth/user", `{"username":"用户名","mobile":"手机号","status":"状态","email":"邮箱地址"}`); err != nil {
			return err
		}
	}
	return nil
}
