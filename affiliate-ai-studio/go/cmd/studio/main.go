package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"ai-learn-playground/affiliate-ai-studio/go/internal/app"
	"ai-learn-playground/affiliate-ai-studio/go/internal/assets"
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
	templatesGlob := filepath.Join(workingDir, "web", "templates", "*.html")
	staticDir := filepath.Join(workingDir, "web", "static")
	distDir := filepath.Join(staticDir, "dist")

	tmpl, err := template.ParseGlob(templatesGlob)
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	workerClient, err := worker.Start(context.Background(), pythonDir)
	if err != nil {
		log.Fatalf("start python worker: %v", err)
	}
	defer workerClient.Close()

	application := app.New("Affiliate AI Studio", workerClient)

	devMode := os.Getenv("AFFILIATE_STUDIO_DEV") == "1"
	devURL := envOr("AFFILIATE_STUDIO_DEV_URL", "http://localhost:5173")

	viteCfg := httpserver.ViteConfig{
		DevMode:      devMode,
		DevServerURL: devURL,
		EntryModule:  "/src/main.tsx",
		Manifest:     assets.NewManifest(distDir, "src/main.tsx"),
	}

	handler := httpserver.NewHandler(application, tmpl).WithVite(viteCfg)
	staticFS := http.FileServer(http.Dir(staticDir))
	mux := httpserver.NewMux(handler, staticFS)

	if devMode {
		log.Printf("Studio DEV mode: Vite client at %s", devURL)
	} else if _, err := viteCfg.Manifest.Resolve(); err != nil {
		log.Printf("WARNING: %v", err)
		log.Printf("Studio page will return 500 until you run `npm run build` in web-studio/ " +
			"or set AFFILIATE_STUDIO_DEV=1 and start `npm run dev`.")
	}

	log.Println("Affiliate AI Studio listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
