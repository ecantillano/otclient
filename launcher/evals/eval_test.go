//go:build eval

package evals

import (
	"encoding/json"
	"os"
	"testing"

	launcher "github.com/ecantillano/otclient/launcher"
)

type validationCase struct {
	Name  string `json:"name"`
	Input string `json:"input"`
	Valid bool   `json:"valid"`
}

type manifestCase struct {
	Name     string `json:"name"`
	Mutation string `json:"mutation"`
	Valid    bool   `json:"valid"`
}

type evalCases struct {
	Semver    []validationCase `json:"semver"`
	Paths     []validationCase `json:"paths"`
	Manifests []manifestCase   `json:"manifests"`
}

func TestReleaseContractEvalFixtures(t *testing.T) {
	data, err := os.ReadFile("cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases evalCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range cases.Semver {
		t.Run("semver/"+testCase.Name, func(t *testing.T) {
			_, err := launcher.ParseVersion(testCase.Input)
			if (err == nil) != testCase.Valid {
				t.Fatalf("input=%q valid=%v err=%v", testCase.Input, testCase.Valid, err)
			}
		})
	}
	for _, testCase := range cases.Paths {
		t.Run("path/"+testCase.Name, func(t *testing.T) {
			_, err := launcher.SafeRelativePath(testCase.Input)
			if (err == nil) != testCase.Valid {
				t.Fatalf("input=%q valid=%v err=%v", testCase.Input, testCase.Valid, err)
			}
		})
	}
	for _, testCase := range cases.Manifests {
		t.Run("manifest/"+testCase.Name, func(t *testing.T) {
			manifest, requestedChannel := baseManifest(), "stable"
			switch testCase.Mutation {
			case "none":
			case "test_channel":
				manifest.Channel, manifest.Environment, manifest.Version = "test", "test", "0.2.0-rc.1"
				requestedChannel = "test"
			case "old_protocol":
				manifest.ProtocolVersion = 1098
			case "stable_prerelease":
				manifest.Version = "0.2.0-rc.1"
			case "wrong_environment":
				manifest.Environment = "test"
			case "untrusted_notes":
				manifest.ReleaseNotesURL = "https://evil.example/notes"
			case "invalid_endpoint":
				manifest.LoginURL = "https://other.example/login.php"
			default:
				t.Fatalf("unknown mutation %q", testCase.Mutation)
			}
			err := manifest.Validate(requestedChannel, launcher.LauncherVersion, []string{"cdn.example", "login.thappy.cl", "other.example"})
			if (err == nil) != testCase.Valid {
				t.Fatalf("valid=%v err=%v manifest=%+v", testCase.Valid, err, manifest)
			}
		})
	}
}

func baseManifest() launcher.Manifest {
	return launcher.Manifest{
		SchemaVersion: launcher.ManifestSchemaVersion, Channel: "stable", Version: "0.1.1",
		ProtocolVersion: launcher.SupportedProtocolVersion, AssetVersion: "1525",
		ReleaseNotesURL: "https://cdn.example/notes", Environment: "production",
		LoginURL: launcher.ProductionLoginURL, LoginPort: launcher.ProductionLoginPort,
		Components: []launcher.Component{{
			Name: "core", Version: "0.1.1", URL: "https://cdn.example/core.zip",
			SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Size:   100, Archive: "zip", Platforms: []string{"windows", "linux"},
		}},
	}
}
