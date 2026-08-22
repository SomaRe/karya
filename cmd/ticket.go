package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/somare/karya/internal/model"
	"github.com/somare/karya/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(ticketCmd)
	ticketCmd.AddCommand(ticketNewCmd)
	ticketCmd.AddCommand(ticketSetCmd)
	ticketCmd.AddCommand(ticketFlagCmd)
	ticketCmd.AddCommand(ticketDeleteCmd)
	ticketCmd.AddCommand(ticketShowCmd)
	ticketCmd.AddCommand(ticketOpenCmd)
	ticketCmd.AddCommand(ticketLsCmd)

	ticketNewCmd.Flags().StringVar(&ticketNewEpic, "epic", "", "Epic name")
	ticketNewCmd.Flags().StringVar(&ticketNewType, "type", "task", "Ticket type (task|bug|spike)")
	ticketNewCmd.Flags().StringVar(&ticketNewPriority, "priority", "medium", "Priority (low|medium|high)")
	ticketNewCmd.Flags().StringVarP(&ticketNewDescription, "description", "d", "", "Ticket description / body")
	ticketNewCmd.MarkFlagRequired("epic")

	ticketLsCmd.Flags().StringVar(&ticketLsEpic, "epic", "", "Filter by epic")
	ticketLsCmd.Flags().StringVar(&ticketLsStatus, "status", "", "Filter by status")
	ticketLsCmd.Flags().StringVar(&ticketLsType, "type", "", "Filter by type (task|bug|spike)")
	ticketLsCmd.Flags().StringVar(&ticketLsGrep, "grep", "", "Filter by text in title (case-insensitive)")
	ticketLsCmd.Flags().BoolVar(&ticketLsFlagged, "flagged", false, "Show only flagged tickets")
	ticketLsCmd.Flags().BoolVar(&ticketLsJSON, "json", false, "Output as JSON")
}

var ticketCmd = &cobra.Command{
	Use:   "ticket",
	Short: "Manage tickets",
}

// --- ticket new ---

var ticketNewEpic, ticketNewType, ticketNewPriority, ticketNewDescription string

var ticketNewCmd = &cobra.Command{
	Use:   "new <title>",
	Short: "Create a new ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := activeProjectDir()
		if err != nil {
			return err
		}
		p, err := store.ReadProjectConfig(projectDir)
		if err != nil {
			return err
		}

		epicDir := filepath.Join(projectDir, ticketNewEpic)
		if _, err := os.Stat(epicDir); err != nil {
			return fmt.Errorf("epic %q not found — run: karya epic new %q", ticketNewEpic, ticketNewEpic)
		}

		id, err := store.NextID(projectDir, p.Prefix)
		if err != nil {
			return err
		}

		t := &model.Ticket{
			ID:       id,
			Title:    args[0],
			Type:     model.TicketType(ticketNewType),
			Status:   model.StatusBacklog,
			Priority: model.Priority(ticketNewPriority),
			Epic:     ticketNewEpic,
			Flagged:  false,
			Body:     ticketNewDescription,
			Dir:      filepath.Join(epicDir, id),
		}
		if err := store.WriteTicket(t); err != nil {
			return err
		}
		fmt.Printf("Created ticket %s: %s\n", t.ID, t.Title)
		return nil
	},
}

// --- ticket set ---

var ticketSetCmd = &cobra.Command{
	Use:   "set <id> <field> <value>",
	Short: "Update a ticket field (status, priority, type)",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := activeProjectDir()
		if err != nil {
			return err
		}
		t, err := store.FindTicket(projectDir, args[0])
		if err != nil {
			return err
		}
		field, value := args[1], args[2]
		switch field {
		case "status":
			t.Status = model.Status(value)
		case "priority":
			t.Priority = model.Priority(value)
		case "type":
			t.Type = model.TicketType(value)
		case "description":
			t.Body = value
		default:
			return fmt.Errorf("unknown field %q — supported: status, priority, type, description", field)
		}
		if err := store.WriteTicket(t); err != nil {
			return err
		}
		fmt.Printf("Updated %s: %s = %s\n", t.ID, field, value)
		return nil
	},
}

