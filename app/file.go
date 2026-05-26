package main

import (
	"os"
	"path/filepath"
	"strings"
)

func searchFileWithPerms(dir string, command string, perms os.FileMode) (bool) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, file := range files {
		if file.Name() == command {
			fileInfo, _ := file.Info()
			mode := fileInfo.Mode()
			if mode&perms != 0 {
				return true
			}
		}
	}
	return false
}

func searchCommandInPath(command string) (string, bool){
	path_var := os.Getenv("PATH")
	path_dirs := filepath.SplitList(path_var)

	for _, dir := range path_dirs {
		if ok := searchFileWithPerms(dir, command, os.FileMode(0111)); ok {
			full_path := filepath.Join(dir, command)
			return full_path, true
		}
	}
	return "", false
}

func dirPartsToPath(parts []string) string {
	if len(parts) == 0 {
		return "/"
	}
	return "/" + strings.Join(parts, "/")
}

func openRedirectFile(path string, append bool) *os.File {
	if append {
		f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		return f
	}
	f, _ := os.Create(path)
	return f
}