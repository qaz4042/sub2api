package devdata

import (
	"os"
	"path/filepath"
	"strings"
)

const DirName = ".dev-data"

func FindNearest() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(dir, DirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func FindNearestOutsideTests() string {
	if strings.HasSuffix(os.Args[0], ".test") {
		return ""
	}
	return FindNearest()
}
