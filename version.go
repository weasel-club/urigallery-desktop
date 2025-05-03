package main

import (
	"io"
	"net/http"
	"strconv"
)

const (
	VersionURL = "https://urigallery.pages.dev/version"
)

func GetVersion() (int, error) {
	resp, err := http.Get(VersionURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// parse body as string to int
	version, err := strconv.Atoi(string(body))
	if err != nil {
		return 0, err
	}

	return version, nil
}
