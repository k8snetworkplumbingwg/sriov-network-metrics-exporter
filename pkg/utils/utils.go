package utils

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// Comma-separated string flag type
type StringListFlag []string

func (list *StringListFlag) String() string {
	return strings.Join(*list, ",")
}

func (list *StringListFlag) Set(val string) error {
	*list = strings.Split(val, ",")

	for i := range *list {
		(*list)[i] = strings.TrimSpace((*list)[i])
	}

	return nil
}

func ResolveFlag(flag string, path *string) error {
	if err := ResolvePath(path); err != nil {
		return fmt.Errorf("%s - %v", flag, err)
	}

	return nil
}

func ResolvePath(path *string) error {
	if *path == "" {
		return fmt.Errorf("unable to resolve an empty path")
	}

	cleanPath, err := filepath.Abs(*path)
	if err != nil {
		*path = ""
		return fmt.Errorf("unsafe or invalid path specified '%s'\n%v", *path, err)
	}

	evaluatedPath, err := EvalSymlinks(cleanPath)
	if err != nil {
		*path = cleanPath
		return fmt.Errorf("unable to evaluate symbolic links on path '%s'\n%v", *path, err)
	}

	*path = evaluatedPath

	return nil
}

// Required to enable testing (filepath.EvalSymlinks does not support the fs.FS interface that fstest implements)
var EvalSymlinks = filepath.EvalSymlinks

// IsSymLink checks if the given path is a symbolic link.
// Starting from Go 1.25, fstest.MapFS changed how it handles files with fs.ModeSymlink:
// - fs.Stat now follows symlinks and returns info about the target (or error if target doesn't exist)
// - fs.Stat's Mode() no longer returns fs.ModeSymlink for symlink entries
// Using fs.ReadDir instead correctly returns the entry's type without following the symlink,
// allowing proper symlink detection.
func IsSymLink(fsys fs.FS, path string) bool {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.Name() == base {
			return entry.Type()&fs.ModeSymlink != 0
		}
	}

	return false
}
