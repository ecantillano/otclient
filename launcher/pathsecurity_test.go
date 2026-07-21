package launcher

import "testing"

func TestSafeRelativePath(t *testing.T) {
	valid, err := SafeRelativePath(`modules\game_interface/main.lua`)
	if err != nil || valid != "modules/game_interface/main.lua" {
		t.Fatalf("unexpected normalized path %q: %v", valid, err)
	}
	for _, value := range []string{"../escape", "/absolute", `C:\\client`, "data/../../escape", "NUL.txt", "file:stream", "trailing./file"} {
		t.Run(value, func(t *testing.T) {
			if _, err := SafeRelativePath(value); err == nil {
				t.Fatalf("expected %q to be rejected", value)
			}
		})
	}
}

func TestDeletePolicyPreservesUserData(t *testing.T) {
	if _, err := buildOperations(nil, []string{"settings/account.otml"}, []string{"settings/"}, []string{"settings/"}); err == nil {
		t.Fatal("expected preserved data deletion to be rejected")
	}
	if _, err := buildOperations(nil, []string{"unknown/file"}, []string{"data/"}, nil); err == nil {
		t.Fatal("expected delete outside local allowlist to be rejected")
	}
}
