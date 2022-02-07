// MIT License

// Copyright (c) 2022 David Bulkow

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
