package fixtures

import "os/exec"

func restartService(name string) error {
	return exec.Command("systemctl", "restart", name).Run()
}
