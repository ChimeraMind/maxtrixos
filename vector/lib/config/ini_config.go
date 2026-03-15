package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type searchPath struct {
	fileName      string
	dirPath       string
	confRoot      string
	artifactsRoot string
	defaultRoot   string
}

// ConfigPath returns the full path to the config file for this search path.
func (sp *searchPath) ConfigPath() string {
	return filepath.Join(sp.dirPath, sp.fileName)
}

const (
	// BaseConfigFileName is the name of the main configuration file that vector looks for.
	BaseConfigFileName = "matrixos.conf"
	// ClientConfigFileName is the name of the client configuration file that vector looks for.
	ClientConfigFileName = "client.conf"
	// MarkerFileName is the name of the marker file that indicates the root of the matrixOS toolkit.
	MarkerFileName = ".matrixos"
)

// smartRootify translates matrixOS.Root into a path that's complying with the config var
// specifications.
func smartRootify(path, defaultRoot string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	// Get the working directory so that we can compare it with path.
	if defaultRoot == "" {
		var err error
		defaultRoot, err = os.Getwd()
		if err != nil {
			return defaultRoot, err
		}
	}

	rootPath := filepath.Join(defaultRoot, path)

	return filepath.Abs(rootPath)
}

func findMarkerDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	goUp := func() bool {
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return false
		}
		cwd = parent
		return true
	}

	for {
		dotMatrixosPath := filepath.Join(cwd, MarkerFileName)
		if _, err := os.Stat(dotMatrixosPath); err != nil {
			if os.IsNotExist(err) {
				if !goUp() {
					break
				}
				continue
			}
			// Error found, and is not "not exist".
			return "", err
		}

		return cwd, nil
	}

	return "", fmt.Errorf("market path not found in any parent directories of %s", cwd)
}

func searchPaths(cfgName string) []searchPath {
	// Navigate CWD up until we find a .matrixos file.
	var sps []searchPath

	markerDir, err := findMarkerDir()
	if err == nil && markerDir != "" {
		sps = append(sps, searchPath{
			fileName:      cfgName,
			dirPath:       filepath.Join(markerDir, "conf"),
			confRoot:      markerDir,
			artifactsRoot: markerDir,
			defaultRoot:   markerDir,
		})
	}

	// add this as last resort option at the moment.
	sps = append(sps, searchPath{
		// Setup for when vector runs from an installed location,
		// with config in /etc/matrixos/conf.
		fileName:      cfgName,
		dirPath:       "/etc/matrixos/conf",
		confRoot:      "/etc/matrixos",
		artifactsRoot: "/var/cache/matrixos",
		defaultRoot:   "/usr/lib/matrixos",
	})

	return sps
}

// IniConfig is a config reader that loads values from an INI file.
type IniConfig struct {
	mu  sync.RWMutex
	sp  *searchPath
	cfg map[string][]string
}

func cfgNameToSearchPath(cfgName string) *searchPath {
	for _, sp := range searchPaths(cfgName) {
		searchFullPath := sp.ConfigPath()
		if _, err := filepath.Abs(searchFullPath); err != nil {
			continue
		}
		if _, err := os.Stat(searchFullPath); err == nil {
			return &sp
		}
	}
	return nil
}

// NewBaseConfig creates a new IniConfig instance for the base configuration.
func NewBaseConfig() (*IniConfig, error) {
	return NewIniConfig(BaseConfigFileName)
}

// NewClientConfig creates a new IniConfig instance for the client configuration.
func NewClientConfig() (*IniConfig, error) {
	return NewIniConfig(ClientConfigFileName)
}

// NewIniConfig creates a new IniConfig instance.
func NewIniConfig(configName string) (*IniConfig, error) {
	sp := cfgNameToSearchPath(configName)
	if sp == nil {
		return nil, fmt.Errorf(
			"config file not found in any of the paths: %v",
			searchPaths(configName),
		)
	}
	return &IniConfig{
		sp: sp,
	}, nil
}

// ConfigFromPathParams holds parameters for creating a config from a specific path.
type ConfigFromPathParams struct {
	ConfigPath    string
	DefaultRoot   string
	ConfRoot      string
	ArtifactsRoot string
}

