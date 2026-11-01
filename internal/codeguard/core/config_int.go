package core

// IntValue reads an optional int setting. Settings whose zero value is a
// meaningful user choice are *int so "unset" (nil) can take the profile
// default while an explicit 0 survives defaulting; after config.ApplyDefaults
// they are non-nil, and a nil here (a hand-built config) reads as 0.
func IntValue(ptr *int) int {
	if ptr == nil {
		return 0
	}
	return *ptr
}
