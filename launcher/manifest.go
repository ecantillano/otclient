package launcher

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	ManifestSchemaVersion    = 1
	SupportedProtocolVersion = 1525
	maxManifestBytes         = 1 << 20
)

var componentNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var assetVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var eventNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

const (
	ProductionLoginURL  = "https://login.thappy.cl/login.php"
	ProductionLoginPort = 443
)

type Manifest struct {
	SchemaVersion          int         `json:"schema_version"`
	Channel                string      `json:"channel"`
	Version                string      `json:"version"`
	ProtocolVersion        int         `json:"protocol_version"`
	AssetVersion           string      `json:"asset_version"`
	ReleaseNotesURL        string      `json:"release_notes_url"`
	Mandatory              bool        `json:"mandatory"`
	Environment            string      `json:"environment"`
	LoginURL               string      `json:"login_url"`
	LoginPort              int         `json:"login_port"`
	HTTPLogin              bool        `json:"http_login"`
	UseAuthenticator       bool        `json:"use_authenticator"`
	MinimumLauncherVersion string      `json:"minimum_launcher_version,omitempty"`
	PublishedAt            string      `json:"published_at,omitempty"`
	Components             []Component `json:"components"`
	Delete                 []string    `json:"delete,omitempty"`
}

type Component struct {
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Platforms []string `json:"platforms,omitempty"`
	URL       string   `json:"url"`
	SHA256    string   `json:"sha256"`
	Size      int64    `json:"size"`
	Archive   string   `json:"archive"`
	Target    string   `json:"target,omitempty"`
}

