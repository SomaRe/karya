package cmd

import (
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/somare/karya/internal/domain"
	"github.com/somare/karya/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(ticketCmd)
	ticketCmd.AddCommand(ticketCreateCmd, ticketListCmd, ticketGetCmd, ticketUpdateCmd, ticketDeleteCmd, ticketNoteCmd)
	ticketNoteCmd.AddCommand(ticketNoteAddCmd, ticketNoteListCmd)

	for _, command := range []*cobra.Command{ticketCreateCmd, ticketListCmd, ticketGetCmd, ticketUpdateCmd, ticketDeleteCmd, ticketNoteAddCmd, ticketNoteListCmd} {
		command.Flags().String("project", "", "Project key")
		command.MarkFlagRequired("project")
	}
	ticketCreateCmd.Flags().String("area", "", "Area slug")
	ticketCreateCmd.Flags().String("parent", "", "Parent ticket key")
	ticketCreateCmd.Flags().String("description", "", "Ticket description")
	ticketCreateCmd.Flags().String("type", "task", "Ticket type (task|bug|spike)")
	ticketCreateCmd.Flags().String("priority", "medium", "Ticket priority (low|medium|high)")

	ticketListCmd.Flags().String("area", "", "Filter by area slug")
	ticketListCmd.Flags().String("parent", "", "Filter by parent ticket key")
	ticketListCmd.Flags().String("status", "", "Filter by status")
	ticketListCmd.Flags().String("type", "", "Filter by type")
	ticketListCmd.Flags().String("priority", "", "Filter by priority")
	ticketListCmd.Flags().String("search", "", "Search ticket titles")
	ticketListCmd.Flags().Bool("flagged", false, "Filter by flagged state")

	ticketUpdateCmd.Flags().String("title", "", "Ticket title")
	ticketUpdateCmd.Flags().String("description", "", "Ticket description")
	ticketUpdateCmd.Flags().String("area", "", "Area slug; pass an empty value to clear")
	ticketUpdateCmd.Flags().String("type", "", "Ticket type")
	ticketUpdateCmd.Flags().String("status", "", "Ticket status")
	ticketUpdateCmd.Flags().String("reason", "", "Cancellation reason")
	ticketUpdateCmd.Flags().String("priority", "", "Ticket priority")
	ticketUpdateCmd.Flags().Bool("flagged", false, "Ticket flagged state")
	ticketUpdateCmd.Flags().Int64("revision", 0, "Expected ticket revision")

	ticketDeleteCmd.Flags().Bool("yes", false, "Confirm deletion")
	ticketDeleteCmd.MarkFlagRequired("yes")
	ticketNoteAddCmd.Flags().String("actor", "", "Optional actor label")
}

var ticketCmd = &cobra.Command{
	Use:   "ticket",
	Short: "Manage tickets",
}

var ticketNoteCmd = &cobra.Command{
	Use:   "note",
	Short: "Manage append-only ticket notes",
}

var ticketCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := service.TicketCreateInput{ProjectKey: flagString(cmd, "project"), AreaSlug: flagString(cmd, "area"), ParentKey: flagString(cmd, "parent"), Title: args[0], Description: flagString(cmd, "description"), Type: domain.TicketType(flagString(cmd, "type")), Priority: domain.Priority(flagString(cmd, "priority"))}
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		ticket, err := svc.CreateTicket(commandContext(), input)
		if err != nil {
			return err
		}
		return writeResource(cmd, ticket, func() error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Created ticket %s: %s\n", ticket.Key, ticket.Title)
			return err
		})
	},
}

var ticketListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tickets",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		input := service.TicketListInput{ProjectKey: flagString(cmd, "project"), AreaSlug: flagString(cmd, "area"), ParentKey: flagString(cmd, "parent"), Status: flagString(cmd, "status"), Type: flagString(cmd, "type"), Priority: flagString(cmd, "priority"), Search: flagString(cmd, "search")}
		if cmd.Flags().Changed("flagged") {
			flagged, err := cmd.Flags().GetBool("flagged")
			if err != nil {
				return err
			}
			input.Flagged = &flagged
		}
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		tickets, err := svc.ListTickets(commandContext(), input)
		if err != nil {
			return err
		}
		return writeResource(cmd, tickets, func() error {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "KEY\tTITLE\tSTATUS\tPRIORITY\tAREA ID\tPARENT\tFLAGGED\tREVISION")
			for _, ticket := range tickets {
				areaID := ""
				if ticket.AreaID != nil {
					areaID = fmt.Sprint(*ticket.AreaID)
				}
				parent := ""
				if ticket.ParentKey != nil {
					parent = *ticket.ParentKey
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%t\t%d\n", ticket.Key, ticket.Title, ticket.Status, ticket.Priority, areaID, parent, ticket.Flagged, ticket.Revision)
			}
			return w.Flush()
		})
	},
}

var ticketGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		ticket, err := svc.GetTicket(commandContext(), flagString(cmd, "project"), args[0])
		if err != nil {
			return err
		}
		return writeResource(cmd, ticket, func() error {
			parent := "None"
			if ticket.ParentKey != nil {
				parent = *ticket.ParentKey
			}
			reason := ""
			if ticket.CancellationReason != nil {
				reason = "\nCancellation reason: " + *ticket.CancellationReason
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%t\t%d\nParent: %s%s\n", ticket.Key, ticket.Title, ticket.Status, ticket.Priority, ticket.Flagged, ticket.Revision, parent, reason)
			return err
		})
	},
}

var ticketUpdateCmd = &cobra.Command{
	Use:   "update <key>",
	Short: "Update a ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		input := service.TicketUpdateInput{ProjectKey: flagString(cmd, "project"), Key: args[0]}
		if cmd.Flags().Changed("title") {
			value := flagString(cmd, "title")
			input.Title = &value
		}
		if cmd.Flags().Changed("description") {
			value := flagString(cmd, "description")
			input.Description = &value
		}
		if cmd.Flags().Changed("area") {
			value := flagString(cmd, "area")
			input.AreaSlug = &value
		}
		if cmd.Flags().Changed("type") {
			value := domain.TicketType(flagString(cmd, "type"))
			input.Type = &value
		}
		if cmd.Flags().Changed("status") {
			value := domain.Status(flagString(cmd, "status"))
			input.Status = &value
		}
		if cmd.Flags().Changed("reason") {
			value := flagString(cmd, "reason")
			input.Reason = &value
		}
		if cmd.Flags().Changed("priority") {
			value := domain.Priority(flagString(cmd, "priority"))
			input.Priority = &value
		}
		if cmd.Flags().Changed("flagged") {
			value, err := cmd.Flags().GetBool("flagged")
			if err != nil {
				return err
			}
			input.Flagged = &value
		}
		if cmd.Flags().Changed("revision") {
			value, err := cmd.Flags().GetInt64("revision")
			if err != nil {
				return err
			}
			input.Revision = &value
		}
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		ticket, err := svc.UpdateTicket(commandContext(), input)
		if err != nil {
			return err
		}
		return writeResource(cmd, ticket, func() error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Updated ticket %s: %s\n", ticket.Key, ticket.Title)
			return err
		})
	},
}

var ticketNoteAddCmd = &cobra.Command{
	Use:   "add <key> <body>",
	Short: "Append a note to a ticket",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var actor *string
		if cmd.Flags().Changed("actor") {
			value := flagString(cmd, "actor")
			actor = &value
		}
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		note, err := svc.AddTicketNote(commandContext(), flagString(cmd, "project"), args[0], args[1], actor)
		if err != nil {
			return err
		}
		return writeResource(cmd, note, func() error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Added note %d to %s\n", note.ID, args[0])
			return err
		})
	},
}

var ticketNoteListCmd = &cobra.Command{
	Use:   "list <key>",
	Short: "List a ticket's notes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		notes, err := svc.ListTicketNotes(commandContext(), flagString(cmd, "project"), args[0])
		if err != nil {
			return err
		}
		return writeResource(cmd, notes, func() error {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tKIND\tACTOR\tCREATED\tBODY")
			for _, note := range notes {
				actor := ""
				if note.Actor != nil {
					actor = *note.Actor
				}
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", note.ID, note.Kind, actor, note.CreatedAt.Format(time.RFC3339), note.Body)
			}
			return w.Flush()
		})
	},
}

var ticketDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a ticket",
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
		project := flagString(cmd, "project")
		ticket, err := svc.GetTicket(commandContext(), project, args[0])
		if err != nil {
			return err
		}
		if err := svc.DeleteTicket(commandContext(), project, ticket.Key); err != nil {
			return err
		}
		return writeDeleted(cmd, ticket.Key)
	},
}

func flagString(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return value
}
