package launcher

import "testing"

func TestSemanticVersionOrdering(t *testing.T) {
	ordered := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0",
	}
	for index := 0; index < len(ordered)-1; index++ {
		left, leftErr := ParseVersion(ordered[index])
		right, rightErr := ParseVersion(ordered[index+1])
		if leftErr != nil || rightErr != nil {
			t.Fatalf("parse versions: %v %v", leftErr, rightErr)
		}
		if left.Compare(right) >= 0 {
			t.Fatalf("expected %s before %s", left, right)
		}
	}
}

func TestSemanticVersionRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"1", "1.2", "01.2.3", "1.2.3-01", "v1.2.3", "1.2.3+"} {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseVersion(value); err == nil {
				t.Fatalf("expected %q to be rejected", value)
			}
		})
	}
}
