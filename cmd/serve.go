package cmd

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/somare/karya/internal/api"
	"github.com/spf13/cobra"
)

//go:embed web
var webFS embed.FS

var servePort int

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVar(&servePort, "port", 8787, "Port to listen on")
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Karya web UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		if servePort < 1 || servePort > 65535 {
			return fmt.Errorf("port must be between 1 and 65535")
		}
		staticFS, err := fs.Sub(webFS, "web")
		if err != nil {
			return err
		}
		svc, err := serviceFor(cmd)
		if err != nil {
			return err
		}
		h := api.NewHandler(staticFS, svc)
		addr := fmt.Sprintf("127.0.0.1:%d", servePort)
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Karya serving on http://%s\n", addr); err != nil {
			return err
		}
		return http.ListenAndServe(addr, h)
	},
}
