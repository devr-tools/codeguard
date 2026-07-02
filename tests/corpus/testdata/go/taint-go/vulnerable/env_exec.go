package main

import (
	"os"
	"os/exec"
)

func main() {
	userCmd := os.Getenv("USER_CMD")
	alias := userCmd
	_ = exec.Command("sh", "-c", alias)
}
