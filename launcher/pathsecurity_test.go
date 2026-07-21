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

func TestDefaultDeletePolicyRejectsBroadRoots(t *testing.T) {
	for _, forbidden := range []string{"bin/", "data/", "modules/", "mods/"} {
		config := DefaultConfig()
		config.DeleteAllowlist = append(config.DeleteAllowlist, forbidden)
		if err := config.Validate(); err == nil {
			t.Fatalf("expected broad delete rule %q to be rejected", forbidden)
		}
	}

	config := DefaultConfig()
	if err := config.Validate(); err != nil {
		t.Fatalf("default delete policy must remain valid: %v", err)
	}
}

func TestUpdateCannotReplaceLauncherExecutable(t *testing.T) {
	preserved := append(DefaultConfig().PreservePaths, "ThappyLauncher.exe", "thappy-launcher")
	for _, executable := range []string{"ThappyLauncher.exe", "thappy-launcher"} {
		files := []ExtractedFile{{RelativePath: executable, SourcePath: "unused"}}
		if _, err := buildOperations(files, nil, nil, preserved); err == nil {
			t.Fatalf("expected launcher replacement %q to be rejected", executable)
		}
	}
}

func TestConfigCannotWeakenMandatoryPreservePolicy(t *testing.T) {
	for _, paths := range [][]string{nil, {"config.otml"}} {
		config := DefaultConfig()
		config.PreservePaths = paths
		if err := config.Validate(); err == nil {
			t.Fatalf("expected incomplete preserve policy %v to be rejected", paths)
		}
	}
}
