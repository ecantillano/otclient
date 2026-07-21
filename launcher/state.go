package launcher

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const StateSchemaVersion = 1

type State struct {
	SchemaVersion    int               `json:"schema_version"`
	Channel          string            `json:"channel"`
	InstalledVersion string            `json:"installed_version"`
	LastGoodVersion  string            `json:"last_good_version"`
	Components       map[string]string `json:"components,omitempty"`
	ComponentHashes  map[string]string `json:"component_hashes,omitempty"`
	LastManifest     *CachedPolicy     `json:"last_manifest,omitempty"`
	LastError        string            `json:"last_error,omitempty"`
	UpdatedAt        string            `json:"updated_at,omitempty"`
}

type CachedPolicy struct {
	Channel                string            `json:"channel"`
	Version                string            `json:"version"`
	MinimumLauncherVersion string            `json:"minimum_launcher_version,omitempty"`
	ProtocolVersion        int               `json:"protocol_version"`
	AssetVersion           string            `json:"asset_version"`
	ReleaseNotesURL        string            `json:"release_notes_url"`
	Mandatory              bool              `json:"mandatory"`
	Environment            string            `json:"environment"`
	LoginURL               string            `json:"login_url"`
	LoginPort              int               `json:"login_port"`
	HTTPLogin              bool              `json:"http_login"`
	UseAuthenticator       bool              `json:"use_authenticator"`
	Components             map[string]string `json:"components"`
	ComponentHashes        map[string]string `json:"component_hashes"`
	FetchedAt              string            `json:"fetched_at"`
}

func DefaultState() State {
	return State{
		SchemaVersion:    StateSchemaVersion,
		InstalledVersion: "0.0.0",
		LastGoodVersion:  "0.0.0",
		Components:       make(map[string]string),
		ComponentHashes:  make(map[string]string),
	}
}

func LoadState(fileName string) (State, error) {
	data, err := os.ReadFile(fileName)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultState(), nil
	}
	if err != nil {
		return State{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var state State
	if err := decoder.Decode(&state); err != nil {
		return State{}, fmt.Errorf("decode launcher state: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return State{}, err
	}
	if state.SchemaVersion != StateSchemaVersion {
		return State{}, fmt.Errorf("unsupported launcher state schema %d", state.SchemaVersion)
	}
	if _, err := ParseVersion(state.InstalledVersion); err != nil {
		return State{}, fmt.Errorf("invalid installed state: %w", err)
	}
	if _, err := ParseVersion(state.LastGoodVersion); err != nil {
		return State{}, fmt.Errorf("invalid last-good state: %w", err)
	}
	if state.Components == nil {
		state.Components = make(map[string]string)
	}
	if state.ComponentHashes == nil {
		state.ComponentHashes = make(map[string]string)
	}
	if state.LastManifest != nil {
		if _, err := ParseVersion(state.LastManifest.Version); err != nil {
			return State{}, fmt.Errorf("invalid cached manifest version: %w", err)
		}
		if state.LastManifest.ProtocolVersion != SupportedProtocolVersion {
			return State{}, fmt.Errorf("invalid cached protocol version %d", state.LastManifest.ProtocolVersion)
		}
		if state.LastManifest.MinimumLauncherVersion != "" {
			if _, err := ParseVersion(state.LastManifest.MinimumLauncherVersion); err != nil {
				return State{}, fmt.Errorf("invalid cached minimum launcher version: %w", err)
			}
		}
		for name, version := range state.LastManifest.Components {
			if _, err := ParseVersion(version); err != nil {
				return State{}, fmt.Errorf("invalid cached component %q: %w", name, err)
			}
		}
	}
	return state, nil
}

func SaveState(fileName string, state State) error {
	state.SchemaVersion = StateSchemaVersion
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONAtomic(fileName, state, 0o600)
}

func writeJSONAtomic(fileName string, value any, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(fileName), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(filepath.Dir(fileName), ".write-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempName)
	}
	if err := temp.Chmod(mode); err != nil {
		cleanup()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return err
	}
	if err := replaceFile(tempName, fileName); err != nil {
		_ = os.Remove(tempName)
		return err
	}
	return syncDirectory(filepath.Dir(fileName))
}
