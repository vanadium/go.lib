package envutil

import (
	"fmt"
	"os/exec"
	"strings"
)

// Arch returns the architecture of the physical machine using
// the "uname -m" command.
func Arch() (string, error) {
	out, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return "", fmt.Errorf("'uname -m' command failed: %v", err)
	}
	machine := string(out)
	arch := ""
	if strings.Contains(machine, "x86_64") || strings.Contains(machine, "amd64") {
		arch = "amd64"
	} else if strings.HasSuffix(machine, "86") {
		arch = "386"
	} else if strings.Contains(machine, "arm") {
		arch = "arm"
	} else {
		return "", fmt.Errorf("unknown architecture: %s", machine)
	}
	return arch, nil
}
