package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type releaseServer struct {
	server       *httptest.Server
	manifest     Manifest
	archives     map[string][]byte
	downloadHits map[string]*atomic.Int32
}

func newReleaseServer(t *testing.T) *releaseServer {
	t.Helper()
	fixture := &releaseServer{archives: make(map[string][]byte), downloadHits: make(map[string]*atomic.Int32)}
	fixture.server = httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/manifest.json" {
			response.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(response).Encode(fixture.manifest); err != nil {
				t.Errorf("encode manifest: %v", err)
			}
			return
		}
		if archive, ok := fixture.archives[request.URL.Path]; ok {
			fixture.downloadHits[request.URL.Path].Add(1)
			response.Header().Set("Content-Type", "application/zip")
			_, _ = response.Write(archive)
			return
		}
		if request.URL.Path == "/notes" {
			_, _ = response.Write([]byte("release notes"))
			return
		}
		http.NotFound(response, request)
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func (fixture *releaseServer) component(t *testing.T, name, version string, files map[string]string) Component {
	t.Helper()
	archive := makeZIP(t, files)
	path := "/" + name + ".zip"
	fixture.archives[path] = archive
	fixture.downloadHits[path] = &atomic.Int32{}
	hash := sha256.Sum256(archive)
	return Component{
		Name: name, Version: version, Platforms: []string{"linux"}, URL: fixture.server.URL + path,
		SHA256: hex.EncodeToString(hash[:]), Size: int64(len(archive)), Archive: "zip",
	}
}

func (fixture *releaseServer) baseManifest(version string, components []Component) Manifest {
	return Manifest{
		SchemaVersion: ManifestSchemaVersion, Channel: "stable", Version: version,
		ProtocolVersion: SupportedProtocolVersion, AssetVersion: "1525",
		ReleaseNotesURL: fixture.server.URL + "/notes", Mandatory: false, Environment: "production",
		LoginURL: ProductionLoginURL, LoginPort: ProductionLoginPort, HTTPLogin: false, UseAuthenticator: false,
		Components: components,
	}
}

