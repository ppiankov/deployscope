package cli

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/ppiankov/deployscope/internal/k8s"
	"github.com/ppiankov/deployscope/internal/metrics"
	"github.com/ppiankov/deployscope/internal/server"
)

func newServeCmd() *cobra.Command {
	var port string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start HTTP server (REST API + dashboard + metrics)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if port == "" {
				port = os.Getenv("PORT")
			}
			if port == "" {
				port = "8080"
			}

			k8sClient, err := k8s.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create kubernetes client: %w", err)
			}

			corsOrigin := os.Getenv("CORS_ORIGIN")
			srv := server.New(k8sClient, corsOrigin)

			mux := http.NewServeMux()
			srv.RegisterRoutes(mux)
			handler := metrics.Middleware(mux)

			log.Printf("deployscope v%s starting on port %s", version, port)
			return http.ListenAndServe(":"+port, handler)
		},
	}

	cmd.Flags().StringVar(&port, "port", "", "HTTP server port (default: $PORT or 8080)")

	return cmd
}
