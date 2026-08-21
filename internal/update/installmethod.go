package update

import "path/filepath"

// InstallMethod identifies how the running rune binary was installed.
type InstallMethod string

const (
	InstallMethodStandalone InstallMethod = "standalone"
)

// DetectInstallMethod identifies native binaries. Rune has no alternate
// package-manager installation mode.
func DetectInstallMethod(executablePath string) InstallMethod {
	_ = filepath.Dir(executablePath)
	return InstallMethodStandalone
}