// NewIniConfigFromPath creates a new IniConfig instance with the specified file path.
func NewIniConfigFromPath(params *ConfigFromPathParams) (*IniConfig, error) {
	if params == nil {
		return nil, fmt.Errorf("params is nil")
	}
	if params.ConfigPath == "" {
		return nil, fmt.Errorf("config path is empty")
	}
	if params.DefaultRoot == "" {
		return nil, fmt.Errorf("default root is empty")
	}
	if params.ConfRoot == "" {
		return nil, fmt.Errorf("conf root is empty")
	}
	if params.ArtifactsRoot == "" {
		return nil, fmt.Errorf("artifacts root is empty")
	}
	sp := searchPath{
		fileName:      filepath.Base(params.ConfigPath),
		dirPath:       filepath.Dir(params.ConfigPath),
		defaultRoot:   params.DefaultRoot,
		confRoot:      params.ConfRoot,
		artifactsRoot: params.ArtifactsRoot,
	}
	return &IniConfig{
		sp: &sp,
	}, nil
}

// Clone creates a deep copy of the IniConfig instance.
// This is useful for forking off configs with overlays,
// without mutating the original config.
func (c *IniConfig) Clone() IConfig {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	var sp searchPath
	if c.sp != nil {
		sp = *(c.sp) // copy.
	}
	clone := &IniConfig{
		sp:  &sp,
		cfg: make(map[string][]string),
	}
	for k, v := range c.cfg {
		clone.cfg[k] = slices.Clone(v)
	}
	return clone
}

// AddOverlay adds the provided overlay to the config.
// The overlay is a map where keys are config keys and values
// are slices of config values.
func (c *IniConfig) AddOverlay(overlay map[string][]string) error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if overlay == nil {
		return fmt.Errorf("overlay is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	for k, v := range overlay {
		vals, ok := c.cfg[k]
		if !ok {
			c.cfg[k] = v
			continue
		}
		vals = append(vals, v...)
		c.cfg[k] = vals
	}
	return nil
}

func (c *IniConfig) loadAndGenerateConfig(configPath string) error {
	ini, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config %s: %w", configPath, err)
	}
	c.generateConfig(ini)
	return nil
}

func (c *IniConfig) generateSubConfigs(configPath string) error {
	// configPath is a valid path to a config file.
	// Use this path to build a list of subconfigs to load, starting
	// with configPath + ".d/*.conf".
	subconfigDir := configPath + ".d"
	subconfigs, err := os.ReadDir(subconfigDir)
	if err != nil {
		// If the directory doesn't exist, that's fine.
		// it just means there are no subconfigs.
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf(
			"failed to read subconfig directory %s: %w",
			subconfigDir,
			err,
		)
	}

	for _, entry := range subconfigs {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".conf" {
			continue
		}
		subconfigPath := filepath.Join(subconfigDir, entry.Name())
		err := c.loadAndGenerateConfig(subconfigPath)
		if err != nil {
			return fmt.Errorf(
				"failed to load subconfig %s: %w",
				subconfigPath,
				err,
			)
		}
	}

	return nil
}

func (c *IniConfig) generateConfig(ini IniFile) {
	if c.cfg == nil {
		c.cfg = make(map[string][]string)
	}

	for section, items := range ini {
		for key, value := range items {
			// Flatten the key: [Section] Key -> Section.Key
			var fullKey string
			if section == "" {
				fullKey = key
			} else {
				fullKey = fmt.Sprintf("%s.%s", section, key)
			}

			val, ok := c.cfg[fullKey]
			if !ok {
				val = []string{}
			}
			val = append(val, value) // preserve history.
			c.cfg[fullKey] = val
		}
	}
}

