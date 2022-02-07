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
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

//go:generate testdata/prep

func TestPrecompressed(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		accept    string
	}{
		{
			name:      "no compression",
			extension: "",
			accept:    "",
		},
		{
			name:      "gzip compression",
			extension: ".gz",
			accept:    "gzip",
		},
		{
			name:      "brotli compression",
			extension: ".br",
			accept:    "br",
		},
	}

	filename := "testdata/page.html"

	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	tval := strconv.FormatInt(info.ModTime().Unix(), 36)
	sval := strconv.FormatInt(info.Size(), 36)
	fileEtag := fmt.Sprintf("%s%s", tval, sval)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := PreCompressed("testdata")

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/page.html", nil)

			if tt.accept != "" {
				r.Header.Set("Accept-Encoding", tt.accept)
			}

			pc.ServeHTTP(w, r)

			result := w.Result()

			body, err := ioutil.ReadAll(result.Body)
			if err != nil {
				t.Fatal(err)
			}

			page, err := ioutil.ReadFile(filename + tt.extension)
			if err != nil {
				t.Fatal(err)
			}

			ctype := result.Header.Get("Content-Type")
			if !strings.Contains(ctype, "text/html") {
				t.Fatalf(`expected Content-Type "text/html" got "%s"`, ctype)
			}

			encoding := result.Header.Get("Content-Encoding")
			if encoding != tt.accept {
				t.Fatalf(`expected Content-Encoding == "%s" got "%s"`, tt.accept, encoding)
			}

			if !bytes.Equal(body, page) {
				t.Fatal("body does not match test data")
			}

			etag := result.Header.Get("ETag")
			if etag != fileEtag {
				t.Fatalf(`expected ETag "%s" got "%s"`, fileEtag, etag)
			}
		})
	}
}
