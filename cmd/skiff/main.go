package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/SwalkTech/Skiff/ui"
)

func main() {
	path, err := hostsPath()
	if err != nil {
		log.Fatal(err)
	}
	model, err := ui.Load(path)
	if err != nil {
		log.Fatal(err)
	}
	if err := ui.Run(model); err != nil {
		log.Fatal(err)
	}
}

func hostsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "Skiff")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "hosts.json"), nil
}
