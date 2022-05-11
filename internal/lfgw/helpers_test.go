package lfgw

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestGetRawAccessToken(t *testing.T) {
	logger := zerolog.New(nil)
	app := &application{
		logger: &logger,
	}

	testsWithToken := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "X-Forwarded-Access-Token",
			header: "X-Forwarded-Access-Token",
			want:   "FAKE_TOKEN",
		},
		{
			name:   "X-Auth-Request-Access-Token",
			header: "X-Auth-Request-Access-Token",
			want:   "FAKE_TOKEN",
		},
		{
			name:   "Authorization",
			header: "Authorization",
			want:   "FAKE_TOKEN",
		},
	}

	for _, tt := range testsWithToken {
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
			assert.Nil(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	testsNoToken := []struct {
		name      string
		userAgent string
		want      error
	}{
		{
			name:      "Request from Grafana",
			userAgent: "Grafana/8.5.0",
			want:      errNoTokenGrafana,
		},
		{
			name:      "Request from another client",
			userAgent: "curl/7.81.0",
			want:      errNoToken,
		},
	}

	for _, tt := range testsNoToken {
		t.Run(tt.name, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatal(err)
			}

			r.Header.Set("User-Agent", tt.userAgent)
			got, err := app.getRawAccessToken(r)
			assert.Empty(t, got)
			assert.ErrorIs(t, err, tt.want)
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
