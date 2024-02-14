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
var EvalSymlinks = func(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

func IsSymLink(fsys fs.FS, path string) bool {
	if info, err := fs.Stat(fsys, path); err != nil {
		return false
	} else if info.Mode() == fs.ModeSymlink {
		return true
	}

	return false
}
