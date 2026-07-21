package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxLauncherLogBytes = int64(1 << 20)

var launcherLogMutex sync.Mutex

type logRecord struct {
	Timestamp string `json:"timestamp"`
	Event     string `json:"event"`
	Message   string `json:"message,omitempty"`
}

func AppendDiagnosticLog(stateRoot, event, message string) error {
	if !eventNamePattern.MatchString(event) {
		return fmt.Errorf("invalid log event %q", event)
	}
	launcherLogMutex.Lock()
	defer launcherLogMutex.Unlock()

	logDirectory := filepath.Join(stateRoot, "logs")
	if err := os.MkdirAll(logDirectory, 0o700); err != nil {
		return err
	}
	logPath := filepath.Join(logDirectory, "launcher.log")
	if info, err := os.Stat(logPath); err == nil && info.Size() >= maxLauncherLogBytes {
		_ = os.Remove(logPath + ".2")
		if err := os.Rename(logPath+".1", logPath+".2"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Rename(logPath, logPath+".1"); err != nil {
			return err
		}
	}

	message = redactLogMessage(message)
	if len(message) > 4096 {
		message = message[:4096] + "[truncated]"
	}
	record, err := json.Marshal(logRecord{Timestamp: time.Now().UTC().Format(time.RFC3339), Event: event, Message: message})
	if err != nil {
		return err
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(record, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func redactLogMessage(message string) string {
	message = PublicError(errors.New(message))
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		message = strings.ReplaceAll(message, home, "~")
	}
	return message
}
