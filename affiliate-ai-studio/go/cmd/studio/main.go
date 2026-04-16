package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"ai-learn-playground/affiliate-ai-studio/go/internal/app"
	"ai-learn-playground/affiliate-ai-studio/go/internal/httpserver"
	"ai-learn-playground/affiliate-ai-studio/go/internal/worker"
)

func main() {
	workingDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("resolve working directory: %v", err)
	}
	projectRoot := filepath.Dir(workingDir)
	pythonDir := filepath.Join(projectRoot, "python")
	templatePath := filepath.Join(workingDir, "web", "templates", "studio.html")
	staticDir := filepath.Join(workingDir, "web", "static")

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Fatalf("parse template: %v", err)
	}

	workerClient, err := worker.Start(context.Background(), pythonDir)
	if err != nil {
		log.Fatalf("start python worker: %v", err)
	}
	defer workerClient.Close()

	application := app.New("Affiliate AI Studio", workerClient)
	handler := httpserver.NewHandler(application, tmpl)
	staticFS := http.FileServer(http.Dir(staticDir))
	mux := httpserver.NewMux(handler, staticFS)

	log.Println("Affiliate AI Studio listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
