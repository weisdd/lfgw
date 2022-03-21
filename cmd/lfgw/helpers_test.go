package main

import (
	"testing"

	"github.com/rs/zerolog"
)

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
