package main

import (
	"log"

	"github.com/SwalkTech/Skiff/store"
	"github.com/SwalkTech/Skiff/ui"
)

func main() {
	model := &ui.Model{
		Hosts: []store.Host{{
			Address: "example.com",
			User:    "alice",
			Port:    22,
		}},
		Screen: "Skiff\n等待连接",
	}
	if err := ui.Run(model); err != nil {
		log.Fatal(err)
	}
}
