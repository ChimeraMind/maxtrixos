package cleaners

import (
	"fmt"
	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/seeder"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
)

var (
	ChrootDirNamePattern = regexp.MustCompile("([a-zA-Z0-9_]+)-([0-9]{8})")
)

func sortChrootDirs(dirs []string) []string {
	slices.SortFunc(dirs, func(a, b string) int {
		matchesA := ChrootDirNamePattern.FindStringSubmatch(a)
		matchesB := ChrootDirNamePattern.FindStringSubmatch(b)
		if len(matchesA) < 3 || len(matchesB) < 3 {
			return 0
		}
		verA, _ := strconv.Atoi(matchesA[2])
		verB, _ := strconv.Atoi(matchesB[2])
		if verA != verB {
			return verA - verB
		}
		return 0
	})
	return dirs
}

type SeedsCleaner struct {
	cfg config.IConfig
}

func (c *SeedsCleaner) Name() string {
	return "seeds"
}

func (c *SeedsCleaner) Init(cfg config.IConfig) error {
	c.cfg = cfg
	return nil
}

func (c *SeedsCleaner) isDryRun() (bool, error) {
	val, err := c.cfg.GetItem("SeedsCleaner.DryRun")
	if err != nil {
		return false, err
	}
	return val == "true", nil
}

func (c *SeedsCleaner) MinAmountOfSeeds() (int, error) {
	val, err := c.cfg.GetItem("SeedsCleaner.MinAmountOfSeeds")
	if err != nil {
		return 0, err
	}
	amount, err := strconv.Atoi(val)
	if err != nil {
		return 0, err
	}
	return amount, nil
}

func filterChrootEntry(regex *regexp.Regexp, path string, entry os.DirEntry) bool {
	stat, err := os.Lstat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stat image %s: %v\n", path, err)
		return false
	}

	// Only accept files.
	if stat.IsDir() {
		fmt.Fprintf(os.Stdout, "Path %s is a directory. Skipping.\n", path)
		return false
	}

	mode := stat.Mode()
	isFile := mode.IsRegular()
	if !isFile {
		fmt.Fprintf(os.Stdout, "Path %s is not a regular file. Ignoring this file.\n", path)
		return false
	}

	name := entry.Name()
	// Search for a %Y%m%d pattern in the file name. Also, file names have to then have an
	// .img.* pattern too and only some extensions are allowed. So, putting everything together:
	return regex.Match([]byte(name))
}

func (c *SeedsCleaner) Run() error {
	det, err := seeder.NewSeederDetector(c.cfg)
	if err != nil {
		return err
	}

	infos, err := det.Detect(nil, nil)
	if err != nil {
		return err
	}

	fmt.Printf("Detected %d seeds:\n", len(infos))
	for _, info := range infos {
		fmt.Printf(" [%s] %s\n", info.Name, info.Dir)
	}

	opts := seeder.NewSeederOptions{}
	sd, err := seeder.NewSeeder(c.cfg, &opts)
	if err != nil {
		return err
	}
	defer sd.Cleanup()

	minAmountOfSeeds, err := c.MinAmountOfSeeds()
	if err != nil {
		return err
	}

	var markedForRemoval []string

	for _, info := range infos {
		name := seeder.SeederNameWithoutOrderPrefix(info.Name)
		fmt.Printf("[%s] Working on seed %s ...\n", info.Name, name)

		// Parse seeder params.
		paramsName, err := sd.ParamsExecutableName()
		if err != nil {
			return err
		}
		paramsPath := filepath.Join(info.Dir, paramsName)
		if !filesystems.FileExists(paramsPath) {
			fmt.Fprintf(
				os.Stderr,
				"[%s] Params file %s does not exist, skipping ...",
				info.Name, paramsPath,
			)
			continue
		}

		params, err := sd.ParseSeederParams(info.Name, paramsPath)
		if err != nil {
			fmt.Fprintf(
				os.Stderr,
				"[%s] Unable to parse params file %s: %v",
				info.Name, paramsPath, err,
			)
			continue
		}

		if params.LatestAvailableChrootDir == "" {
			fmt.Fprintf(
				os.Stderr,
				"[%s] A latest available chroot dir does not exist. Skipping ...",
				info.Name,
			)
			continue
		}

		if len(params.AllChrootDirs) == 0 {
			fmt.Fprintf(
				os.Stderr,
				"[%s] No chroot dirs exist. Skipping ...",
				info.Name,
			)
			continue
		}

		for _, chrootDir := range params.AllChrootDirs {
			fmt.Printf("[%s] Dir: %s\n", info.Name, chrootDir)
		}
		if len(params.AllChrootDirs) < minAmountOfSeeds {
			fmt.Printf(
				"[%s] Nothing to do. Within the minimum amount of seeds (%d).\n",
				info.Name,
				minAmountOfSeeds,
			)
			continue
		}

		sortedChrootDirs := sortChrootDirs(params.AllChrootDirs)
		for _, chrootDir := range sortedChrootDirs {
			fmt.Printf("[%s|sorted] Dir: %s\n", info.Name, chrootDir)
		}
		// Pick the first N elements up to minAmountOfSeeds
		rmrf := sortedChrootDirs[:len(sortedChrootDirs)-minAmountOfSeeds]
		for _, chrootDir := range rmrf {
			fmt.Printf("[%s|marked] Dir: %s\n", info.Name, chrootDir)
		}
		markedForRemoval = append(markedForRemoval, rmrf...)
	}

	if len(markedForRemoval) == 0 {
		fmt.Println("No seeds to remove.")
		return nil
	}

	dryRun, err := c.isDryRun()
	if err != nil {
		return err
	}

	var errors []error

	for _, path := range markedForRemoval {
		stat, err := os.Stat(path)
		if err != nil {
			errors = append(errors, err)
			fmt.Fprintf(os.Stderr, "Failed to stat path %s: %v\n", path, err)
			continue
		}
		if stat.Mode().IsRegular() {
			fmt.Fprintf(os.Stderr, "Path %s is a regular file. Removing ...\n", path)
			if err := os.Remove(path); err != nil {
				errors = append(errors, err)
				fmt.Fprintf(os.Stderr, "Failed to remove path %s: %v\n", path, err)
			}
			continue
		}

		if !stat.IsDir() {
			errors = append(errors, fmt.Errorf("path %s is not a directory", path))
			fmt.Fprintf(os.Stderr, "Path %s is not a directory. Skipping.\n", path)
			continue
		}

		if !filesystems.DirectoryExists(path) {
			errors = append(errors, fmt.Errorf("directory %s does not exist", path))
			fmt.Fprintf(os.Stderr, "Directory %s does not exist. Skipping.\n", path)
			continue
		}

		if err := filesystems.CheckActiveMounts(path); err != nil {
			errors = append(errors, err)
			fmt.Fprintf(os.Stderr, "Directory %s has active mounts. Skipping.\n", path)
			continue
		}

		if dryRun {
			fmt.Printf("Dry run mode enabled. Not removing %s ...\n", path)
			continue
		}

		fmt.Printf("Removing: %s\n", path)
		if err := os.RemoveAll(path); err != nil {
			errors = append(errors, err)
			fmt.Fprintf(
				os.Stderr, "Failed to remove path %s: %v. Continuing ...\n",
				path,
				err,
			)
			continue
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors occurred during cleanup: %v", errors)
	}
	return nil
}
