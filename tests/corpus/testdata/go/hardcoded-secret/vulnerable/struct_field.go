package fixtures

type clientConfig struct {
	Host     string
	Password string
}

// defaultConfig embeds a quoted secret in realistic configuration code with
// the assignment spelled through a struct field.
func defaultConfig() clientConfig {
	cfg := clientConfig{Host: "db.example.net"}
	cfg.Password = "hunter2hunter2"
	return cfg
}
