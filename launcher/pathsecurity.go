package launcher

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var windowsReservedNames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

// SafeRelativePath normalizes an archive or manifest path using slash separators.
// It deliberately applies Windows restrictions on every OS because release archives
// are produced once and may later be installed on Windows.
func SafeRelativePath(value string) (string, error) {
	if value == "" || strings.ContainsRune(value, '\x00') {
		return "", fmt.Errorf("path is empty or contains NUL")
	}
	value = strings.ReplaceAll(value, `\`, "/")
	if strings.HasPrefix(value, "/") || strings.Contains(value, ":") {
		return "", fmt.Errorf("path %q is absolute or contains a drive/stream separator", value)
	}

	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path %q escapes the install root", value)
	}

	for _, segment := range strings.Split(cleaned, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("path %q contains an invalid segment", value)
		}
		if strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") {
			return "", fmt.Errorf("path %q has a Windows-ambiguous segment", value)
		}
		base := strings.ToUpper(strings.SplitN(segment, ".", 2)[0])
		if _, reserved := windowsReservedNames[base]; reserved {
			return "", fmt.Errorf("path %q contains reserved Windows name %q", value, segment)
		}
	}
	return cleaned, nil
}

func secureJoin(root, relative string) (string, error) {
	normalized, err := SafeRelativePath(relative)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, filepath.FromSlash(normalized))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes root", relative)
	}
	return target, nil
}

func ensureNoSymlinkParents(root, relative string, includeLeaf bool) error {
	normalized, err := SafeRelativePath(relative)
	if err != nil {
		return err
	}
	parts := strings.Split(normalized, "/")
	if !includeLeaf && len(parts) > 0 {
		parts = parts[:len(parts)-1]
	}
	current := root
	for _, part := range parts {
		current = filepath.Join(current, filepath.FromSlash(part))
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing path through symlink %q", current)
		}
	}
	return nil
}

func canonicalRule(value string) (string, bool, error) {
	prefix := strings.HasSuffix(strings.ReplaceAll(value, `\`, "/"), "/")
	trimmed := strings.TrimSuffix(value, "/")
	normalized, err := SafeRelativePath(trimmed)
	if err != nil {
		return "", false, err
	}
	return strings.ToLower(normalized), prefix, nil
}

func matchesRules(relative string, rules []string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(relative, `\`, "/"))
	for _, rawRule := range rules {
		rule, prefix, err := canonicalRule(rawRule)
		if err != nil {
			continue
		}
		if (!prefix && normalized == rule) || (prefix && (normalized == rule || strings.HasPrefix(normalized, rule+"/"))) {
			return true
		}
	}
	return false
}

func pathsOverlap(left, right string) bool {
	left = strings.ToLower(strings.TrimSuffix(strings.ReplaceAll(left, `\`, "/"), "/"))
	right = strings.ToLower(strings.TrimSuffix(strings.ReplaceAll(right, `\`, "/"), "/"))
	return left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}
