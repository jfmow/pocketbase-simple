package tools

import (
	"os"
	"path/filepath"
	"strings"
)

func GetWorkDir() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}

	dir := filepath.Dir(ex)

	// Helpful when developing:
	// when running `go run`, the executable is in a temporary directory.
	if strings.Contains(dir, "go-build") {

		return "."
	}
	return filepath.Dir(ex)
}
