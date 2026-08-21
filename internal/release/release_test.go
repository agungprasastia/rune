package release

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseArchiveName(t *testing.T) {
	name, err := ReleaseArchiveName("0.1.0", "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if name != "rune-v0.1.0-linux-x64.tar.gz" {
		t.Fatalf("archive name = %q", name)
	}
}

func TestPackageVersionUsesDevelopmentVersion(t *testing.T) {
	version, err := PackageVersion(t.TempDir())
	if err != nil || version != "dev" {
		t.Fatalf("PackageVersion = %q, %v", version, err)
	}
}

func TestSmokeRejectsMissingArtifact(t *testing.T) {
	_, err := Smoke(context.Background(), SmokeOptions{RootDir: t.TempDir(), GOOS: "linux"})
	if err == nil || !strings.Contains(err.Error(), "build artifact not found") {
		t.Fatalf("Smoke error = %v", err)
	}
}

func TestDefaultBuildOutput(t *testing.T) {
	if got := DefaultBuildOutput("/tmp/root", "linux"); got != filepath.Join("/tmp/root", "rune") {
		t.Fatalf("output = %q", got)
	}
	if got := DefaultBuildOutput("/tmp/root", "windows"); got != filepath.Join("/tmp/root", "rune.exe") {
		t.Fatalf("windows output = %q", got)
	}
}

func TestSHA256ChecksumRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rune")
	if err := os.WriteFile(path, []byte("rune"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseSHA256Checksum(digest + "  rune\n")
	if err != nil || parsed.Checksum != digest || parsed.FileName != "rune" {
		t.Fatalf("checksum = %#v, %v", parsed, err)
	}
}
