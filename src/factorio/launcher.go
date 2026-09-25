package factorio

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

// nativeARMFactorioVersion is the first Factorio version that ships an
// official linux-arm64 headless build. Older versions only provide x86_64
// builds and have to run through box64 on ARM hosts.
var nativeARMFactorioVersion = Version{2, 1, 18, 0}

// hasNativeARMBuild returns true if the given Factorio version ships an
// official linux-arm64 headless build.
func hasNativeARMBuild(version Version) bool {
	return !version.Less(nativeARMFactorioVersion)
}

// needsEmulationFor reports whether the Factorio binary of the given version
// has to run through box64 on the given GOARCH. The explicit goarch parameter
// keeps this function testable on every host.
func needsEmulationFor(goarch string, version Version) bool {
	if goarch == "amd64" {
		return false
	}
	if goarch == "arm64" {
		return !hasNativeARMBuild(version)
	}
	return true
}

// needsEmulation reports whether Factorio has to run through box64 on this
// host for the given installed version.
func needsEmulation(version Version) bool {
	return needsEmulationFor(runtime.GOARCH, version)
}

// factorioBinaryPath returns the binary to execute for the given installed
// Factorio version. arm64 hosts running a version with a native ARM build use
// the bundled bin/arm64 binary, everything else uses the configured path
// (bin/x64 by default).
func factorioBinaryPath(version Version) string {
	config := bootstrap.GetConfig()
	if runtime.GOARCH == "arm64" && !needsEmulation(version) {
		nativeBinary := filepath.Join(config.FactorioDir, "bin", "arm64", "factorio")
		if info, err := os.Stat(nativeBinary); err == nil && !info.IsDir() {
			return nativeBinary
		}
	}
	return config.FactorioBinary
}

// runFactorio builds the command that executes the Factorio server binary with
// the given arguments. Factorio ships official linux-arm64 headless builds
// since 2.1.18, so older releases (and architectures without native builds)
// are run through the box64 emulator.
func runFactorio(version Version, args ...string) *exec.Cmd {
	binary := factorioBinaryPath(version)
	if !needsEmulation(version) {
		return exec.Command(binary, args...)
	}
	return exec.Command("box64", append([]string{binary}, args...)...)
}
