package fix

func goTestPattern(dir string) (string, string) {
	if dir == "." {
		return ".", "go test ."
	}
	pattern := "./" + dir
	return pattern, "go test " + pattern
}
