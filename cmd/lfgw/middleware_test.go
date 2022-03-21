package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

func TestProhibitedMethodsMiddleware(t *testing.T) {
	logger := zerolog.New(nil)
	app := &application{
		logger: &logger,
	}

	tests := []struct {
		name   string
		method string
		want   int
	}{
		{
			name:   "GET",
			method: http.MethodGet,
			want:   http.StatusOK,
		},
		{
			name:   "POST",
			method: http.MethodGet,
			want:   http.StatusOK,
		},
		{
			name:   "PATCH",
			method: http.MethodPatch,
			want:   http.StatusMethodNotAllowed,
		}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			r, err := http.NewRequest(tt.method, "/", nil)
			if err != nil {
				t.Fatal(err)
			}
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("OK"))
			})
			app.prohibitedMethodsMiddleware(next).ServeHTTP(rr, r)
			rs := rr.Result()

			if rs.StatusCode != tt.want {
				t.Errorf("want %d; got %d", tt.want, rs.StatusCode)
			}
			defer rs.Body.Close()
		})
	}
}

func TestProhibitedPathsMiddleware(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		safeMode bool
		want     int
	}{
		{
			name:     "safe mode on tsdb",
			path:     "/admin/tsdb",
			safeMode: true,
			want:     http.StatusForbidden,
		},
		{
			name:     "safe mode off tsdb",
			path:     "/admin/tsdb",
			safeMode: false,
			want:     http.StatusOK,
		},
		{
			name:     "safe mode on api write",
			path:     "/api/v1/write",
			safeMode: true,
			want:     http.StatusForbidden,
		},
		{
			name:     "safe mode off api write",
			path:     "/api/v1/write",
			safeMode: false,
			want:     http.StatusOK,
		},
		{
			name:     "safe mode on random path",
			path:     "/api/v1/test",
			safeMode: false,
			want:     http.StatusOK,
		},
		{
			name:     "safe mode off random path",
			path:     "/api/v1/test",
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
			r, err := http.NewRequest(http.MethodGet, tt.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("OK"))
			})
			app.prohibitedPathsMiddleware(next).ServeHTTP(rr, r)
			rs := rr.Result()

			if rs.StatusCode != tt.want {
				t.Errorf("want %d; got %d", tt.want, rs.StatusCode)
			}
			defer rs.Body.Close()
		})
	}
}
