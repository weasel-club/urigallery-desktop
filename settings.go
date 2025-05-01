package main

import (
	"bytes"
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

type Settings struct {
	path        string
	Token       string `toml:"token"`
	PicturesDir string `toml:"pictures_dir"`
}

func (s *Settings) decode(text string) error {
	_, err := toml.Decode(text, s)
	return err
}

func (s *Settings) encode() ([]byte, error) {
	var buf bytes.Buffer
	err := toml.NewEncoder(&buf).Encode(s)
	return buf.Bytes(), err
}

func defaultSettings(path string) Settings {
	return Settings{path: path}
}

func (s *Settings) Load() error {
	text, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	return s.decode(string(text))
}
func (s *Settings) Save() error {
	text, err := s.encode()
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, text, 0644)
}

func LoadOrDefaultSettings(path string) Settings {
	settings := defaultSettings(path)

	err := settings.Load()
	if err != nil {
		settings = defaultSettings(path)
	}

	err = settings.Save()
	if err != nil {
		log.Fatal(err)
	}

	return settings
}
