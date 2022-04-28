package utils

import (
	"log"
	"path/filepath"
)

func VerifyPath(path *string) {
	cleanPath, err := filepath.EvalSymlinks(filepath.Clean(*path))
	if err != nil {
		log.Fatalf("Unsafe or invalid path specified, Error: %v", err)
	} else {
		*path = cleanPath
	}
}
