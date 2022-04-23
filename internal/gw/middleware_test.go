package gw

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestSafeModeMiddleware(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		method   string
		safeMode bool
		want     int
	}{
		{
			name:     "tsdb (safe mode on)",
			path:     "/admin/tsdb",
			method:   http.MethodGet,
			safeMode: true,
			want:     http.StatusForbidden,
		},
		{
			name:     "tsdb (safe mode off)",
			path:     "/admin/tsdb",
			method:   http.MethodGet,
			safeMode: false,
			want:     http.StatusOK,
		},
		{
			name:     "api write (safe mode on)",
			path:     "/api/v1/write",
			method:   http.MethodGet,
			safeMode: true,
			want:     http.StatusForbidden,
		},
		{
			name:     "api write (safe mode off)",
			path:     "/api/v1/write",
			method:   http.MethodGet,
			safeMode: false,
			want:     http.StatusOK,
		},
		{
			name:     "random path (safe mode on)",
			path:     "/api/v1/test",
			method:   http.MethodGet,
			safeMode: true,
			want:     http.StatusOK,
		},
		{
			name:     "random path (safe mode off)",
			path:     "/api/v1/test",
			method:   http.MethodGet,
			safeMode: false,
			want:     http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.New(nil)
			app := &application{
				logger:   &logger,
				SafeMode: tt.safeMode,
			}

			rr := httptest.NewRecorder()
			r, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("OK"))
			})
			app.safeModeMiddleware(next).ServeHTTP(rr, r)
			rs := rr.Result()
			got := rs.StatusCode

			assert.Equal(t, tt.want, got)

			defer rs.Body.Close()
		})
	}
}
