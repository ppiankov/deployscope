package main

import (
	"log"
	"net/http"
	"os"

	"github.com/ppiankov/deployscope/internal/k8s"
	"github.com/ppiankov/deployscope/internal/server"
)

var version = "dev"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	k8sClient, err := k8s.NewClient()
	if err != nil {
		log.Fatalf("failed to create kubernetes client: %v", err)
	}

	corsOrigin := os.Getenv("CORS_ORIGIN")
	srv := server.New(k8sClient, corsOrigin)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	log.Printf("deployscope v%s starting on port %s", version, port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
