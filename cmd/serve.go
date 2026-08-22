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
		staticFS, err := fs.Sub(webFS, "web")
		if err != nil {
			return err
		}
		h := api.NewHandler(staticFS)
		addr := fmt.Sprintf(":%d", servePort)
		fmt.Printf("Karya serving on http://localhost%s\n", addr)
		return http.ListenAndServe(addr, h)
	},
}
