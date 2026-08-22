package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Global struct {
	ActiveProject string `toml:"active_project"`
}

func dir() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "karya")
}

func ProjectsDir() string {
	return filepath.Join(dir(), "projects")
}

func globalConfigPath() string {
	return filepath.Join(dir(), "config.toml")
}

func Load() (*Global, error) {
	var g Global
	path := globalConfigPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &g, nil
	}
	_, err := toml.DecodeFile(path, &g)
	return &g, err
}

func Save(g *Global) error {
	if err := os.MkdirAll(dir(), 0755); err != nil {
		return err
	}
	f, err := os.Create(globalConfigPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(g)
}
