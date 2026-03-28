package pms

import (
	"fmt"
	"os"
	"path/filepath"
)

// PackageList returns a list of installed packages by reading the vdb directory.
func PackageList(vdb string) ([]string, error) {
	var pkgList []string
	categories, err := os.ReadDir(vdb)
	if err != nil {
		return nil, fmt.Errorf("failed to read vdb directory %s: %w", vdb, err)
	}
	for _, cat := range categories {
		if !cat.IsDir() {
			continue
		}
		catPath := filepath.Join(vdb, cat.Name())
		pkgs, err := os.ReadDir(catPath)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to read category directory %s: %w", catPath, err)
		}
		for _, pkg := range pkgs {
			pkgList = append(pkgList, filepath.Join(cat.Name(), pkg.Name()))
		}
	}
	return pkgList, nil
}