func (c *IniConfig) generateParent(ini IniFile) error {
	mos, ok := ini["matrixOS"]
	if !ok {
		return nil
	}
	parentVal, ok := mos["ParentConfig"]
	if !ok {
		return nil
	}
	parentPath := filepath.Join(c.sp.dirPath, parentVal)
	if _, err := os.Stat(parentPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	c.loadAndGenerateConfig(parentPath)
	return nil
}

func (c *IniConfig) loadRootConfigs(fullPath string) error {
	rootDependents := []string{
		"matrixOS.PrivateGitRepoPath",
		"matrixOS.LogsDir",
		"matrixOS.LocksDir",
		"Seeder.LocksDir",
		"Releaser.HooksDir",
		"Seeder.SeedersDir",
		"Releaser.LocksDir",
		"Imager.LocksDir",
		"Ostree.RepoDir",
	}

	// Set defaults for base paths if missing, to allow expansion
	rootVal, foundRoot := c.getVal("matrixOS.Root")
	if !foundRoot {
		log.Printf(
			`WARNING WARNING WARNING:
- matrixOS.Root is not set in %s.
- Relative paths depending on it will be expanded using %s.
- Those paths are:
  - %s`,
			fullPath,
			c.sp.defaultRoot,
			strings.Join(rootDependents, "\n  - "),
		)
		c.setVal("matrixOS.Root", c.sp.defaultRoot)
	} else {
		rootVal, err := smartRootify(rootVal, c.sp.defaultRoot)
		if err != nil {
			return err
		}
		c.setVal("matrixOS.Root", rootVal)
	}

	// Expand base paths to absolute
	if err := c.expandAbs("matrixOS.Root"); err != nil {
		return err
	}

	for _, key := range rootDependents {
		c.expand(key, "matrixOS.Root")
	}

	return nil
}

func (c *IniConfig) loadDefaultRootConfigs(fullPath string) error {
	defaultRootDependents := []string{
		"Seeder.ChrootSeedersDir",
	}

	// Some very minimal sanity checks at this stage.
	_, foundDefaultRoot := c.getVal("matrixOS.DefaultRoot")
	if !foundDefaultRoot {
		log.Printf(
			`WARNING WARNING WARNING:
- matrixOS.DefaultRoot is not set in %s.
- Relative paths depending on it will be expanded using %s.
- Those paths are:
  - %s`,
			fullPath,
			c.sp.defaultRoot,
			strings.Join(defaultRootDependents, "\n  - "),
		)
		c.setVal("matrixOS.DefaultRoot", c.sp.defaultRoot)
	}

	for _, key := range defaultRootDependents {
		c.expand(key, "matrixOS.DefaultRoot")
	}
	return nil
}

func (c *IniConfig) loadConfRootConfigs(fullPath string) error {
	confRootDependents := []string{
		"Seeder.GpgKeysDir",
		"Ostree.GpgOfficialPublicKey",
	}

	confRootVal, foundConfRoot := c.getVal("matrixOS.ConfRoot")
	if !foundConfRoot {
		log.Printf(
			`WARNING WARNING WARNING:
- matrixOS.ConfRoot is not set in %s.
- Relative paths depending on it will be expanded using %s.
- Those paths are:
  - %s`,
			fullPath,
			c.sp.confRoot,
			strings.Join(confRootDependents, "\n  - "),
		)
		c.setVal("matrixOS.ConfRoot", c.sp.confRoot)
	} else {
		confRootVal, err := smartRootify(confRootVal, c.sp.confRoot)
		if err != nil {
			return err
		}
		c.setVal("matrixOS.ConfRoot", confRootVal)
	}

	// Expand paths depending on base paths.
	for _, key := range confRootDependents {
		c.expand(key, "matrixOS.ConfRoot")
	}
	return nil
}

func (c *IniConfig) loadArtifactsRootConfigs(fullPath string) error {
	artifactsRootDependents := []string{
		"Ostree.DevGpgHomeDir",
		"Seeder.DownloadsDir",
		"Seeder.DistfilesDir",
		"Seeder.BinpkgsDir",
		"Seeder.PortageReposDir",
		"Imager.ImagesDir",
		"Imager.MountDir",
	}

	artifactsRootVal, foundArtifactsRoot := c.getVal("matrixOS.ArtifactsRoot")
	if !foundArtifactsRoot {
		log.Printf(
			`WARNING WARNING WARNING:
- matrixOS.ArtifactsRoot is not set in %s.
- Relative paths depending on it will be expanded using %s.
- Those paths are:
  - %s`,
			fullPath,
			c.sp.defaultRoot,
			strings.Join(artifactsRootDependents, "\n  - "),
		)
		c.setVal("matrixOS.ArtifactsRoot", c.sp.defaultRoot)
	} else {
		artifactsRootVal, err := smartRootify(artifactsRootVal, c.sp.defaultRoot)
		if err != nil {
			return err
		}
		c.setVal("matrixOS.ArtifactsRoot", artifactsRootVal)
	}

	// Expand paths depending on base paths.
	for _, key := range artifactsRootDependents {
		c.expand(key, "matrixOS.ArtifactsRoot")
	}
	return nil
}

func (c *IniConfig) loadPrivateRepoConfigs() error {
	privateRepoDependents := []string{
		"Seeder.SecureBootPrivateKey",
		"Seeder.SecureBootPublicKey",
		"Seeder.SecureBootKekPublicKey",
		"Ostree.GpgPrivateKey",
		"Ostree.GpgPublicKey",
	}

	if err := c.loadPrivateRepoConfigs(); err != nil {
		return err
	}
	return nil
}

func (c *IniConfig) Load() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sp == nil {
		return fmt.Errorf("no configuration found in any of the search paths.")
	}
	fullPath := c.sp.ConfigPath()
	ini, err := LoadConfig(fullPath)
	if err != nil {
		return err
	}

	c.generateParent(ini)
	c.generateConfig(ini)
	if err := c.generateSubConfigs(fullPath); err != nil {
		return err
	}

	if err := c.loadRootConfigs(fullPath); err != nil {
		return err
	}

	if err := c.loadDefaultRootConfigs(fullPath); err != nil {
		return err
	}

	if err := c.loadConfRootConfigs(fullPath); err != nil {
		return err
	}

	if err := c.loadArtifactsRootConfigs(fullPath); err != nil {
		return err
	}

	if err := c.loadPrivateRepoConfigs(); err != nil {
		return err
	}

	return nil
}

