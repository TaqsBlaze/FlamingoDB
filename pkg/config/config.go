package config

// Config holds the primary configuration for FlamingoDB.
type Config struct {
	DataDirectory string
	PageSize      uint32
	MaxPages      int
}

// Default returns a standard configuration.
func Default() *Config {
	return &Config{
		DataDirectory: "./data",
		PageSize:      8192, // 8KB is standard for good cache locality
		MaxPages:      10000,
	}
}
