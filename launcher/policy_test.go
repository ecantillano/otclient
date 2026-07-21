package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidManifestHostAndSchemaAreRejected(t *testing.T) {
	fixture := newReleaseServer(t)
	component := fixture.component(t, "core", "1.0.0", map[string]string{"bin/client": "client"})
	manifest := fixture.baseManifest("0.1.0", []Component{component})

	manifest.Components[0].URL = "https://evil.example/core.zip"
	if err := manifest.Validate("stable", LauncherVersion, []string{"127.0.0.1", "login.thappy.cl"}); err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("expected disallowed component host, got %v", err)
	}
	if err := ValidateHTTPSURL("http://127.0.0.1/manifest.json", []string{"127.0.0.1"}); err == nil {
		t.Fatal("expected HTTP manifest URL to be rejected")
	}
	if _, err := DecodeManifest(strings.NewReader(`{"schema_version":1,"unexpected":true}`)); err == nil {
		t.Fatal("expected unknown manifest field to be rejected")
	}
}

func TestManifestCarriesProtocolAndEndpointPolicy(t *testing.T) {
	fixture := newReleaseServer(t)
	component := fixture.component(t, "core", "1.0.0", map[string]string{"bin/client": "client"})
	manifest := fixture.baseManifest("0.1.0", []Component{component})
	if err := manifest.Validate("stable", LauncherVersion, []string{"127.0.0.1", "login.thappy.cl"}); err != nil {
		t.Fatal(err)
	}

	manifest.ProtocolVersion = 1098
	if err := manifest.Validate("stable", LauncherVersion, []string{"127.0.0.1", "login.thappy.cl"}); err == nil || !strings.Contains(err.Error(), "requires 1525") {
		t.Fatalf("expected protocol rejection, got %v", err)
	}
	manifest.ProtocolVersion = SupportedProtocolVersion
	manifest.MinimumLauncherVersion = "99.0.0"
	if err := manifest.Validate("stable", LauncherVersion, []string{"127.0.0.1", "login.thappy.cl"}); err == nil || !strings.Contains(err.Error(), "update Thappy Launcher manually") {
		t.Fatalf("expected clear minimum launcher error, got %v", err)
	}
}

func TestOfflineLaunchUsesCachedMandatoryPolicy(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		mandatory  bool
		wantLaunch bool
	}{
		{name: "optional release", mandatory: false, wantLaunch: true},
		{name: "mandatory release", mandatory: true, wantLaunch: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newReleaseServer(t)
			updater := testUpdater(t, fixture)
			policy := &CachedPolicy{
				Channel: "stable", Version: "0.1.1", ProtocolVersion: SupportedProtocolVersion,
				AssetVersion: "1525", ReleaseNotesURL: fixture.server.URL + "/notes",
				Mandatory: testCase.mandatory, Environment: "production", LoginURL: ProductionLoginURL,
				LoginPort: ProductionLoginPort, Components: map[string]string{"core": "1.0.1"},
			}
			if err := SaveState(updater.statePath(), State{
				SchemaVersion: StateSchemaVersion, Channel: "stable", InstalledVersion: "0.1.0",
				LastGoodVersion: "0.1.0", Components: map[string]string{"core": "1.0.0"}, LastManifest: policy,
			}); err != nil {
				t.Fatal(err)
			}
			writeInstallFile(t, updater, "bin/client", "installed")
			fixture.server.Close()
			launched := false
			runner := &Runner{
				Updater: updater, Channel: "stable", ManifestURL: fixture.server.URL + "/manifest.json",
				ClientExecutable: "bin/client", launch: func(string, []string) error { launched = true; return nil },
			}
			result, err := runner.Run(context.Background())
			if testCase.wantLaunch {
				if err != nil || !result.Offline || !launched {
					t.Fatalf("offline client did not launch: result=%+v err=%v", result, err)
				}
			} else {
				if err == nil || launched || !strings.Contains(err.Error(), "mandatory release 0.1.1") {
					t.Fatalf("mandatory offline policy not enforced: launched=%v err=%v", launched, err)
				}
			}
		})
	}
}

func TestLauncherLockRejectsConcurrentRun(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "launcher.lock")
	first, err := AcquireFileLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := AcquireFileLock(lockPath)
	if second != nil {
		_ = second.Close()
	}
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("expected ErrLocked, got %v", err)
	}
}

