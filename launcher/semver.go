package launcher

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

// Version is a strict Semantic Versioning 2.0.0 value.
type Version struct {
	Major      uint64
	Minor      uint64
	Patch      uint64
	Prerelease []string
	raw        string
}

func ParseVersion(value string) (Version, error) {
	matches := semverPattern.FindStringSubmatch(value)
	if matches == nil {
		return Version{}, fmt.Errorf("invalid semantic version %q", value)
	}

	parts := make([]uint64, 3)
	for i := range parts {
		parsed, err := strconv.ParseUint(matches[i+1], 10, 64)
		if err != nil {
			return Version{}, fmt.Errorf("invalid semantic version %q: %w", value, err)
		}
		parts[i] = parsed
	}

	var prerelease []string
	if matches[4] != "" {
		prerelease = strings.Split(matches[4], ".")
		for _, identifier := range prerelease {
			if identifier == "" {
				return Version{}, fmt.Errorf("invalid semantic version %q", value)
			}
			if isNumeric(identifier) && len(identifier) > 1 && identifier[0] == '0' {
				return Version{}, fmt.Errorf("invalid semantic version %q: numeric prerelease has a leading zero", value)
			}
		}
	}

	return Version{Major: parts[0], Minor: parts[1], Patch: parts[2], Prerelease: prerelease, raw: value}, nil
}

func (v Version) String() string { return v.raw }

func (v Version) IsPrerelease() bool { return len(v.Prerelease) != 0 }

// Compare returns -1, 0, or 1 when v is less than, equal to, or greater than other.
func (v Version) Compare(other Version) int {
	for _, pair := range [][2]uint64{{v.Major, other.Major}, {v.Minor, other.Minor}, {v.Patch, other.Patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}

	if len(v.Prerelease) == 0 && len(other.Prerelease) == 0 {
		return 0
	}
	if len(v.Prerelease) == 0 {
		return 1
	}
	if len(other.Prerelease) == 0 {
		return -1
	}

	limit := len(v.Prerelease)
	if len(other.Prerelease) < limit {
		limit = len(other.Prerelease)
	}
	for i := 0; i < limit; i++ {
		left, right := v.Prerelease[i], other.Prerelease[i]
		leftNumeric, rightNumeric := isNumeric(left), isNumeric(right)
		switch {
		case leftNumeric && rightNumeric:
			leftValue, _ := strconv.ParseUint(left, 10, 64)
			rightValue, _ := strconv.ParseUint(right, 10, 64)
			if leftValue < rightValue {
				return -1
			}
			if leftValue > rightValue {
				return 1
			}
		case leftNumeric:
			return -1
		case rightNumeric:
			return 1
		default:
			if left < right {
				return -1
			}
			if left > right {
				return 1
			}
		}
	}

	if len(v.Prerelease) < len(other.Prerelease) {
		return -1
	}
	if len(v.Prerelease) > len(other.Prerelease) {
		return 1
	}
	return 0
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
