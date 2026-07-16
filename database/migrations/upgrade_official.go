package migrations

// officialMigrations is the only registry allowed to write the legacy
// <prefix>migrations ledger. The Source values are the upstream BuildAdmin
// release identities retained by the project.
var officialMigrations = []OfficialMigration{
	{Key: OfficialKey{Version: 20230622221507, Name: "Version200"}, Source: "BuildAdmin:20230622221507:Version200", Up: version200},
	{Key: OfficialKey{Version: 20230719211338, Name: "Version201"}, Source: "BuildAdmin:20230719211338:Version201", Up: version201},
	{Key: OfficialKey{Version: 20230905060702, Name: "Version202"}, Source: "BuildAdmin:20230905060702:Version202", Up: version202},
	{Key: OfficialKey{Version: 20231112093414, Name: "Version205"}, Source: "BuildAdmin:20231112093414:Version205", Up: version205},
	{Key: OfficialKey{Version: 20231229043002, Name: "Version206"}, Source: "BuildAdmin:20231229043002:Version206", Up: version206},
	{Key: OfficialKey{Version: 20250412134127, Name: "Version222"}, Source: "BuildAdmin:20250412134127:Version222", Up: version222},
}

func OfficialMigrations() []OfficialMigration {
	return append([]OfficialMigration(nil), officialMigrations...)
}