func TestMandatoryManifestRequiringManualLauncherUpdateBlocksOffline(t *testing.T) {
	fixture := newReleaseServer(t)
	component := fixture.component(t, "core", "1.0.1", map[string]string{"bin/client": "new"})
	fixture.manifest = fixture.baseManifest("0.1.1", []Component{component})
	fixture.manifest.Mandatory = true
	fixture.manifest.MinimumLauncherVersion = "99.0.0"
	updater := testUpdater(t, fixture)
	saveInstalledState(t, updater, "0.1.0", map[string]string{"core": "1.0.0"})
	writeInstallFile(t, updater, "bin/client", "old")
	launched := false
	runner := &Runner{
		Updater: updater, Channel: "stable", ManifestURL: fixture.server.URL + "/manifest.json",
		ClientExecutable: "bin/client", launch: func(string, []string) error { launched = true; return nil },
	}
	if _, err := runner.Run(context.Background()); err == nil || launched || !strings.Contains(err.Error(), "mandatory release 0.1.1") || !strings.Contains(err.Error(), "update Thappy Launcher manually") {
		t.Fatalf("incompatible mandatory release did not fail closed: launched=%v err=%v", launched, err)
	}
	state, err := LoadState(updater.statePath())
	if err != nil {
		t.Fatal(err)
	}
	if state.LastManifest == nil || !state.LastManifest.Mandatory || state.LastManifest.MinimumLauncherVersion != "99.0.0" {
		t.Fatalf("mandatory policy was not cached: %+v", state.LastManifest)
	}
}

func TestDiagnosticRedactsURLsAndReportsReleasePolicy(t *testing.T) {
	fixture := newReleaseServer(t)
	updater := testUpdater(t, fixture)
	policy := &CachedPolicy{
		Channel: "stable", Version: "0.1.1", ProtocolVersion: SupportedProtocolVersion,
		AssetVersion: "1525", ReleaseNotesURL: fixture.server.URL + "/notes?token=secret",
		Mandatory: true, Environment: "production", LoginURL: ProductionLoginURL, LoginPort: ProductionLoginPort,
		Components: map[string]string{"core": "1.0.1"},
	}
	if err := SaveState(updater.statePath(), State{
		SchemaVersion: StateSchemaVersion, InstalledVersion: "0.1.0", LastGoodVersion: "0.1.0",
		Components: map[string]string{"core": "1.0.0"}, LastManifest: policy,
	}); err != nil {
		t.Fatal(err)
	}
	report := BuildDiagnostic(updater, "stable", fixture.server.URL+"/manifest.json?token=secret", "bin/client")
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if strings.Contains(encoded, "secret") || !strings.Contains(encoded, "redacted") {
		t.Fatalf("diagnostic leaked query data: %s", encoded)
	}
	if report.ProtocolVersion != SupportedProtocolVersion || report.AssetVersion != "1525" || !report.Mandatory || report.LoginURL != ProductionLoginURL || report.LoginPort != 443 || report.HTTPLogin || report.UseAuthenticator {
		t.Fatalf("diagnostic omitted release policy: %+v", report)
	}
}

func TestPendingNoLaunchIsRolledBackOnRestart(t *testing.T) {
	fixture := newReleaseServer(t)
	component := fixture.component(t, "core", "1.0.1", map[string]string{"bin/client": "new"})
	fixture.manifest = fixture.baseManifest("0.1.1", []Component{component})
	updater := testUpdater(t, fixture)
	saveInstalledState(t, updater, "0.1.0", map[string]string{"core": "1.0.0"})
	writeInstallFile(t, updater, "bin/client", "old")
	result, err := updater.Update(context.Background(), "stable", fixture.server.URL+"/manifest.json")
	if err != nil || !result.PendingConfirmation {
		t.Fatalf("create pending update: result=%+v err=%v", result, err)
	}

	restarted, err := NewUpdater(updater.InstallDir, updater.Config, fixture.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	restarted.Platform = "linux"
	recovered, err := restarted.Recover()
	if err != nil || !recovered {
		t.Fatalf("recover pending update: recovered=%v err=%v", recovered, err)
	}
	if got := readInstallFile(t, restarted, "bin/client"); got != "old" {
		t.Fatalf("unconfirmed update survived restart: %q", got)
	}
	if _, err := os.Stat(restarted.journalPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pending journal survived recovery: %v", err)
	}
}

func TestDiagnosticLogIsPersistentAndRedacted(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".thappy-launcher")
	home, _ := os.UserHomeDir()
	message := "fetch https://cdn.example/manifest?token=secret from " + home
	if err := AppendDiagnosticLog(stateRoot, "update_failed", message); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(stateRoot, "logs", "launcher.log"))
	if err != nil {
		t.Fatal(err)
	}
	logged := string(data)
	if strings.Contains(logged, "secret") || (home != "" && strings.Contains(logged, home)) || !strings.Contains(logged, "redacted") {
		t.Fatalf("log was not redacted: %s", logged)
	}
}

func TestShippedExamplesValidate(t *testing.T) {
	config, err := LoadConfig("launcher-config.example.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		file    string
		channel string
	}{{"manifest-stable.example.json", "stable"}, {"manifest-test.example.json", "test"}} {
		file, err := os.Open(fixture.file)
		if err != nil {
			t.Fatal(err)
		}
		manifest, decodeErr := DecodeManifest(file)
		_ = file.Close()
		if decodeErr != nil {
			t.Fatalf("%s: %v", fixture.file, decodeErr)
		}
		if err := manifest.Validate(fixture.channel, LauncherVersion, config.AllowedHosts); err != nil {
			t.Fatalf("%s: %v", fixture.file, err)
		}
	}
}
