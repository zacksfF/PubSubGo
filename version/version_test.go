package version

import (
	"fmt"
	"testing"
)

func TestFullVersion(t *testing.T) {
	version := FullVersion()

	expected := fmt.Sprintf("%s@%s", Version, Commit)

	if version != expected {
		t.Fatalf("invalid version returned: %s", version)
	}
}
