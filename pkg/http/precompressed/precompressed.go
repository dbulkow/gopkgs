package precompressed

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var suffixes = map[string]string{
	"gzip": ".gz",
	"br":   ".br",
}

func genEtag(info os.FileInfo) string {
	tval := strconv.FormatInt(info.ModTime().Unix(), 36)
	sval := strconv.FormatInt(info.Size(), 36)
	return fmt.Sprintf("%s%s", tval, sval)
}

func PreCompressed(root string) http.Handler {
	if root == "" {
		root = "."
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename := filepath.Join(root, filepath.Clean("/"+r.URL.Path))

		if strings.HasSuffix(r.URL.Path, "/") && len(r.URL.Path) > 1 {
			filename += string(filepath.Separator)
		}

		info, err := os.Stat(filename)
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "file not found", http.StatusNotFound)
				return
			} else if os.IsPermission(err) {
				http.Error(w, "permission denied", http.StatusForbidden)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if info.IsDir() {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		// check for hidden file? - NotFound

		var file *os.File

		accept := strings.Split(r.Header.Get("Accept-Encoding"), ",")
		for _, enc := range accept {
			suffix, ok := suffixes[enc]
			if !ok {
				continue
			}

			compFilename := filename + suffix

			compInfo, err := os.Stat(compFilename)
			if err != nil {
				continue
			}
			if compInfo.IsDir() {
				continue
			}

			file, err = os.Open(compFilename)
			if err != nil {
				continue
			}
			defer file.Close()

			w.Header().Set("Content-Encoding", enc)
			w.Header().Del("Accept-Ranges")
			w.Header().Add("Vary", "Accept-Encoding")
		}

		if file == nil {
			file, err = os.Open(filename)
			if err != nil {
				if os.IsNotExist(err) {
					http.Error(w, "file not found", http.StatusNotFound)
					return
				} else if os.IsPermission(err) {
					http.Error(w, "permissions denied", http.StatusForbidden)
					return
				}
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			defer file.Close()
		}

		w.Header().Set("ETag", genEtag(info))

		// keep Go stdlib from trying to determine content type
		w.Header()["Content-Type"] = nil

		mimetype := mime.TypeByExtension(filepath.Ext(filename))
		if mimetype != "" {
			w.Header().Set("Content-Type", mimetype)
		}

		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	})
}
