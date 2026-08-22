package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/somare/karya/internal/config"
	"github.com/somare/karya/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(epicCmd)
	epicCmd.AddCommand(epicNewCmd)
	epicCmd.AddCommand(epicLsCmd)
}

var epicCmd = &cobra.Command{
	Use:   "epic",
	Short: "Manage epics",
}

var epicNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new epic in the active project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := activeProjectDir()
		if err != nil {
			return err
		}
		epic, err := store.CreateEpic(projectDir, args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Created epic %q at %s\n", epic.Name, epic.Dir)
		return nil
	},
}

var epicLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List epics in the active project",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := activeProjectDir()
		if err != nil {
			return err
		}
		epics, err := store.ListEpics(projectDir)
		if err != nil {
			return err
		}
		if len(epics) == 0 {
			fmt.Println("No epics. Run: karya epic new <name>")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "EPIC\tPATH")
		for _, e := range epics {
			fmt.Fprintf(w, "%s\t%s\n", e.Name, e.Dir)
		}
		return w.Flush()
	},
}

// activeProjectDir resolves the active project's directory.
func activeProjectDir() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	if cfg.ActiveProject == "" {
		return "", fmt.Errorf("no active project — run: karya use <project>")
	}
	dir := filepath.Join(config.ProjectsDir(), cfg.ActiveProject)
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("active project %q not found at %s", cfg.ActiveProject, dir)
	}
	return dir, nil
}
