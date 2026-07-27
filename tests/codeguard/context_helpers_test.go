package codeguard_test

// contextOff disables the default-enabled Agent Context section for tests
// that isolate another check family's findings and artifacts.
func contextOff() *bool {
	off := false
	return &off
}

// localPrecisionOff keeps older tests focused on the legacy quality finding
// they exercise instead of newer local naming/function/error precision rules.
func localPrecisionOff() *bool {
	off := false
	return &off
}
