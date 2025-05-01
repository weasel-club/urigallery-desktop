package util

import (
	"os"
	"os/user"
	"path/filepath"
)

func GetBasePath() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func GetPath(path string) string {
	return filepath.Join(GetBasePath(), path)
}

func GetPicturesDir() (string, error) {
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(user.HomeDir, "Pictures", "VRChat"), nil
}
