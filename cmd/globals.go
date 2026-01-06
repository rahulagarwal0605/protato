// Package cmd provides CLI command implementations.
package cmd

// GlobalOptions contains global CLI options (flags and environment variables).
type GlobalOptions struct {
	CacheDir    string `help:"Registry cache directory" env:"PROTATO_REGISTRY_CACHE" default:"${defaultCacheDir}"`
	RegistryURL string `help:"Registry Git URL" env:"PROTATO_REGISTRY_URL"`
}
