package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/SwalkTech/Skiff/ui"
)

func main() {
	defer func() {
		if recovered := recover(); recovered != nil {
			writeCrash(fmt.Sprintf("panic: %v\n%s", recovered, debug.Stack()))
		}
	}()
	path, err := hostsPath()
	if err != nil {
		writeCrash(err.Error())
		log.Fatal(err)
	}
	model, err := ui.Load(path)
	if err != nil {
		writeCrash(err.Error())
		log.Fatal(err)
	}
	if err := ui.Run(model); err != nil {
		writeCrash(err.Error())
		log.Fatal(err)
	}
}

func writeCrash(text string) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	dir = filepath.Join(dir, "Skiff")
	_ = os.MkdirAll(dir, 0o700)
	_ = os.WriteFile(filepath.Join(dir, "crash.log"), []byte(text), 0o600)
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
