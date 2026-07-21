package launcher

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const LauncherVersion = "0.1.0"

type UpdateStatus string

const (
	UpdateNotNeeded UpdateStatus = "current"
	UpdateApplied   UpdateStatus = "updated"
)

type UpdateResult struct {
	Status              UpdateStatus
	PreviousVersion     string
	InstalledVersion    string
	Recovered           bool
	PendingConfirmation bool
}

type ProgressEvent struct {
	Phase          string
	Component      string
	CompletedBytes uint64
	TotalBytes     uint64
}

type Updater struct {
	InstallDir      string
	StateRoot       string
	Platform        string
	LauncherVersion string
	Config          Config
	Fetcher         *Fetcher
	Progress        func(ProgressEvent)
	applyHook       func(int) error
}

func (updater *Updater) reportProgress(event ProgressEvent) {
	if updater.Progress != nil {
		updater.Progress(event)
	}
}

func NewUpdater(installDir string, config Config, httpClient *http.Client) (*Updater, error) {
	absolute, err := filepath.Abs(installDir)
	if err != nil {
		return nil, err
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &Updater{
		InstallDir:      absolute,
		StateRoot:       filepath.Join(absolute, ".thappy-launcher"),
		Platform:        runtime.GOOS,
		LauncherVersion: LauncherVersion,
		Config:          config,
		Fetcher:         NewFetcher(config.AllowedHosts, httpClient),
	}, nil
}

func (updater *Updater) statePath() string { return filepath.Join(updater.StateRoot, "state.json") }
func (updater *Updater) journalPath() string {
	return filepath.Join(updater.StateRoot, "transaction.json")
}
func (updater *Updater) lockPath() string { return filepath.Join(updater.StateRoot, "launcher.lock") }

func (updater *Updater) Recover() (bool, error) {
	data, err := os.ReadFile(updater.journalPath())
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var tx transaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return false, fmt.Errorf("cannot recover invalid transaction journal: %w", err)
	}
	if tx.SchemaVersion != transactionSchemaVersion || tx.InstallDir != updater.InstallDir {
		return false, fmt.Errorf("transaction journal does not belong to this installation")
	}
	if err := updater.validateTransactionPaths(&tx); err != nil {
		return false, err
	}
	switch tx.Phase {
	case "applying", "committing", "pending_launch":
		return true, rollbackTransaction(updater.journalPath(), updater.statePath(), &tx)
	case "confirming":
		confirmed := tx.NextState
		confirmed.LastGoodVersion = confirmed.InstalledVersion
		if err := SaveState(updater.statePath(), confirmed); err != nil {
			return false, err
		}
		tx.Phase = "confirmed"
		if err := writeJSONAtomic(updater.journalPath(), &tx, 0o600); err != nil {
			return false, err
		}
		return true, cleanupConfirmedTransaction(updater.journalPath(), &tx)
	case "confirmed":
		return true, cleanupConfirmedTransaction(updater.journalPath(), &tx)
	default:
		return false, fmt.Errorf("transaction journal has unknown phase %q", tx.Phase)
	}
}

func (updater *Updater) loadTransaction() (*transaction, error) {
	data, err := os.ReadFile(updater.journalPath())
	if err != nil {
		return nil, err
	}
	var tx transaction
	if err := json.Unmarshal(data, &tx); err != nil {
		return nil, fmt.Errorf("invalid transaction journal: %w", err)
	}
	if tx.SchemaVersion != transactionSchemaVersion || tx.InstallDir != updater.InstallDir {
		return nil, fmt.Errorf("transaction journal does not belong to this installation")
	}
	if err := updater.validateTransactionPaths(&tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

func (updater *Updater) ConfirmPending() error {
	tx, err := updater.loadTransaction()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if tx.Phase != "pending_launch" {
		return fmt.Errorf("cannot confirm transaction in phase %q", tx.Phase)
	}
	tx.Phase = "confirming"
	if err := writeJSONAtomic(updater.journalPath(), tx, 0o600); err != nil {
		return err
	}
	confirmed := tx.NextState
	confirmed.LastGoodVersion = confirmed.InstalledVersion
	if err := SaveState(updater.statePath(), confirmed); err != nil {
		return err
	}
	tx.Phase = "confirmed"
	if err := writeJSONAtomic(updater.journalPath(), tx, 0o600); err != nil {
		return err
	}
	return cleanupConfirmedTransaction(updater.journalPath(), tx)
}

func (updater *Updater) RollbackPending() error {
	tx, err := updater.loadTransaction()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if tx.Phase != "pending_launch" && tx.Phase != "committing" {
		return fmt.Errorf("cannot roll back transaction in phase %q", tx.Phase)
	}
	return rollbackTransaction(updater.journalPath(), updater.statePath(), tx)
}

func (updater *Updater) validateTransactionPaths(tx *transaction) error {
	for name, candidate := range map[string]string{"staging": tx.StagingDir, "backup": tx.BackupDir} {
		relative, err := filepath.Rel(updater.StateRoot, candidate)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("transaction %s directory is outside launcher state", name)
		}
	}
	for _, operation := range tx.Operations {
		if _, err := SafeRelativePath(operation.Target); err != nil {
			return fmt.Errorf("invalid transaction target: %w", err)
		}
		if operation.Kind == "file" {
			relative, err := filepath.Rel(tx.StagingDir, operation.Source)
			if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return fmt.Errorf("transaction source is outside staging")
			}
		}
	}
	return nil
}

