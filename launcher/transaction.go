package launcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const transactionSchemaVersion = 2

var errSimulatedInterruption = errors.New("simulated update interruption")

type transaction struct {
	SchemaVersion int         `json:"schema_version"`
	ID            string      `json:"id"`
	Phase         string      `json:"phase"`
	InstallDir    string      `json:"install_dir"`
	StagingDir    string      `json:"staging_dir"`
	BackupDir     string      `json:"backup_dir"`
	PreviousState State       `json:"previous_state"`
	NextState     State       `json:"next_state"`
	Operations    []operation `json:"operations"`
}

type operation struct {
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	Source      string `json:"source,omitempty"`
	HadOriginal bool   `json:"had_original"`
	BackedUp    bool   `json:"backed_up"`
	Started     bool   `json:"started"`
	Applied     bool   `json:"applied"`
}

func buildOperations(files []ExtractedFile, deleted []string, deleteAllowlist, preservePaths []string) ([]operation, error) {
	operations := make([]operation, 0, len(files)+len(deleted))
	deleteTargets := make([]string, 0, len(deleted))
	for _, rawTarget := range deleted {
		target, err := SafeRelativePath(strings.TrimSuffix(strings.ReplaceAll(rawTarget, `\`, "/"), "/"))
		if err != nil {
			return nil, err
		}
		if matchesRules(target, preservePaths) {
			return nil, fmt.Errorf("delete path %q overlaps preserved user data", target)
		}
		if !matchesRules(target, deleteAllowlist) {
			return nil, fmt.Errorf("delete path %q is outside the local delete allowlist", target)
		}
		for _, previous := range deleteTargets {
			if pathsOverlap(target, previous) {
				return nil, fmt.Errorf("overlapping delete paths %q and %q", target, previous)
			}
		}
		deleteTargets = append(deleteTargets, target)
	}
	sort.Strings(deleteTargets)
	for _, target := range deleteTargets {
		operations = append(operations, operation{Kind: "delete", Target: target})
	}

	fileTargets := make(map[string]struct{}, len(files))
	sort.Slice(files, func(i, j int) bool { return files[i].RelativePath < files[j].RelativePath })
	for _, file := range files {
		target, err := SafeRelativePath(file.RelativePath)
		if err != nil {
			return nil, err
		}
		if matchesRules(target, preservePaths) {
			return nil, fmt.Errorf("component path %q overlaps preserved user data", target)
		}
		key := strings.ToLower(target)
		if _, duplicate := fileTargets[key]; duplicate {
			return nil, fmt.Errorf("duplicate component target %q", target)
		}
		fileTargets[key] = struct{}{}
		operations = append(operations, operation{Kind: "file", Target: target, Source: file.SourcePath})
	}
	return operations, nil
}

func applyTransaction(journalPath, statePath string, tx *transaction, hook func(int) error) error {
	tx.SchemaVersion = transactionSchemaVersion
	tx.Phase = "applying"
	if err := writeJSONAtomic(journalPath, tx, 0o600); err != nil {
		return err
	}

	for index := range tx.Operations {
		operation := &tx.Operations[index]
		destination, err := secureJoin(tx.InstallDir, operation.Target)
		if err != nil {
			return rollbackAfterError(journalPath, statePath, tx, err)
		}
		if err := ensureNoSymlinkParents(tx.InstallDir, operation.Target, true); err != nil {
			return rollbackAfterError(journalPath, statePath, tx, err)
		}
		backup, err := secureJoin(tx.BackupDir, operation.Target)
		if err != nil {
			return rollbackAfterError(journalPath, statePath, tx, err)
		}

		info, statErr := os.Lstat(destination)
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return rollbackAfterError(journalPath, statePath, tx, statErr)
		}
		if statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return rollbackAfterError(journalPath, statePath, tx, fmt.Errorf("refusing to replace symlink %q", operation.Target))
			}
			operation.HadOriginal = true
		}
		operation.Started = true
		if err := writeJSONAtomic(journalPath, tx, 0o600); err != nil {
			return rollbackAfterError(journalPath, statePath, tx, err)
		}

		if operation.HadOriginal {
			if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
				return rollbackAfterError(journalPath, statePath, tx, err)
			}
			if err := os.Rename(destination, backup); err != nil {
				return rollbackAfterError(journalPath, statePath, tx, err)
			}
			operation.BackedUp = true
			if err := writeJSONAtomic(journalPath, tx, 0o600); err != nil {
				return rollbackAfterError(journalPath, statePath, tx, err)
			}
		}
		if operation.Kind == "file" {
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return rollbackAfterError(journalPath, statePath, tx, err)
			}
			if err := os.Rename(operation.Source, destination); err != nil {
				return rollbackAfterError(journalPath, statePath, tx, err)
			}
		} else if operation.Kind != "delete" {
			return rollbackAfterError(journalPath, statePath, tx, fmt.Errorf("unknown transaction operation %q", operation.Kind))
		}
		operation.Applied = true
		if err := writeJSONAtomic(journalPath, tx, 0o600); err != nil {
			return rollbackAfterError(journalPath, statePath, tx, err)
		}
		if hook != nil {
			if err := hook(index); err != nil {
				if errors.Is(err, errSimulatedInterruption) {
					return err
				}
				return rollbackAfterError(journalPath, statePath, tx, err)
			}
		}
	}

	tx.Phase = "committing"
	if err := writeJSONAtomic(journalPath, tx, 0o600); err != nil {
		return rollbackAfterError(journalPath, statePath, tx, err)
	}
	if err := SaveState(statePath, tx.NextState); err != nil {
		return rollbackAfterError(journalPath, statePath, tx, err)
	}
	tx.Phase = "pending_launch"
	if err := writeJSONAtomic(journalPath, tx, 0o600); err != nil {
		return err
	}
	return nil
}

func rollbackAfterError(journalPath, statePath string, tx *transaction, cause error) error {
	if rollbackErr := rollbackTransaction(journalPath, statePath, tx); rollbackErr != nil {
		return fmt.Errorf("update failed: %v; rollback failed: %w", cause, rollbackErr)
	}
	return cause
}

func rollbackTransaction(journalPath, statePath string, tx *transaction) error {
	var rollbackErrors []error
	for index := len(tx.Operations) - 1; index >= 0; index-- {
		operation := tx.Operations[index]
		if !operation.Started {
			continue
		}
		destination, destinationErr := secureJoin(tx.InstallDir, operation.Target)
		backup, backupErr := secureJoin(tx.BackupDir, operation.Target)
		if destinationErr != nil || backupErr != nil {
			rollbackErrors = append(rollbackErrors, errors.Join(destinationErr, backupErr))
			continue
		}
		if operation.HadOriginal {
			_, backupStatErr := os.Lstat(backup)
			if backupStatErr == nil {
				if err := removeTransactionTarget(destination, tx.InstallDir); err != nil {
					rollbackErrors = append(rollbackErrors, err)
					continue
				}
				if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
					rollbackErrors = append(rollbackErrors, err)
					continue
				}
				if err := os.Rename(backup, destination); err != nil {
					rollbackErrors = append(rollbackErrors, err)
				}
			} else if !errors.Is(backupStatErr, os.ErrNotExist) {
				rollbackErrors = append(rollbackErrors, backupStatErr)
			}
		} else if err := removeTransactionTarget(destination, tx.InstallDir); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	if err := SaveState(statePath, tx.PreviousState); err != nil {
		rollbackErrors = append(rollbackErrors, err)
	}
	if len(rollbackErrors) != 0 {
		return errors.Join(rollbackErrors...)
	}
	_ = os.RemoveAll(tx.StagingDir)
	_ = os.RemoveAll(tx.BackupDir)
	return os.Remove(journalPath)
}

func removeTransactionTarget(target, installDir string) error {
	relative, err := filepath.Rel(installDir, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to remove transaction target outside install directory")
	}
	if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return os.RemoveAll(target)
}

func cleanupConfirmedTransaction(journalPath string, tx *transaction) error {
	if err := os.RemoveAll(tx.BackupDir); err != nil {
		return err
	}
	if err := os.RemoveAll(tx.StagingDir); err != nil {
		return err
	}
	if err := os.Remove(journalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
