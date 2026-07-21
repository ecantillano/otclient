package launcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type RunResult struct {
	Update      UpdateResult
	Offline     bool
	UpdateError error
}

type Runner struct {
	Updater          *Updater
	Channel          string
	ManifestURL      string
	ClientExecutable string
	ClientArgs       []string
	NoLaunch         bool
	launch           func(string, []string) error
}

func (runner *Runner) Run(ctx context.Context) (RunResult, error) {
	var result RunResult
	if runner.Updater == nil {
		return result, fmt.Errorf("updater is required")
	}
	if err := os.MkdirAll(runner.Updater.StateRoot, 0o700); err != nil {
		return result, err
	}
	runner.Updater.log("launcher_start", "channel="+runner.Channel)
	lock, err := AcquireFileLock(runner.Updater.lockPath())
	if err != nil {
		return result, err
	}
	defer lock.Close()

	result.Update, result.UpdateError = runner.Updater.Update(ctx, runner.Channel, runner.ManifestURL)
	if result.UpdateError != nil {
		runner.Updater.log("update_failed", result.UpdateError.Error())
		_ = runner.Updater.RecordLastError(result.UpdateError)
	} else if result.Update.Status == UpdateApplied {
		runner.Updater.log("update_applied", fmt.Sprintf("from=%s to=%s pending=%t", result.Update.PreviousVersion, result.Update.InstalledVersion, result.Update.PendingConfirmation))
	}
	if runner.NoLaunch {
		if result.Update.PendingConfirmation {
			runner.Updater.log("update_pending", "confirmation deferred by --no-launch")
		}
		return result, result.UpdateError
	}

	clientPath, err := runner.clientPath()
	if err != nil {
		if result.Update.PendingConfirmation {
			_ = runner.Updater.RollbackPending()
		}
		return result, err
	}
	if result.UpdateError != nil {
		if _, journalErr := os.Stat(runner.Updater.journalPath()); journalErr == nil {
			return result, fmt.Errorf("unsafe update transaction remains; refusing offline launch: %w", result.UpdateError)
		} else if !errors.Is(journalErr, os.ErrNotExist) {
			return result, fmt.Errorf("cannot verify update transaction state: %w", journalErr)
		}
		allowed, requiredVersion, policyErr := runner.Updater.OfflineLaunchAllowed()
		if policyErr != nil {
			return result, fmt.Errorf("cannot evaluate offline policy: %w", policyErr)
		}
		if !allowed {
			runner.Updater.log("offline_blocked", "mandatory_version="+requiredVersion)
			return result, fmt.Errorf("offline launch blocked by mandatory release %s: %w", requiredVersion, result.UpdateError)
		}
		info, statErr := os.Stat(clientPath)
		if statErr != nil || !info.Mode().IsRegular() {
			return result, fmt.Errorf("update failed and no installed client is available: %w", result.UpdateError)
		}
		result.Offline = true
		runner.Updater.log("offline_launch", "using last verified client")
	}

	launch := runner.launch
	if launch == nil {
		launch = func(path string, args []string) error {
			command := exec.Command(path, args...)
			command.Dir = runner.Updater.InstallDir
			command.Env = managedLaunchEnvironment()
			command.Stdin = os.Stdin
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			return command.Run()
		}
	}
	if err := launch(clientPath, append([]string(nil), runner.ClientArgs...)); err != nil {
		if result.Update.PendingConfirmation {
			if rollbackErr := runner.Updater.RollbackPending(); rollbackErr != nil {
				runner.Updater.log("rollback_failed", rollbackErr.Error())
				return result, fmt.Errorf("client failed: %v; update rollback failed: %w", err, rollbackErr)
			}
			runner.Updater.log("update_rolled_back", "managed client exited with an error")
		}
		_ = runner.Updater.RecordLastError(err)
		runner.Updater.log("client_failed", err.Error())
		return result, fmt.Errorf("client exited with an error: %w", err)
	}
	if result.Update.PendingConfirmation {
		if err := runner.Updater.ConfirmPending(); err != nil {
			runner.Updater.log("confirm_failed", err.Error())
			return result, fmt.Errorf("confirm update after successful client run: %w", err)
		}
		runner.Updater.log("update_confirmed", "version="+result.Update.InstalledVersion)
	}
	if !result.Offline {
		_ = runner.Updater.RecordLastError(nil)
	}
	return result, nil
}

func managedLaunchEnvironment() []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "THAPPY_MANAGED_LAUNCH") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment, "THAPPY_MANAGED_LAUNCH=1")
}

func (runner *Runner) clientPath() (string, error) {
	relative := runner.ClientExecutable
	if relative == "" {
		relative = runner.Updater.Config.ClientExecutables[runtime.GOOS]
	}
	if relative == "" {
		return "", fmt.Errorf("no client executable configured for %s", runtime.GOOS)
	}
	if matchesRules(relative, []string{".thappy-launcher/"}) {
		return "", fmt.Errorf("client executable cannot be launcher state")
	}
	path, err := secureJoin(runner.Updater.InstallDir, relative)
	if err != nil {
		return "", err
	}
	if err := ensureNoSymlinkParents(runner.Updater.InstallDir, relative, true); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return path, nil
}

func DefaultInstallDir() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err == nil {
		executable = resolved
	}
	return filepath.Dir(executable), nil
}
