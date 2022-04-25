package lfgw

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/rs/zerolog"
)

func TestGetRawAccessToken(t *testing.T) {
	logger := zerolog.New(nil)
	app := &application{
		logger: &logger,
	}

	tests := []struct {
		name   string
		header string
		fail   bool
		want   string
	}{
		{
			name:   "X-Forwarded-Access-Token",
			header: "X-Forwarded-Access-Token",
			fail:   false,
			want:   "FAKE_TOKEN",
		},
		{
			name:   "X-Auth-Request-Access-Token",
			header: "X-Auth-Request-Access-Token",
			fail:   false,
			want:   "FAKE_TOKEN",
		},
		{
			name:   "Authorization",
			header: "Authorization",
			fail:   false,
			want:   "FAKE_TOKEN",
		},
		{
			name:   "Random header",
			header: "Random-Header",
			fail:   true,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatal(err)
			}

			switch tt.header {
			case "Authorization":
				r.Header.Set(tt.header, fmt.Sprintf("Bearer %s", tt.want))
			default:
				r.Header.Set(tt.header, tt.want)
			}

			got, err := app.getRawAccessToken(r)
			if tt.fail {
				if err == nil {
					t.Error("Expected a non-nil error, though got a nil one")
				}
			} else {
				if got != tt.want {
					t.Errorf("want %s; got %s", tt.want, got)
				}
			}
		})
	}
}

func TestIsUnsafePath(t *testing.T) {
	logger := zerolog.New(nil)
	app := &application{
		logger: &logger,
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "tsdb",
			path: "/admin/tsdb/1",
			want: true,
		},
		{
			name: "write",
			path: "/api/v1/write",
			want: true,
		},
		{
			name: "random endpoint",
			path: "/api/v1/random",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := app.isUnsafePath(tt.path)
			if got != tt.want {
				t.Errorf("want %t; got %t", tt.want, got)
			}
		})
	}
}

func TestIsNotAPIRequest(t *testing.T) {
	logger := zerolog.New(nil)
	app := &application{
		logger: &logger,
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "api",
			path: "/api/v1/query",
			want: false,
		},
		{
			name: "federate",
			path: "/federate",
			want: false,
		},
		{
			name: "random endpoint",
			path: "/metrics",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := app.isNotAPIRequest(tt.path)
			if got != tt.want {
				t.Errorf("want %t; got %t", tt.want, got)
			}
		})
	}
}