// GetItem retrieves the single config value associated to the provided config key.
// If multiple values are present, it returns the last one.
func (c *IniConfig) GetItem(key string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("config is nil")
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	lst, ok := c.cfg[key]
	if !ok {
		return "", fmt.Errorf("invalid key %s", key)
	}

	var val string
	if len(lst) > 0 {
		val = lst[len(lst)-1]
	}
	return val, nil
}

func (c *IniConfig) GetBool(key string) (bool, error) {
	val, err := c.GetItem(key)
	if err != nil {
		return false, err
	}
	return val == "true", nil
}

func (c *IniConfig) GetItems(key string) ([]string, error) {
	var vals []string
	if c == nil {
		return vals, fmt.Errorf("config is nil")
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	lst, ok := c.cfg[key]
	if !ok {
		return vals, fmt.Errorf("invalid key %s", key)
	}
	return lst, nil
}

func (c *IniConfig) getVal(key string) (string, bool) {
	if vals, ok := c.cfg[key]; ok && len(vals) > 0 {
		return vals[len(vals)-1], true
	}
	return "", false
}

func (c *IniConfig) setVal(key, val string) {
	if vals, ok := c.cfg[key]; ok && len(vals) > 0 {
		vals[len(vals)-1] = val
		return
	}
	c.cfg[key] = []string{val}
}

func (c *IniConfig) expandAbs(key string) error {
	val, ok := c.getVal(key)
	if !ok {
		return nil
	}
	if !filepath.IsAbs(val) {
		abs, err := filepath.Abs(val)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for %s: %w", key, err)
		}
		c.setVal(key, abs)
	}
	return nil
}

func (c *IniConfig) expand(key, baseKey string) {
	val, ok := c.getVal(key)
	if !ok {
		return
	}
	if filepath.IsAbs(val) {
		return
	}
	base, ok := c.getVal(baseKey)
	if !ok {
		return
	}
	c.setVal(key, filepath.Join(base, val))
}
