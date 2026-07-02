package main

import (
	"bufio"
	"os"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	path, _ := reader.ReadString('\n')
	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	_ = file
	_ = err
}
