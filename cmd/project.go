package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/somare/karya/internal/config"
	"github.com/somare/karya/internal/model"
	"github.com/somare/karya/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(useCmd)

	projectCmd.AddCommand(projectNewCmd)
	projectCmd.AddCommand(projectLsCmd)

	projectNewCmd.Flags().StringVar(&projectNewPrefix, "prefix", "", "ID prefix (e.g. MYAPP)")
	projectNewCmd.MarkFlagRequired("prefix")
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
}

// --- project new ---

var projectNewPrefix string

var projectNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		projectsDir := config.ProjectsDir()
		slug := filepath.Join(projectsDir, toProjectSlug(name))

		if _, err := os.Stat(slug); err == nil {
			return fmt.Errorf("project %q already exists", name)
		}

		p := &model.Project{Name: name, Prefix: projectNewPrefix}
		if err := store.WriteProjectConfig(slug, p); err != nil {
			return err
		}

		fmt.Printf("Created project %q (prefix: %s) at %s\n", name, projectNewPrefix, slug)
		return nil
	},
}

// --- project ls ---

var projectLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		projects, err := store.ListProjects(config.ProjectsDir())
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			fmt.Println("No projects. Run: karya project new <name> --prefix <PREFIX>")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tPREFIX\t")
		for _, p := range projects {
			active := ""
			if p.Name == cfg.ActiveProject {
				active = "*"
			}
			fmt.Fprintf(w, "%s%s\t%s\t\n", active, p.Name, p.Prefix)
		}
		return w.Flush()
	},
}

// --- use ---

var useCmd = &cobra.Command{
	Use:   "use <project>",
	Short: "Set the active project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		projects, err := store.ListProjects(config.ProjectsDir())
		if err != nil {
			return err
		}
		for _, p := range projects {
			if p.Name == name {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				cfg.ActiveProject = name
				if err := config.Save(cfg); err != nil {
					return err
				}
				fmt.Printf("Active project set to %q\n", name)
				return nil
			}
		}
		return fmt.Errorf("project %q not found", name)
	},
}

func toProjectSlug(name string) string {
	return name
}
