package launcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const ConfigSchemaVersion = 1

type Config struct {
	SchemaVersion     int                      `json:"schema_version"`
	DefaultChannel    string                   `json:"default_channel"`
	Channels          map[string]ChannelConfig `json:"channels"`
	AllowedHosts      []string                 `json:"allowed_hosts"`
	ClientExecutables map[string]string        `json:"client_executables"`
	ClientArgs        []string                 `json:"client_args,omitempty"`
	DeleteAllowlist   []string                 `json:"delete_allowlist"`
	PreservePaths     []string                 `json:"preserve_paths"`
}

type ChannelConfig struct {
	ManifestURL string `json:"manifest_url"`
}

var mandatoryPreservePaths = []string{
	".thappy-launcher/",
	"launcher-config.json",
	"ThappyLauncher.exe",
	"thappy-launcher",
	"config.otml",
	"settings/",
	"profiles/",
	"screenshots/",
	"records/",
	"logs/",
	"minimap/",
}

func DefaultConfig() Config {
	return Config{
		SchemaVersion:  ConfigSchemaVersion,
		DefaultChannel: "stable",
		Channels: map[string]ChannelConfig{
			"stable": {},
			"test":   {},
		},
		AllowedHosts: []string{
			"github.com",
			"objects.githubusercontent.com",
			"release-assets.githubusercontent.com",
			"login.thappy.cl",
		},
		ClientExecutables: map[string]string{
			"windows": "Thappy.exe",
			"linux":   "thappy",
		},
		DeleteAllowlist: []string{
			"otclient", "otclient.exe", "otclient_dx.exe", "otclient.ilk", "otclient.pdb",
			"mods/game_bot/", "mods/game_tasks/",
			"modules/client_bottommenu/", "modules/client_debug_info/", "modules/client_serverlist/",
			"modules/client_terminal/", "modules/game_actionbar/", "modules/game_analyser/",
			"modules/game_attachedeffects/", "modules/game_blessing/", "modules/game_creatureinformation/",
			"modules/game_cyclopedia/", "modules/game_forge/", "modules/game_healthcircle/",
			"modules/game_highscore/", "modules/game_htmlsample/", "modules/game_imbuementtracker/",
			"modules/game_imbuing/", "modules/game_inspect/", "modules/game_lootsplitter/",
			"modules/game_market/", "modules/game_notifications/", "modules/game_paperdolls/",
			"modules/game_playermount/", "modules/game_prey/", "modules/game_proficiency/",
			"modules/game_quickloot/", "modules/game_rewardwall/", "modules/game_shop/",
			"modules/game_stash/", "modules/game_store/", "modules/game_taskboard/",
			"modules/game_tutorial/", "modules/game_wheel/", "modules/updater/",
		},
		PreservePaths: append([]string(nil), mandatoryPreservePaths...),
	}
}

func LoadConfig(fileName string) (Config, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return Config{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode launcher config: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Config{}, err
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config Config) Validate() error {
	if config.SchemaVersion != ConfigSchemaVersion {
		return fmt.Errorf("unsupported launcher config schema %d", config.SchemaVersion)
	}
	if config.DefaultChannel != "stable" && config.DefaultChannel != "test" {
		return fmt.Errorf("invalid default channel %q", config.DefaultChannel)
	}
	for _, channel := range []string{"stable", "test"} {
		entry, ok := config.Channels[channel]
		if !ok {
			return fmt.Errorf("missing %s channel", channel)
		}
		if entry.ManifestURL != "" {
			if err := ValidateHTTPSURL(entry.ManifestURL, config.AllowedHosts); err != nil {
				return fmt.Errorf("%s manifest URL: %w", channel, err)
			}
		}
	}
	if len(config.AllowedHosts) == 0 {
		return fmt.Errorf("allowed_hosts cannot be empty")
	}
	for platform, executable := range config.ClientExecutables {
		if platform != "windows" && platform != "linux" {
			return fmt.Errorf("unsupported client platform %q", platform)
		}
		if _, err := SafeRelativePath(executable); err != nil {
			return fmt.Errorf("invalid %s client executable: %w", platform, err)
		}
	}
	for _, platform := range []string{"windows", "linux"} {
		if _, exists := config.ClientExecutables[platform]; !exists {
			return fmt.Errorf("missing client executable for %s", platform)
		}
	}
	for _, ruleSet := range [][]string{config.DeleteAllowlist, config.PreservePaths} {
		for _, rule := range ruleSet {
			if _, _, err := canonicalRule(rule); err != nil {
				return fmt.Errorf("invalid path rule %q: %w", rule, err)
			}
		}
	}
	for _, rule := range config.DeleteAllowlist {
		normalized, prefix, _ := canonicalRule(rule)
		if prefix && !strings.Contains(normalized, "/") {
			return fmt.Errorf("delete allowlist rule %q is too broad", rule)
		}
	}
	for _, required := range mandatoryPreservePaths {
		target := strings.TrimSuffix(required, "/")
		if !matchesRules(target, config.PreservePaths) {
			return fmt.Errorf("preserve_paths must protect %q", required)
		}
	}
	return nil
}

func (config Config) ManifestURL(channel string) (string, error) {
	entry, ok := config.Channels[channel]
	if !ok || (channel != "stable" && channel != "test") {
		return "", fmt.Errorf("unknown channel %q", channel)
	}
	if entry.ManifestURL == "" {
		return "", fmt.Errorf("channel %q has no manifest URL configured", channel)
	}
	return entry.ManifestURL, nil
}
