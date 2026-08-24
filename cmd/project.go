package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var projectCreateKey string

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectCreateCmd, projectListCmd, projectGetCmd, projectDeleteCmd)
	projectCreateCmd.Flags().StringVar(&projectCreateKey, "key", "", "Project key")
	projectCreateCmd.MarkFlagRequired("key")
	projectDeleteCmd.Flags().Bool("yes", false, "Confirm deletion")
	projectDeleteCmd.MarkFlagRequired("yes")
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
}

var projectCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		project, err := svc.CreateProject(commandContext(), projectCreateKey, args[0])
		if err != nil {
			return err
		}
		return writeResource(cmd, project, func() error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Created project %s: %s\n", project.Key, project.Name)
			return err
		})
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		projects, err := svc.ListProjects(commandContext())
		if err != nil {
			return err
		}
		return writeResource(cmd, projects, func() error {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "KEY\tNAME")
			for _, project := range projects {
				fmt.Fprintf(w, "%s\t%s\n", project.Key, project.Name)
			}
			return w.Flush()
		})
	},
}

var projectGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		project, err := svc.GetProject(commandContext(), args[0])
		if err != nil {
			return err
		}
		return writeResource(cmd, project, func() error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", project.Key, project.Name)
			return err
		})
	},
}

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, err := cmd.Flags().GetBool("yes")
		if err != nil {
			return err
		}
		if !yes {
			return fmt.Errorf("--yes must be true")
		}
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		project, err := svc.GetProject(commandContext(), args[0])
		if err != nil {
			return err
		}
		if err := svc.DeleteProject(commandContext(), project.Key); err != nil {
			return err
		}
		return writeDeleted(cmd, project.Key)
	},
}
