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
	"strconv"
	"strings"
)

type dataRecorder struct {
	headers     http.Header
	shadow      http.Header
	body        bytes.Buffer
	codeWritten bool
	code        int
}

func newDataRecorder() *dataRecorder {
	dr := &dataRecorder{
		headers: make(map[string][]string),
		shadow:  make(map[string][]string),
	}
	return dr
}

func (dr *dataRecorder) Write(buf []byte) (int, error) {
	if !dr.codeWritten {
		dr.WriteHeader(http.StatusOK)
	}
	return dr.body.Write(buf)
}

func (dr *dataRecorder) WriteHeader(code int) {
	dr.code = code
	dr.codeWritten = true

	// save all values written before WriteHeader as headers
	for k, v := range dr.shadow {
		dr.headers[k] = v
	}
}

func (dr *dataRecorder) Header() http.Header {
	return dr.shadow
}

// generate a Content Type if one not provided
func (dr *dataRecorder) finish() {
	var found bool
	for k := range dr.headers {
		if strings.ToLower(k) == "content-type" {
			found = true
		}
	}

	if !found {
		ctype := http.DetectContentType(dr.body.Bytes()[:512])
		dr.headers.Set("Content-Type", ctype)
	}
}

func Compress(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := newDataRecorder()
		next.ServeHTTP(recorder, r)
		recorder.finish()
		body := recorder.body.Bytes()

		// Check if client supports compression. Replaces body in
		// response and sets content-length header if compression
		// performed.
		compress := func() {
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
		}

		compress()

		for k, v := range recorder.headers {
			if k == "Trailer" {
				continue
			}
			w.Header()[k] = v
		}

		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Write(body)

		// trailer keys are stored in the Headers with the key "Trailer"
		var trailers []string
		for k, v := range recorder.headers {
			if k == "Trailer" {
				trailers = v
			}
		}

		// for each trailer key, send the provided values
		for _, k := range trailers {
			if v, found := recorder.headers[k]; found {
				w.Header()[k] = v
			}
		}
	})
}
