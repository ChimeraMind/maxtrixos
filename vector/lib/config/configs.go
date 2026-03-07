// package config controls matrixOS development config files loading and config
// params reading.
package config

import "strings"

type IConfig interface {
	// Load loads the associated config file or source.
	Load() error

	// Clone creates a deep copy of the config object, including all its internal state.
	Clone() IConfig

	// AddOverlay adds an overlay of config values that take precedence over the loaded config file.
	// This allows for dynamic overrides of config values without modifying the original config file.
	AddOverlay(overlay map[string][]string) error

	// GetItem retrieves the single config value associated to the provided config key.
	// If multiple values are present, it returns the last one.
	// Config keys can be of type: category.name.
	GetItem(key string) (string, error)

	// GetBool retrieves the single config value associated to the provided config key
	// and casts it to a bool value. This is a shortcut function for config values that
	// are strictly boolean.
	GetBool(key string) (bool, error)

	// GetItems retrieves the config values associated to the provided config key.
	// Config keys can be of type: category.name.
	GetItems(key string) ([]string, error)
}

// FilterEnvKey returns a copy of env with all entries for the given key removed.
func FilterEnvKey(env []string, key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
