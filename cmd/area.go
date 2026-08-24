package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	areaCreateProject string
	areaCreateSlug    string
)

func init() {
	rootCmd.AddCommand(areaCmd)
	areaCmd.AddCommand(areaCreateCmd, areaListCmd, areaGetCmd, areaDeleteCmd)
	areaCreateCmd.Flags().StringVar(&areaCreateProject, "project", "", "Project key")
	areaCreateCmd.Flags().StringVar(&areaCreateSlug, "slug", "", "Area slug")
	areaCreateCmd.MarkFlagRequired("project")
	areaListCmd.Flags().String("project", "", "Project key")
	areaListCmd.MarkFlagRequired("project")
	areaGetCmd.Flags().String("project", "", "Project key")
	areaGetCmd.MarkFlagRequired("project")
	areaDeleteCmd.Flags().String("project", "", "Project key")
	areaDeleteCmd.Flags().Bool("yes", false, "Confirm deletion")
	areaDeleteCmd.MarkFlagRequired("project")
	areaDeleteCmd.MarkFlagRequired("yes")
}

var areaCmd = &cobra.Command{
	Use:   "area",
	Short: "Manage areas",
}

var areaCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create an area",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		area, err := svc.CreateArea(commandContext(), areaCreateProject, args[0], areaCreateSlug)
		if err != nil {
			return err
		}
		return writeResource(cmd, area, func() error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Created area %s: %s\n", area.Slug, area.Name)
			return err
		})
	},
}

var areaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List areas in a project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := cmd.Flags().GetString("project")
		if err != nil {
			return err
		}
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		areas, err := svc.ListAreas(commandContext(), project)
		if err != nil {
			return err
		}
		return writeResource(cmd, areas, func() error {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SLUG\tNAME")
			for _, area := range areas {
				fmt.Fprintf(w, "%s\t%s\n", area.Slug, area.Name)
			}
			return w.Flush()
		})
	},
}

var areaGetCmd = &cobra.Command{
	Use:   "get <slug>",
	Short: "Get an area",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := cmd.Flags().GetString("project")
		if err != nil {
			return err
		}
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		area, err := svc.GetArea(commandContext(), project, args[0])
		if err != nil {
			return err
		}
		return writeResource(cmd, area, func() error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", area.Slug, area.Name)
			return err
		})
	},
}

var areaDeleteCmd = &cobra.Command{
	Use:   "delete <slug>",
	Short: "Delete an area",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := cmd.Flags().GetString("project")
		if err != nil {
			return err
		}
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
		area, err := svc.GetArea(commandContext(), project, args[0])
		if err != nil {
			return err
		}
		if err := svc.DeleteArea(commandContext(), project, area.Slug); err != nil {
			return err
		}
		return writeDeleted(cmd, area.Slug)
	},
}
