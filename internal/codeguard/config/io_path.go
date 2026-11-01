package config

func DefaultConfigPath() string {
	return defaultConfigNames[0]
}

// ResolveConfigPath returns the file LoadFile would read for path: an empty
// path or a bare default name searches the working directory's default
// locations, and a directory resolves to the config file inside it.
func ResolveConfigPath(path string) (string, error) {
	return resolveConfigPath(path)
}
