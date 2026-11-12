package migrations

import "testing"

func TestMigrationRegistry(t *testing.T) {
	official := OfficialMigrations()
	if err := ValidateOfficialMigrations(official); err != nil {
		t.Fatal(err)
	}
	want := []int64{20230622221507, 20230719211338, 20230905060702, 20231112093414, 20231229043002, 20250412134127}
	if len(official) != len(want) {
		t.Fatalf("migration count = %d", len(official))
	}
	for i, v := range want {
		if official[i].Key.Version != v {
			t.Fatalf("migration %d = %d, want %d", i, official[i].Key.Version, v)
		}
	}
}

func TestSeedMarker(t *testing.T) {
	if installDataVersion != 20230620180916 || installDataName != "InstallData" {
		t.Fatal("unexpected install marker")
	}
}