func makeZIP(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for name, contents := range files {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o755)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func releaseConfig(serverURL string) Config {
	config := DefaultConfig()
	config.AllowedHosts = []string{"127.0.0.1", "login.thappy.cl"}
	config.Channels["stable"] = ChannelConfig{ManifestURL: serverURL + "/manifest.json"}
	config.Channels["test"] = ChannelConfig{ManifestURL: serverURL + "/manifest.json"}
	config.DeleteAllowlist = append(config.DeleteAllowlist, "bin/obsolete")
	return config
}

func testUpdater(t *testing.T, fixture *releaseServer) *Updater {
	t.Helper()
	updater, err := NewUpdater(t.TempDir(), releaseConfig(fixture.server.URL), fixture.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	updater.Platform = "linux"
	updater.Fetcher.maxAttempts = 1
	return updater
}

func saveInstalledState(t *testing.T, updater *Updater, version string, components map[string]string) {
	t.Helper()
	if err := SaveState(updater.statePath(), State{
		SchemaVersion: StateSchemaVersion, Channel: "stable", InstalledVersion: version,
		LastGoodVersion: version, Components: components,
	}); err != nil {
		t.Fatal(err)
	}
}

func writeInstallFile(t *testing.T, updater *Updater, relative, contents string) {
	t.Helper()
	path, err := secureJoin(updater.InstallDir, relative)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readInstallFile(t *testing.T, updater *Updater, relative string) string {
	t.Helper()
	path, err := secureJoin(updater.InstallDir, relative)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestUpdate010To011DownloadsOnlyChangedComponentsAndConfirms(t *testing.T) {
	fixture := newReleaseServer(t)
	core := fixture.component(t, "core", "1.0.0", map[string]string{"bin/core": "unchanged package"})
	assets := fixture.component(t, "assets", "1.0.1", map[string]string{"data/client.dat": "new assets"})
	fixture.manifest = fixture.baseManifest("0.1.1", []Component{core, assets})
	fixture.manifest.Delete = []string{"bin/obsolete"}

	updater := testUpdater(t, fixture)
	var progress []ProgressEvent
	updater.Progress = func(event ProgressEvent) { progress = append(progress, event) }
	saveInstalledState(t, updater, "0.1.0", map[string]string{"core": "1.0.0", "assets": "1.0.0"})
	writeInstallFile(t, updater, "data/client.dat", "old assets")
	writeInstallFile(t, updater, "bin/obsolete", "remove me")
	writeInstallFile(t, updater, "settings/account.otml", "keep me")

	result, err := updater.Update(context.Background(), "stable", fixture.server.URL+"/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != UpdateApplied || !result.PendingConfirmation || result.InstalledVersion != "0.1.1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if hits := fixture.downloadHits["/core.zip"].Load(); hits != 0 {
		t.Fatalf("unchanged component downloaded %d times", hits)
	}
	if hits := fixture.downloadHits["/assets.zip"].Load(); hits != 1 {
		t.Fatalf("changed component downloaded %d times", hits)
	}
	if len(progress) < 4 || progress[0].Phase != "planned" || progress[0].TotalBytes != uint64(assets.Size) || progress[len(progress)-1].Phase != "pending_launch" {
		t.Fatalf("unexpected progress events: %+v", progress)
	}
	if got := readInstallFile(t, updater, "data/client.dat"); got != "new assets" {
		t.Fatalf("unexpected client data %q", got)
	}
	if _, err := os.Stat(filepath.Join(updater.InstallDir, "bin", "obsolete")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("obsolete file still exists: %v", err)
	}
	if got := readInstallFile(t, updater, "settings/account.otml"); got != "keep me" {
		t.Fatalf("preserved data changed: %q", got)
	}
	pending, err := LoadState(updater.statePath())
	if err != nil {
		t.Fatal(err)
	}
	if pending.LastGoodVersion != "0.1.0" {
		t.Fatalf("update became last-good before launch: %+v", pending)
	}
	if err := updater.ConfirmPending(); err != nil {
		t.Fatal(err)
	}
	confirmed, err := LoadState(updater.statePath())
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.LastGoodVersion != "0.1.1" || confirmed.Components["assets"] != "1.0.1" {
		t.Fatalf("update not confirmed: %+v", confirmed)
	}
	if _, err := os.Stat(updater.journalPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal was not cleaned: %v", err)
	}
}

func TestSameAppVersionUpdatesChangedComponent(t *testing.T) {
	fixture := newReleaseServer(t)
	assets := fixture.component(t, "assets", "1.0.1", map[string]string{"data/client.dat": "repaired"})
	fixture.manifest = fixture.baseManifest("0.1.0", []Component{assets})
	updater := testUpdater(t, fixture)
	saveInstalledState(t, updater, "0.1.0", map[string]string{"assets": "1.0.0"})
	writeInstallFile(t, updater, "data/client.dat", "old")

	result, err := updater.Update(context.Background(), "stable", fixture.server.URL+"/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != UpdateApplied || !result.PendingConfirmation {
		t.Fatalf("component repair was skipped: %+v", result)
	}
	if got := readInstallFile(t, updater, "data/client.dat"); got != "repaired" {
		t.Fatalf("component not repaired: %q", got)
	}
}

func TestInterruptedTransactionRecoversLastGoodFiles(t *testing.T) {
	fixture := newReleaseServer(t)
	component := fixture.component(t, "core", "1.0.1", map[string]string{"bin/client": "new", "data/assets": "new"})
	fixture.manifest = fixture.baseManifest("0.1.1", []Component{component})
	updater := testUpdater(t, fixture)
	saveInstalledState(t, updater, "0.1.0", map[string]string{"core": "1.0.0"})
	writeInstallFile(t, updater, "bin/client", "old")
	updater.applyHook = func(int) error { return errSimulatedInterruption }

	if _, err := updater.Update(context.Background(), "stable", fixture.server.URL+"/manifest.json"); !errors.Is(err, errSimulatedInterruption) {
		t.Fatalf("expected simulated interruption, got %v", err)
	}
	restarted, err := NewUpdater(updater.InstallDir, updater.Config, fixture.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	restarted.Platform = "linux"
	recovered, err := restarted.Recover()
	if err != nil || !recovered {
		t.Fatalf("recover: recovered=%v err=%v", recovered, err)
	}
	if got := readInstallFile(t, restarted, "bin/client"); got != "old" {
		t.Fatalf("last-good client was not restored: %q", got)
	}
	state, _ := LoadState(restarted.statePath())
	if state.InstalledVersion != "0.1.0" {
		t.Fatalf("state was not rolled back: %+v", state)
	}
}

func TestInvalidHashLeavesInstallationUntouched(t *testing.T) {
	fixture := newReleaseServer(t)
	component := fixture.component(t, "core", "1.0.1", map[string]string{"bin/client": "new"})
	component.SHA256 = strings.Repeat("0", 64)
	fixture.manifest = fixture.baseManifest("0.1.1", []Component{component})
	updater := testUpdater(t, fixture)
	saveInstalledState(t, updater, "0.1.0", map[string]string{"core": "1.0.0"})
	writeInstallFile(t, updater, "bin/client", "old")

	if _, err := updater.Update(context.Background(), "stable", fixture.server.URL+"/manifest.json"); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("expected SHA-256 failure, got %v", err)
	}
	if got := readInstallFile(t, updater, "bin/client"); got != "old" {
		t.Fatalf("invalid payload changed installation: %q", got)
	}
}

func TestZipTraversalIsRejected(t *testing.T) {
	fixture := newReleaseServer(t)
	component := fixture.component(t, "core", "1.0.1", map[string]string{"../escaped": "bad"})
	fixture.manifest = fixture.baseManifest("0.1.1", []Component{component})
	updater := testUpdater(t, fixture)
	saveInstalledState(t, updater, "0.1.0", map[string]string{"core": "1.0.0"})

	if _, err := updater.Update(context.Background(), "stable", fixture.server.URL+"/manifest.json"); err == nil || !strings.Contains(err.Error(), "unsafe ZIP entry") {
		t.Fatalf("expected unsafe ZIP rejection, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(updater.StateRoot, "staging", "escaped")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("traversal created a file: %v", err)
	}
}

func TestPostLaunchFailureRollsBackPendingUpdate(t *testing.T) {
	fixture := newReleaseServer(t)
	component := fixture.component(t, "core", "1.0.1", map[string]string{"bin/client": "new"})
	fixture.manifest = fixture.baseManifest("0.1.1", []Component{component})
	updater := testUpdater(t, fixture)
	saveInstalledState(t, updater, "0.1.0", map[string]string{"core": "1.0.0"})
	writeInstallFile(t, updater, "bin/client", "old")
	runner := &Runner{
		Updater: updater, Channel: "stable", ManifestURL: fixture.server.URL + "/manifest.json",
		ClientExecutable: "bin/client", launch: func(string, []string) error { return fmt.Errorf("startup failed") },
	}

	if _, err := runner.Run(context.Background()); err == nil || !strings.Contains(err.Error(), "startup failed") {
		t.Fatalf("expected launch failure, got %v", err)
	}
	if got := readInstallFile(t, updater, "bin/client"); got != "old" {
		t.Fatalf("failed client was not rolled back: %q", got)
	}
	state, _ := LoadState(updater.statePath())
	if state.InstalledVersion != "0.1.0" || state.LastGoodVersion != "0.1.0" {
		t.Fatalf("failed update remained installed: %+v", state)
	}
}

func TestApplyFailureRollsBackImmediately(t *testing.T) {
	fixture := newReleaseServer(t)
	component := fixture.component(t, "core", "1.0.1", map[string]string{"bin/client": "new", "data/assets": "new"})
	fixture.manifest = fixture.baseManifest("0.1.1", []Component{component})
	updater := testUpdater(t, fixture)
	saveInstalledState(t, updater, "0.1.0", map[string]string{"core": "1.0.0"})
	writeInstallFile(t, updater, "bin/client", "old")
	updater.applyHook = func(int) error { return fmt.Errorf("injected apply failure") }

	if _, err := updater.Update(context.Background(), "stable", fixture.server.URL+"/manifest.json"); err == nil {
		t.Fatal("expected apply failure")
	}
	if got := readInstallFile(t, updater, "bin/client"); got != "old" {
		t.Fatalf("rollback did not restore client: %q", got)
	}
}
