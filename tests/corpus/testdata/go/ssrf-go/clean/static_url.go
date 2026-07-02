package main

import "net/http"

func main() {
	_, _ = http.Get("https://status.example.com/healthz")
}
