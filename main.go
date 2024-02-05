package main

import (
	"log"
	"path/filepath"

	core "github.com/MapoMagpie/rimedm/core"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	opts, configPath := core.ParseOptions()
	f, err := tea.LogToFile(filepath.Dir(configPath)+"/debug.log", "DEBUG")
	if err != nil {
		log.Fatalf("log to file err : %s", err)
	}
	defer func() {
		_ = f.Close()
	}()
	core.Start(&opts)
}
