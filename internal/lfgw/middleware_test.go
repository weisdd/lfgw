package lfgw

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func Test_safeModeMiddleware(t *testing.T) {
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

			r, err := http.NewRequest(tt.method, tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("OK"))
			})

			rr := httptest.NewRecorder()
			app.safeModeMiddleware(next).ServeHTTP(rr, r)
			rs := rr.Result()
			got := rs.StatusCode

			assert.Equal(t, tt.want, got)

			defer rs.Body.Close()
		})
	}
}

func Test_proxyHeadersMiddleware(t *testing.T) {
	// Just to hold reference values
	headers := map[string]string{
		"X-Forwarded-For":   "1.2.3.4",
		"X-Forwarded-Proto": "http",
		"X-Forwarded-Host":  "lfgw",
	}

	// Set the values that will be used by middleware to set new headers in case app.SetProxyHeaders = true
	r, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s://lfgw", headers["X-Forwarded-Proto"]), nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Host", headers["X-Forwarded-Host"])
	r.RemoteAddr = headers["X-Forwarded-For"]

	t.Run("Proxy headers are set", func(t *testing.T) {
		app := &application{
			SetProxyHeaders: true,
		}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for h, want := range headers {
				got := r.Header.Get(h)
				assert.Equal(t, want, got, fmt.Sprintf("%s is set to a different value", h))
			}
			_, _ = w.Write([]byte("OK"))
		})

		rr := httptest.NewRecorder()
		// Better to clone the request to make sure tests don't interfere with each other
		app.proxyHeadersMiddleware(next).ServeHTTP(rr, r.Clone(r.Context()))
		rs := rr.Result()

		got := rs.StatusCode
		want := http.StatusOK

		assert.Equal(t, want, got)

		defer rs.Body.Close()
	})

	t.Run("Proxy headers are NOT set", func(t *testing.T) {
		app := &application{
			SetProxyHeaders: false,
		}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for h := range headers {
				assert.Empty(t, r.Header.Get(h), fmt.Sprintf("%s must be empty", h))
			}
			_, _ = w.Write([]byte("OK"))
		})

		rr := httptest.NewRecorder()
		// Better to clone the request to make sure tests don't interfere with each other
		app.proxyHeadersMiddleware(next).ServeHTTP(rr, r.Clone(r.Context()))
		rs := rr.Result()

		got := rs.StatusCode
		want := http.StatusOK

		assert.Equal(t, want, got)

		defer rs.Body.Close()
	})

}