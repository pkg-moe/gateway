package gateway

import (
	"encoding/base64"
	"io"
	"net/http"

	"pkg.moe/pkg/encryption"
)

var xorKey []byte

func init() {
	xorKey, _ = base64.StdEncoding.DecodeString("U1lKCAgI")
}

type xorResponseWriter struct {
	http.ResponseWriter
}

func (w xorResponseWriter) Write(b []byte) (int, error) {
	compbody, err := encryption.Gzip(b)
	if err != nil {
		return 0, err
	}

	return w.ResponseWriter.Write(encryption.Xor(compbody, xorKey))
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (g *GateWay) httpXor(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-sec-fetch-last") == "hit" {
			w.Header().Set("x-sec-fetch-last", "hit")

			gz := xorResponseWriter{
				ResponseWriter: w,
			}

			h.ServeHTTP(gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}
