package password

import "testing"

func TestHashAndCompare(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if Compare(hash, "correct horse battery staple") != nil {
		t.Fatal("hashed password did not compare")
	}
	if Compare(hash, "wrong password") == nil {
		t.Fatal("wrong password compared successfully")
	}
}
