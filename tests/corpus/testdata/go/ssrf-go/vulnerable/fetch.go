package main

import (
	"net/http"
	"os"
)

func main() {
	target := os.Getenv("TARGET_URL")
	_, _ = http.Get(target)
}
