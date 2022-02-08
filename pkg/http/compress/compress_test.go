package compress

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

const testdata = `now is the time for all good men to come to aid of the party.
the quick brown fox jumps over the lazy dog.
Lorem ipsum dolor sit amet, consectetur adipiscing elit,
sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
Ut enim ad minim veniam, quis nostrud exercitation ullamco
laboris nisi ut aliquip ex ea commodo consequat. Duis aute
irure dolor in reprehenderit in voluptate velit esse cillum
dolore eu fugiat nulla pariatur. Excepteur sint occaecat
cupidatat non proident, sunt in culpa qui officia deserunt
mollit anim id est laborum.
`

func TestCompress(t *testing.T) {
	var compressed bytes.Buffer
	var zw io.WriteCloser

	zw = gzip.NewWriter(&compressed)
	zw.Write([]byte(testdata))
	zw.Close()
	gzipdata := compressed.Bytes()

	compressed.Reset()

	zw, _ = flate.NewWriter(&compressed, flate.DefaultCompression)
	zw.Write([]byte(testdata))
	zw.Close()
	flatedata := compressed.Bytes()

	datasrc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(testdata))
	})

	h := Compress(log.Default(), datasrc)

	tests := []struct {
		name   string
		encode string
		data   []byte
	}{
		{
			name: "uncompressed",
			data: []byte(testdata),
		},
		{
			name:   "gzip",
			encode: "gzip",
			data:   gzipdata,
		},
		{
			name:   "deflate",
			encode: "deflate",
			data:   flatedata,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)

			r.Header.Set("Accept-Encoding", tt.encode)

			h.ServeHTTP(w, r)

			result := w.Result()
			body, _ := ioutil.ReadAll(result.Body)

			if !bytes.Equal(tt.data, body) {
				fmt.Println(hex.Dump(body))
				fmt.Println(hex.Dump(tt.data))
				t.Fatalf("data mismatch")
			}

			enc := result.Header.Get("Content-Encoding")
			if enc != tt.encode {
				t.Fatalf(`expected encoding "%s" got "%s"`, tt.encode, enc)
			}

			val := result.Header.Get("Content-Length")
			clen, err := strconv.Atoi(val)
			if err != nil {
				t.Fatal("content length conversion", err)
			}
			if clen != len(tt.data) {
				t.Fatalf(`expected content length "%d" got "%d"`, len(tt.data), clen)
			}
		})
	}
}
