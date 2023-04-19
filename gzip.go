package gateway

import (
	"bytes"
	"io"
	"net/http"

	"pkg.moe/pkg/encryption"
)

func (g *GateWay) httpReqGzip(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("content-encoding") == "gzip" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "can't read body", http.StatusBadRequest)
				return
			}

			content, err := encryption.UnGzip(body)
			if err != nil {
				http.Error(w, "can't read body by gzip", http.StatusBadRequest)
				return
			}

			r.Body = io.NopCloser(bytes.NewBuffer(content))
		}
		h.ServeHTTP(w, r)
	})
}
