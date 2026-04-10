package config

// NormalizeArgs is a test export of normalizeArgs.
func NormalizeArgs(args []string) []string { return normalizeArgs(args) }

// Load is a test export of load.
func Load(args []string) (Config, error) { return load(args) }

// ExpandEnvVars is a test export of expandEnvVars.
func ExpandEnvVars(s string) string { return expandEnvVars(s) }