func DecodeManifest(reader io.Reader) (Manifest, error) {
	limited := io.LimitReader(reader, maxManifestBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	if len(data) > maxManifestBytes {
		return Manifest{}, fmt.Errorf("manifest exceeds %d bytes", maxManifestBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("JSON contains more than one document")
		}
		return fmt.Errorf("invalid trailing JSON: %w", err)
	}
	return nil
}

func (manifest Manifest) Validate(expectedChannel, launcherVersion string, allowedHosts []string) error {
	if manifest.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("unsupported manifest schema %d", manifest.SchemaVersion)
	}
	if manifest.Channel != expectedChannel || (manifest.Channel != "stable" && manifest.Channel != "test") {
		return fmt.Errorf("manifest channel %q does not match requested channel %q", manifest.Channel, expectedChannel)
	}
	if manifest.ProtocolVersion != SupportedProtocolVersion {
		return fmt.Errorf("manifest protocol_version %d is unsupported; this launcher requires %d", manifest.ProtocolVersion, SupportedProtocolVersion)
	}
	if !assetVersionPattern.MatchString(manifest.AssetVersion) {
		return fmt.Errorf("invalid asset_version %q", manifest.AssetVersion)
	}
	if err := ValidateHTTPSURL(manifest.ReleaseNotesURL, allowedHosts); err != nil {
		return fmt.Errorf("release notes URL: %w", err)
	}
	expectedEnvironment := "production"
	if manifest.Channel == "test" {
		expectedEnvironment = "test"
	}
	if manifest.Environment != expectedEnvironment {
		return fmt.Errorf("environment %q does not match %s channel", manifest.Environment, manifest.Channel)
	}
	if err := ValidateHTTPSURL(manifest.LoginURL, allowedHosts); err != nil {
		return fmt.Errorf("login URL: %w", err)
	}
	if manifest.LoginPort < 1 || manifest.LoginPort > 65535 {
		return fmt.Errorf("invalid login_port %d", manifest.LoginPort)
	}
	if manifest.Environment == "production" {
		if manifest.LoginURL != ProductionLoginURL || manifest.LoginPort != ProductionLoginPort || manifest.HTTPLogin || manifest.UseAuthenticator {
			return fmt.Errorf("production endpoint must be %s on port %d with http_login=false and use_authenticator=false", ProductionLoginURL, ProductionLoginPort)
		}
	}
	targetVersion, err := ParseVersion(manifest.Version)
	if err != nil {
		return err
	}
	if manifest.Channel == "stable" && targetVersion.IsPrerelease() {
		return fmt.Errorf("stable channel cannot install prerelease %q", manifest.Version)
	}
	currentLauncher, err := ParseVersion(launcherVersion)
	if err != nil {
		return fmt.Errorf("invalid launcher build version: %w", err)
	}
	if manifest.MinimumLauncherVersion != "" {
		minimum, parseErr := ParseVersion(manifest.MinimumLauncherVersion)
		if parseErr != nil {
			return fmt.Errorf("invalid minimum launcher version: %w", parseErr)
		}
		if currentLauncher.Compare(minimum) < 0 {
			return fmt.Errorf("launcher %s is older than required version %s; update Thappy Launcher manually before continuing", currentLauncher, minimum)
		}
	}
	if manifest.PublishedAt != "" {
		if _, err := time.Parse(time.RFC3339, manifest.PublishedAt); err != nil {
			return fmt.Errorf("invalid published_at: %w", err)
		}
	}
	if len(manifest.Components) == 0 {
		return fmt.Errorf("manifest has no components")
	}

	seenNames := make(map[string]struct{}, len(manifest.Components))
	for index, component := range manifest.Components {
		if !componentNamePattern.MatchString(component.Name) {
			return fmt.Errorf("component %d has invalid name %q", index, component.Name)
		}
		nameKey := strings.ToLower(component.Name)
		if _, duplicate := seenNames[nameKey]; duplicate {
			return fmt.Errorf("duplicate component name %q", component.Name)
		}
		seenNames[nameKey] = struct{}{}
		if _, err := ParseVersion(component.Version); err != nil {
			return fmt.Errorf("component %q: %w", component.Name, err)
		}
		if component.Size <= 0 {
			return fmt.Errorf("component %q has invalid size", component.Name)
		}
		if component.Archive != "zip" {
			return fmt.Errorf("component %q uses unsupported archive %q", component.Name, component.Archive)
		}
		if _, err := decodeSHA256(component.SHA256); err != nil {
			return fmt.Errorf("component %q: %w", component.Name, err)
		}
		if err := ValidateHTTPSURL(component.URL, allowedHosts); err != nil {
			return fmt.Errorf("component %q URL: %w", component.Name, err)
		}
		if component.Target != "" && component.Target != "." {
			if _, err := SafeRelativePath(component.Target); err != nil {
				return fmt.Errorf("component %q target: %w", component.Name, err)
			}
		}
		for _, platform := range component.Platforms {
			if platform != "windows" && platform != "linux" {
				return fmt.Errorf("component %q has unsupported platform %q", component.Name, platform)
			}
		}
	}
	for _, deleted := range manifest.Delete {
		if _, err := SafeRelativePath(strings.TrimSuffix(strings.ReplaceAll(deleted, `\`, "/"), "/")); err != nil {
			return fmt.Errorf("invalid delete path %q: %w", deleted, err)
		}
	}
	return nil
}

func (manifest Manifest) PolicyFor(platform string) CachedPolicy {
	components := make(map[string]string)
	hashes := make(map[string]string)
	for _, component := range manifest.Components {
		if component.AppliesTo(platform) {
			components[component.Name] = component.Version
			hashes[component.Name] = component.SHA256
		}
	}
	return CachedPolicy{
		Channel:                manifest.Channel,
		Version:                manifest.Version,
		MinimumLauncherVersion: manifest.MinimumLauncherVersion,
		ProtocolVersion:        manifest.ProtocolVersion,
		AssetVersion:           manifest.AssetVersion,
		ReleaseNotesURL:        manifest.ReleaseNotesURL,
		Mandatory:              manifest.Mandatory,
		Environment:            manifest.Environment,
		LoginURL:               manifest.LoginURL,
		LoginPort:              manifest.LoginPort,
		HTTPLogin:              manifest.HTTPLogin,
		UseAuthenticator:       manifest.UseAuthenticator,
		Components:             components,
		ComponentHashes:        hashes,
		FetchedAt:              time.Now().UTC().Format(time.RFC3339),
	}
}

func (component Component) AppliesTo(platform string) bool {
	if platform == "" {
		platform = runtime.GOOS
	}
	if len(component.Platforms) == 0 {
		return true
	}
	for _, candidate := range component.Platforms {
		if candidate == platform {
			return true
		}
	}
	return false
}

func decodeSHA256(value string) ([]byte, error) {
	if len(value) != 64 {
		return nil, fmt.Errorf("SHA-256 must contain 64 hexadecimal characters")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("invalid SHA-256: %w", err)
	}
	return decoded, nil
}

func ValidateHTTPSURL(rawURL string, allowedHosts []string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Hostname() == "" {
		return fmt.Errorf("URL must use HTTPS and include a host")
	}
	if parsed.User != nil {
		return fmt.Errorf("URL userinfo is forbidden")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	for _, allowed := range allowedHosts {
		if host == strings.ToLower(strings.TrimSuffix(strings.TrimSpace(allowed), ".")) {
			return nil
		}
	}
	return fmt.Errorf("host %q is not allowlisted", host)
}
