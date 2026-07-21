package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type DiagnosticReport struct {
	LauncherVersion  string   `json:"launcher_version"`
	OS               string   `json:"os"`
	Architecture     string   `json:"architecture"`
	ProtocolVersion  int      `json:"protocol_version"`
	AssetVersion     string   `json:"asset_version,omitempty"`
	ReleaseNotesURL  string   `json:"release_notes_url,omitempty"`
	Mandatory        bool     `json:"mandatory"`
	Environment      string   `json:"environment,omitempty"`
	LoginURL         string   `json:"login_url,omitempty"`
	LoginPort        int      `json:"login_port,omitempty"`
	HTTPLogin        bool     `json:"http_login"`
	UseAuthenticator bool     `json:"use_authenticator"`
	Channel          string   `json:"channel"`
	ClientVersion    string   `json:"client_version,omitempty"`
	ManifestURL      string   `json:"manifest_url"`
	InstallDirectory string   `json:"install_directory"`
	State            *State   `json:"state,omitempty"`
	StateError       string   `json:"state_error,omitempty"`
	ClientExecutable string   `json:"client_executable"`
	ClientExists     bool     `json:"client_exists"`
	Locked           bool     `json:"locked"`
	PendingUpdate    bool     `json:"pending_update"`
	LastError        string   `json:"last_error,omitempty"`
	LastUpdate       string   `json:"last_update,omitempty"`
	IntegrityStatus  string   `json:"integrity_status"`
	LogFile          string   `json:"log_file"`
	DiskFreeBytes    *uint64  `json:"disk_free_bytes,omitempty"`
	DiskCheck        string   `json:"disk_check"`
	AllowedHosts     []string `json:"allowed_hosts"`
}

func BuildDiagnostic(updater *Updater, channel, manifestURL, clientExecutable string) DiagnosticReport {
	report := DiagnosticReport{
		LauncherVersion:  updater.LauncherVersion,
		OS:               runtime.GOOS,
		Architecture:     runtime.GOARCH,
		ProtocolVersion:  SupportedProtocolVersion,
		Channel:          channel,
		ManifestURL:      RedactURL(manifestURL),
		InstallDirectory: RedactPath(updater.InstallDir),
		AllowedHosts:     append([]string(nil), updater.Config.AllowedHosts...),
		DiskCheck:        "unsupported",
		IntegrityStatus:  "no-package-hashes",
		LogFile:          RedactPath(filepath.Join(updater.StateRoot, "logs", "launcher.log")),
	}
	state, err := LoadState(updater.statePath())
	if err != nil {
		report.StateError = PublicError(err)
	} else {
		report.ClientVersion = state.InstalledVersion
		if state.LastError != "" {
			report.LastError = PublicError(errors.New(state.LastError))
		}
		report.LastUpdate = state.UpdatedAt
		if len(state.ComponentHashes) != 0 {
			report.IntegrityStatus = "verified-package-hashes-recorded"
		}
		if state.LastManifest != nil {
			policy := *state.LastManifest
			policy.ReleaseNotesURL = RedactURL(policy.ReleaseNotesURL)
			policy.Components = cloneStringMap(policy.Components)
			policy.ComponentHashes = cloneStringMap(policy.ComponentHashes)
			state.LastManifest = &policy
			report.ProtocolVersion = policy.ProtocolVersion
			report.AssetVersion = policy.AssetVersion
			report.ReleaseNotesURL = policy.ReleaseNotesURL
			report.Mandatory = policy.Mandatory
			report.Environment = policy.Environment
			report.LoginURL = RedactURL(policy.LoginURL)
			report.LoginPort = policy.LoginPort
			report.HTTPLogin = policy.HTTPLogin
			report.UseAuthenticator = policy.UseAuthenticator
		}
		report.State = &state
	}
	if _, journalErr := os.Stat(updater.journalPath()); journalErr == nil {
		report.PendingUpdate = true
		report.IntegrityStatus = "pending-launch-confirmation"
	}
	runner := &Runner{Updater: updater, ClientExecutable: clientExecutable}
	if clientPath, pathErr := runner.clientPath(); pathErr == nil {
		report.ClientExecutable = RedactPath(clientPath)
		if info, statErr := os.Stat(clientPath); statErr == nil && info.Mode().IsRegular() {
			report.ClientExists = true
		}
	}
	if lock, lockErr := AcquireFileLock(updater.lockPath()); lockErr == nil {
		_ = lock.Close()
	} else {
		report.Locked = true
	}
	if available, supported, diskErr := availableDiskBytes(updater.InstallDir); diskErr != nil {
		report.DiskCheck = PublicError(diskErr)
	} else if supported {
		report.DiskCheck = "ok"
		report.DiskFreeBytes = &available
	}
	return report
}

func cloneStringMap(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func WriteDiagnostic(report DiagnosticReport) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func RedactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "[invalid URL]"
	}
	if parsed.User != nil {
		parsed.User = url.User("[redacted]")
	}
	if parsed.RawQuery != "" {
		parsed.RawQuery = "redacted"
	}
	parsed.Fragment = ""
	return parsed.String()
}

func RedactPath(value string) string {
	home, err := os.UserHomeDir()
	if err == nil {
		if relative, relErr := filepath.Rel(home, value); relErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			if relative == "." {
				return "~"
			}
			return filepath.Join("~", relative)
		}
	}
	return filepath.Clean(value)
}

func PublicError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	cursor := 0
	for {
		relativeStart := strings.Index(message[cursor:], "https://")
		if relativeStart < 0 {
			break
		}
		start := cursor + relativeStart
		end := len(message)
		for index := start; index < len(message); index++ {
			if strings.ContainsRune(" \t\r\n\"'", rune(message[index])) {
				end = index
				break
			}
		}
		redacted := RedactURL(strings.TrimRight(message[start:end], ").,;"))
		message = message[:start] + redacted + message[end:]
		cursor = start + len(redacted)
	}
	if message == "" {
		return fmt.Sprintf("%T", err)
	}
	return message
}
