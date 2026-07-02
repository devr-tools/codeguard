package codeguard_test

// contextOff disables the default-enabled Agent Context section for tests
// that isolate another check family's findings and artifacts.
func contextOff() *bool {
	off := false
	return &off
}
