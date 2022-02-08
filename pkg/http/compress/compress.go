// If the client requests compression, try to comply, as long as the
// compression makes the response smaller.
package compress

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

func Compress(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, r)
		resp := recorder.Result()
		body, _ := io.ReadAll(resp.Body)

		// Check if client supports compression. Replaces body in
		// response and sets content-length header if compression
		// performed.
		func() {
			accept := strings.Split(r.Header.Get("Accept-Encoding"), ",")

			var compressed bytes.Buffer
			var zw io.WriteCloser
			var encoding string
			var err error

			// pick the first available compression accepted by client
			for _, enc := range accept {
				switch enc {
				case "gzip":
					zw = gzip.NewWriter(&compressed)
					encoding = "gzip"

				case "deflate":
					zw, err = flate.NewWriter(&compressed, flate.DefaultCompression)
					if err != nil {
						logger.Println("deflate error", err)
						return
					}
					encoding = "deflate"
				}

				if zw != nil {
					_, err := zw.Write(body)
					if err != nil {
						logger.Println("compression error", err)
						return
					}

					err = zw.Close()
					if err != nil {
						logger.Println("compression error", err)
						return
					}

					// make sure compression actually makes output smaller
					if compressed.Len() < len(body) {
						body = compressed.Bytes()
						w.Header().Add("Content-Encoding", encoding)
					}

					return
				}
			}
		}()

		for k, v := range resp.Header {
			w.Header()[k] = v
		}

		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Write(body)
	})
}
