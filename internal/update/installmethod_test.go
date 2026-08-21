package update

import (
	"path/filepath"
	"testing"
)

func TestDetectInstallMethodIsAlwaysStandalone(t *testing.T) {
	if method := DetectInstallMethod(filepath.Join(t.TempDir(), "rune")); method != InstallMethodStandalone {
		t.Fatalf("DetectInstallMethod = %q, want standalone", method)
	}
}