func (updater *Updater) Update(ctx context.Context, channel, manifestURL string) (UpdateResult, error) {
	result := UpdateResult{}
	if err := os.MkdirAll(updater.StateRoot, 0o700); err != nil {
		return result, err
	}
	recovered, err := updater.Recover()
	if err != nil {
		return result, err
	}
	result.Recovered = recovered

	state, err := LoadState(updater.statePath())
	if err != nil {
		return result, err
	}
	result.PreviousVersion = state.InstalledVersion
	result.InstalledVersion = state.InstalledVersion

	manifest, err := updater.Fetcher.FetchManifest(ctx, manifestURL)
	if err != nil {
		return result, err
	}
	if validationErr := manifest.Validate(channel, updater.LauncherVersion, updater.Config.AllowedHosts); validationErr != nil {
		const maximumLauncherVersion = "18446744073709551615.18446744073709551615.18446744073709551615"
		if policyErr := manifest.Validate(channel, maximumLauncherVersion, updater.Config.AllowedHosts); policyErr == nil {
			state.LastManifest = pointerToPolicy(manifest.PolicyFor(updater.Platform))
			if saveErr := SaveState(updater.statePath(), state); saveErr != nil {
				return result, fmt.Errorf("cache incompatible release policy: %v; validation failed: %w", saveErr, validationErr)
			}
		}
		return result, validationErr
	}
	state.LastManifest = pointerToPolicy(manifest.PolicyFor(updater.Platform))
	if err := SaveState(updater.statePath(), state); err != nil {
		return result, fmt.Errorf("cache release policy: %w", err)
	}
	installedVersion, _ := ParseVersion(state.InstalledVersion)
	targetVersion, _ := ParseVersion(manifest.Version)
	comparison := targetVersion.Compare(installedVersion)
	if comparison < 0 {
		return result, fmt.Errorf("refusing downgrade from %s to %s", state.InstalledVersion, manifest.Version)
	}

	components := make([]Component, 0, len(manifest.Components))
	nextComponents := make(map[string]string)
	nextHashes := make(map[string]string)
	var downloadBytes uint64
	for _, component := range manifest.Components {
		if !component.AppliesTo(updater.Platform) {
			continue
		}
		nextComponents[component.Name] = component.Version
		nextHashes[component.Name] = component.SHA256
		installedComponentVersion, installed := state.Components[component.Name]
		if installed {
			currentComponent, _ := ParseVersion(installedComponentVersion)
			targetComponent, _ := ParseVersion(component.Version)
			if targetComponent.Compare(currentComponent) < 0 {
				return result, fmt.Errorf("refusing component %q downgrade from %s to %s", component.Name, installedComponentVersion, component.Version)
			}
			if component.Version == installedComponentVersion {
				if installedHash := state.ComponentHashes[component.Name]; installedHash != "" && !strings.EqualFold(installedHash, component.SHA256) {
					return result, fmt.Errorf("component %q changed hash without a version change", component.Name)
				}
				continue
			}
		}
		components = append(components, component)
		if uint64(component.Size) > ^uint64(0)-downloadBytes {
			return result, fmt.Errorf("component sizes overflow")
		}
		downloadBytes += uint64(component.Size)
	}
	if len(nextComponents) == 0 {
		return result, fmt.Errorf("manifest has no components for %s", updater.Platform)
	}
	if comparison == 0 && len(components) == 0 {
		result.Status = UpdateNotNeeded
		return result, nil
	}
	if len(components) == 0 && len(manifest.Delete) == 0 {
		state.InstalledVersion = manifest.Version
		state.LastGoodVersion = manifest.Version
		state.Components = nextComponents
		state.ComponentHashes = nextHashes
		if err := SaveState(updater.statePath(), state); err != nil {
			return result, err
		}
		result.Status = UpdateApplied
		result.InstalledVersion = manifest.Version
		return result, nil
	}
	updater.reportProgress(ProgressEvent{Phase: "planned", TotalBytes: downloadBytes})
	if available, supported, diskErr := availableDiskBytes(updater.StateRoot); diskErr != nil {
		return result, fmt.Errorf("download disk preflight: %w", diskErr)
	} else if supported && available < downloadBytes+diskSafetyMarginBytes {
		return result, fmt.Errorf("insufficient disk space for downloads")
	}

	transactionID, err := randomID()
	if err != nil {
		return result, err
	}
	stagingDir := filepath.Join(updater.StateRoot, "staging", transactionID)
	downloadDir := filepath.Join(stagingDir, "downloads")
	payloadDir := filepath.Join(stagingDir, "payload")
	backupDir := filepath.Join(updater.StateRoot, "backups", transactionID)
	if err := os.MkdirAll(downloadDir, 0o700); err != nil {
		return result, err
	}
	defer func() {
		if _, statErr := os.Stat(updater.journalPath()); errors.Is(statErr, os.ErrNotExist) {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	extractor := NewArchiveExtractor()
	var extracted []ExtractedFile
	var completedBytes uint64
	for _, component := range components {
		updater.reportProgress(ProgressEvent{Phase: "downloading", Component: component.Name, CompletedBytes: completedBytes, TotalBytes: downloadBytes})
		archivePath := filepath.Join(downloadDir, strings.ToLower(component.Name)+".zip")
		if err := updater.Fetcher.DownloadComponent(ctx, component, archivePath); err != nil {
			return result, err
		}
		files, _, err := extractor.Extract(archivePath, payloadDir, component.Target)
		if err != nil {
			return result, fmt.Errorf("extract component %q: %w", component.Name, err)
		}
		extracted = append(extracted, files...)
		completedBytes += uint64(component.Size)
		updater.reportProgress(ProgressEvent{Phase: "downloaded", Component: component.Name, CompletedBytes: completedBytes, TotalBytes: downloadBytes})
	}

	preservePaths := append([]string(nil), updater.Config.PreservePaths...)
	preservePaths = append(preservePaths, ".thappy-launcher/", "launcher-config.json")
	operations, err := buildOperations(extracted, manifest.Delete, updater.Config.DeleteAllowlist, preservePaths)
	if err != nil {
		return result, err
	}
	updater.reportProgress(ProgressEvent{Phase: "applying", CompletedBytes: completedBytes, TotalBytes: downloadBytes})
	nextState := State{
		SchemaVersion:    StateSchemaVersion,
		Channel:          channel,
		InstalledVersion: manifest.Version,
		LastGoodVersion:  state.LastGoodVersion,
		Components:       nextComponents,
		ComponentHashes:  nextHashes,
		LastManifest:     state.LastManifest,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	tx := &transaction{
		ID:            transactionID,
		InstallDir:    updater.InstallDir,
		StagingDir:    stagingDir,
		BackupDir:     backupDir,
		PreviousState: state,
		NextState:     nextState,
		Operations:    operations,
	}
	if err := applyTransaction(updater.journalPath(), updater.statePath(), tx, updater.applyHook); err != nil {
		return result, err
	}
	result.Status = UpdateApplied
	result.InstalledVersion = manifest.Version
	result.PendingConfirmation = true
	updater.reportProgress(ProgressEvent{Phase: "pending_launch", CompletedBytes: downloadBytes, TotalBytes: downloadBytes})
	return result, nil
}

func pointerToPolicy(policy CachedPolicy) *CachedPolicy { return &policy }

func (updater *Updater) OfflineLaunchAllowed() (bool, string, error) {
	state, err := LoadState(updater.statePath())
	if err != nil {
		return false, "", err
	}
	policy := state.LastManifest
	if policy == nil || !policy.Mandatory {
		return true, "", nil
	}
	if policy.MinimumLauncherVersion != "" {
		currentLauncher, currentErr := ParseVersion(updater.LauncherVersion)
		minimumLauncher, minimumErr := ParseVersion(policy.MinimumLauncherVersion)
		if currentErr != nil || minimumErr != nil {
			return false, "", errors.Join(currentErr, minimumErr)
		}
		if currentLauncher.Compare(minimumLauncher) < 0 {
			return false, policy.Version, nil
		}
	}
	installed, _ := ParseVersion(state.InstalledVersion)
	required, err := ParseVersion(policy.Version)
	if err != nil {
		return false, "", err
	}
	if installed.Compare(required) < 0 {
		return false, policy.Version, nil
	}
	for name, requiredVersion := range policy.Components {
		installedVersion, exists := state.Components[name]
		if !exists || installedVersion != requiredVersion {
			return false, policy.Version, nil
		}
	}
	for name, requiredHash := range policy.ComponentHashes {
		installedHash, exists := state.ComponentHashes[name]
		if !exists || !strings.EqualFold(installedHash, requiredHash) {
			return false, policy.Version, nil
		}
	}
	return true, "", nil
}

func (updater *Updater) RecordLastError(recorded error) error {
	state, err := LoadState(updater.statePath())
	if err != nil {
		return err
	}
	state.LastError = PublicError(recorded)
	return SaveState(updater.statePath(), state)
}

func (updater *Updater) log(event, message string) {
	_ = AppendDiagnosticLog(updater.StateRoot, event, message)
}

func randomID() (string, error) {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
