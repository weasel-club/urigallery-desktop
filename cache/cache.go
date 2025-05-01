package cache

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"urigallery/util"
)

type CacheEntry struct {
	*os.File
	Size int64
}

var cacheDir = util.GetPath(".cache")

func init() {
	os.MkdirAll(cacheDir, 0755)
}

func path(key string) string {
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(cacheDir, fmt.Sprintf("%x", hash))
}

func Get(key string) (*CacheEntry, error) {
	filePath := path(key)
	file, err := os.Open(filePath)
	if err != nil {
		file.Close()
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	return &CacheEntry{
		File: file,
		Size: info.Size(),
	}, nil
}

func Set(key string, value []byte) error {
	filePath := path(key)

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	_, err = file.Write(value)
	if err != nil {
		return err
	}

	return nil
}

func Delete(key string) error {
	filePath := path(key)
	return os.Remove(filePath)
}

func Clear() error {
	return os.RemoveAll(cacheDir)
}

func Exists(key string) bool {
	filePath := path(key)
	_, err := os.Stat(filePath)
	return err == nil
}
