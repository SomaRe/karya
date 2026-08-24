package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/somare/karya/internal/service"
	"github.com/somare/karya/internal/sqlite"
	"github.com/spf13/cobra"
)

var (
	dbPath     string
	jsonOutput bool
	store      *sqlite.Store
	app        *service.Service
)

var rootCmd = &cobra.Command{
	Use:           "karya",
	Short:         "A SQLite-backed tracker for projects, areas, and tickets",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "docs" {
			return nil
		}
		path, err := databasePath(dbPath)
		if err != nil {
			return err
		}
		if dbPath == "" {
			if err := protectDefaultDatabaseDir(path); err != nil {
				return err
			}
		}
		store, err = sqlite.Open(path)
		if err != nil {
			return err
		}
		app = service.New(store)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "SQLite database path")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output JSON")
}

func Execute() {
	err := rootCmd.Execute()
	if store != nil {
		if closeErr := store.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		store = nil
		app = nil
	}
	if err != nil {
		fmt.Fprintln(rootCmd.ErrOrStderr(), err)
		os.Exit(1)
	}
}

func databasePath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "karya", "karya.db"), nil
}

func protectDefaultDatabaseDir(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("protect database directory: %w", err)
	}
	return nil
}

func serviceFor(cmd *cobra.Command) (*service.Service, error) {
	if app == nil {
		return nil, fmt.Errorf("initialize database for %s", cmd.CommandPath())
	}
	return app, nil
}

func commandContext() context.Context {
	return context.Background()
}
