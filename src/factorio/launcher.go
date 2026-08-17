package factorio

import (
	"os/exec"
	"runtime"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

// runFactorio builds the command that executes the Factorio server binary with
// the given arguments. Factorio ships only x86_64 headless builds, so on
// non-amd64 hosts the binary is run through the box64 emulator.
func runFactorio(args ...string) *exec.Cmd {
	config := bootstrap.GetConfig()
	if runtime.GOARCH == "amd64" {
		return exec.Command(config.FactorioBinary, args...)
	}
	return exec.Command("box64", append([]string{config.FactorioBinary}, args...)...)
}