// --- ticket flag ---

var ticketFlagCmd = &cobra.Command{
	Use:   "flag <id>",
	Short: "Toggle the flagged field on a ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := activeProjectDir()
		if err != nil {
			return err
		}
		t, err := store.FindTicket(projectDir, args[0])
		if err != nil {
			return err
		}
		t.Flagged = !t.Flagged
		if err := store.WriteTicket(t); err != nil {
			return err
		}
		state := "flagged"
		if !t.Flagged {
			state = "unflagged"
		}
		fmt.Printf("%s is now %s\n", t.ID, state)
		return nil
	},
}

// --- ticket show ---

var ticketShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Print a ticket to stdout",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := activeProjectDir()
		if err != nil {
			return err
		}
		t, err := store.FindTicket(projectDir, args[0])
		if err != nil {
			return err
		}
		flagged := ""
		if t.Flagged {
			flagged = " [FLAGGED]"
		}
		fmt.Printf("%-10s %s%s\n", t.ID, t.Title, flagged)
		fmt.Printf("  Status:   %s\n", t.Status)
		fmt.Printf("  Type:     %s\n", t.Type)
		fmt.Printf("  Priority: %s\n", t.Priority)
		fmt.Printf("  Epic:     %s\n", t.Epic)
		fmt.Printf("  Created:  %s\n", t.Created.Format("2006-01-02 15:04"))
		fmt.Printf("  Modified: %s\n", t.Modified.Format("2006-01-02 15:04"))
		if t.Body != "" {
			fmt.Printf("\n%s\n", t.Body)
		}
		return nil
	},
}

// --- ticket open ---

var ticketOpenCmd = &cobra.Command{
	Use:   "open <id>",
	Short: "Open ticket.md in $EDITOR",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := activeProjectDir()
		if err != nil {
			return err
		}
		t, err := store.FindTicket(projectDir, args[0])
		if err != nil {
			return err
		}
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nano"
		}
		c := exec.Command(editor, filepath.Join(t.Dir, "ticket.md"))
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

// --- ticket ls ---

var ticketLsEpic, ticketLsStatus, ticketLsType, ticketLsGrep string
var ticketLsFlagged, ticketLsJSON bool

var ticketLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List tickets",
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := activeProjectDir()
		if err != nil {
			return err
		}
		tickets, err := store.ListTickets(projectDir)
		if err != nil {
			return err
		}

		// apply filters
		filtered := tickets[:0]
		for _, t := range tickets {
			if ticketLsEpic != "" && t.Epic != ticketLsEpic {
				continue
			}
			if ticketLsStatus != "" && string(t.Status) != ticketLsStatus {
				continue
			}
			if ticketLsFlagged && !t.Flagged {
				continue
			}
			if ticketLsType != "" && string(t.Type) != ticketLsType {
				continue
			}
			if ticketLsGrep != "" && !strings.Contains(strings.ToLower(t.Title), strings.ToLower(ticketLsGrep)) {
				continue
			}
			filtered = append(filtered, t)
		}
		store.SortTickets(filtered)

		if ticketLsJSON {
			return json.NewEncoder(os.Stdout).Encode(filtered)
		}

		if len(filtered) == 0 {
			fmt.Println("No tickets found.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tPRIORITY\tEPIC\tFLAGGED")
		for _, t := range filtered {
			flagged := ""
			if t.Flagged {
				flagged = "!"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				t.ID, t.Title, t.Status, t.Priority, t.Epic, flagged)
		}
		return w.Flush()
	},
}

// --- ticket delete ---

var ticketDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a ticket (removes its folder and all contents)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectDir, err := activeProjectDir()
		if err != nil {
			return err
		}
		t, err := store.FindTicket(projectDir, args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Delete ticket %s (%s)? [y/N] ", t.ID, t.Title)
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
		if err := os.RemoveAll(t.Dir); err != nil {
			return err
		}
		fmt.Printf("Deleted %s\n", t.ID)
		return nil
	},
}
