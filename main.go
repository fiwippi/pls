package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func main() {
	jpegDir := "."
	if len(os.Args) >= 2 {
		jpegDir = os.Args[1]
	}
	servePage(jpegDir)
	serveJpegs(jpegDir)

	slog.Info("Serving images", slog.String("directory", jpegDir))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func servePage(jpegDir string) {
	const tpl = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>Photos</title>
		<style>
			img { max-width: 800px; }
			body { font-family: system-ui, sans-serif; }
		</style>
	</head>
	<body>
		<div style="display: flex; flex-direction: column; row-gap: 10px; margin-left: 10px; margin-top: 10px;">
			{{range .}}<img src="/jpeg/{{ . }}"></img>{{else}}<strong>No photos right now!</strong>{{end}}
		</div>
	</body>
</html>`
	t, err := template.New("page").Parse(tpl)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		images := make([]string, 0)
		entries, err := os.ReadDir(jpegDir)
		if err != nil {
			slog.Error("Failed to list images", slog.Any("err", err))
			return
		}
		for _, e := range entries {
			if !e.IsDir() && isJpeg(e.Name()) {
				images = append(images, e.Name())
			}
		}
		err = t.Execute(w, images)
		if err != nil {
			slog.Error("Failed to render page", slog.Any("err", err))
			return
		}
	})
}

func isJpeg(name string) bool {
	ext := filepath.Ext(strings.ToLower(name))
	return strings.HasSuffix(ext, ".jpg") || strings.HasSuffix(ext, ".jpeg")
}

func serveJpegs(jpegDir string) {
	fsrv := http.FileServer(http.Dir(jpegDir))
	http.Handle("/jpeg/", http.StripPrefix("/jpeg/", fsrv))
}
