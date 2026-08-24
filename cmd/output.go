package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func writeResource(cmd *cobra.Command, value any, human func() error) error {
	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(value)
	}
	return human()
}

func writeDeleted(cmd *cobra.Command, identifier string) error {
	if jsonOutput {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
			Deleted string `json:"deleted"`
		}{Deleted: identifier})
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", identifier)
	return err
}
