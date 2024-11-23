package main

import (
	"bufio"
	"bytes"
	"image/jpeg"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/disintegration/imageorient"
	"github.com/nfnt/resize"
)

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	jpegDir := "."
	if len(os.Args) >= 2 {
		jpegDir = os.Args[1]
	}
	db, err := badger.Open(badger.DefaultOptions("/tmp/pls"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	handlePage(jpegDir)
	handleOriginal(jpegDir)
	handleThumbnail(jpegDir, db)

	slog.Info("Serving images", slog.String("directory", jpegDir))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handlePage(jpegDir string) {
	const tpl = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>Photos</title>
		<style>
			img { max-width: 700px; }
			body { font-family: system-ui, sans-serif; }
		</style>
	</head>
	<body>
		<div style="display: flex; flex-direction: column; row-gap: 10px; margin-left: 10px; margin-top: 10px;">
			{{range .}}<a href="/original/{{ . }}"><img src="/thumb/{{ . }}"></img></a>{{else}}<strong>No photos right now!</strong>{{end}}
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
			slog.Error("Failed to list images (page)", slog.Any("err", err))
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

func handleOriginal(jpegDir string) {
	fsrv := http.FileServer(http.Dir(jpegDir))
	http.Handle("/original/", http.StripPrefix("/original/", fsrv))
}

func handleThumbnail(jpegDir string, db *badger.DB) {
	http.Handle("/thumb/", http.StripPrefix("/thumb/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := []byte(r.URL.Path)

		var data []byte
		err := db.Update(func(tx *badger.Txn) error {
			// Serve the thumbnail if it exists
			item, err := tx.Get(key)
			if err == nil {
				slog.Debug("Loading thumbnail from cache", slog.String("key", string(key)))
				data, err = item.ValueCopy(nil)
				if err != nil {
					return err
				}
				return nil
			}

			// Otherwise resize the image and store it 
			// with a TTL. We don't 
			slog.Debug("Generating thumbnail", slog.String("key", string(key)))
			f, err := os.Open(jpegDir + "/" + string(key))
			if err != nil {
				return err
			}
			img, _, err := imageorient.Decode(f)
			if err != nil {
				return err
			}



			var b bytes.Buffer
   			w := bufio.NewWriter(&b)
    		if err := jpeg.Encode(w, resize.Resize(800, 0, img, resize.Lanczos3), nil); err != nil {
				return err
			}
			data = b.Bytes()
			e := badger.NewEntry(key, b.Bytes()).WithTTL(7 * 24 * time.Hour)
  			return tx.SetEntry(e)
		  })
		  if err != nil {
			slog.Error("Failed to serve thumbnails", slog.Any("err", err))
			return
		}
		
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(data)
	})))
}
