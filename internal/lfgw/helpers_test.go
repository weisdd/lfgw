package lfgw

import (
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

	userAgentGrafana := "Grafana/8.5.0"
	userAgentCurl := "curl/7.81.0"

	tests := []struct {
		name        string
		userAgent   string
		header      string
		headerValue string
		want        string
		wantErr     error
	}{
		{
			name:        "X-Forwarded-Access-Token",
			userAgent:   userAgentGrafana,
			header:      "X-Forwarded-Access-Token",
			headerValue: "FAKE_TOKEN",
			want:        "FAKE_TOKEN",
			wantErr:     nil,
		},
		{
			name:        "X-Auth-Request-Access-Token",
			userAgent:   userAgentGrafana,
			header:      "X-Auth-Request-Access-Token",
			headerValue: "FAKE_TOKEN",
			want:        "FAKE_TOKEN",
			wantErr:     nil,
		},
		{
			name:        "Authorization Bearer",
			userAgent:   userAgentGrafana,
			header:      "Authorization",
			headerValue: "Bearer FAKE_TOKEN",
			want:        "FAKE_TOKEN",
			wantErr:     nil,
		},
		{
			name:        "No token: Authorization Basic",
			userAgent:   userAgentGrafana,
			header:      "Authorization",
			headerValue: "Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==",
			want:        "",
			wantErr:     errNoTokenGrafana,
		},
		{
			name:        "No token: request from grafana",
			userAgent:   userAgentGrafana,
			header:      "",
			headerValue: "",
			want:        "",
			wantErr:     errNoTokenGrafana,
		},
		{
			name:        "No token: request from curl",
			userAgent:   userAgentCurl,
			header:      "",
			headerValue: "",
			want:        "",
			wantErr:     errNoToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodGet, "/", nil)
			if err != nil {
				t.Fatal(err)
			}

			if tt.header != "" && tt.headerValue != "" {
				r.Header.Set(tt.header, tt.headerValue)
			}

			r.Header.Set("User-Agent", tt.userAgent)

			got, err := app.getRawAccessToken(r)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, got)
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
