package official

import (
	"go-build-admin/database/migrations/internal/core"
)

var migrations = []core.OfficialMigration{
	{Key: core.OfficialKey{Version: 20230622221507, Name: "Version200"}, Source: "BuildAdmin:20230622221507:Version200", Up: version200},
	{Key: core.OfficialKey{Version: 20230719211338, Name: "Version201"}, Source: "BuildAdmin:20230719211338:Version201", Up: version201},
	{Key: core.OfficialKey{Version: 20230905060702, Name: "Version202"}, Source: "BuildAdmin:20230905060702:Version202", Up: version202},
	{Key: core.OfficialKey{Version: 20231112093414, Name: "Version205"}, Source: "BuildAdmin:20231112093414:Version205", Up: version205},
	{Key: core.OfficialKey{Version: 20231229043002, Name: "Version206"}, Source: "BuildAdmin:20231229043002:Version206", Up: version206},
	{Key: core.OfficialKey{Version: 20250412134127, Name: "Version222"}, Source: "BuildAdmin:20250412134127:Version222", Up: version222},
}

func Migrations() []core.OfficialMigration {
	return append([]core.OfficialMigration(nil), migrations...)
}
