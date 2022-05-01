package lfgw

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/weisdd/lfgw/internal/querymodifier"
)

// TODO: logMiddleware add a test https://go.dev/src/net/http/httputil/reverseproxy_test.go
// to make sure such errors don't happen: reverseproxy.go:489 >  error="http: proxy error: net/http: HTTP/1.x transport connection broken: http: ContentLength=57 with Body length 0\n"

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

func Test_rewriteRequestMiddleware(t *testing.T) {
	logger := zerolog.New(nil)

	t.Run("UpstreamURL is not set", func(t *testing.T) {
		app := &application{
			logger:      &logger,
			UpstreamURL: nil,
		}

		r, err := http.NewRequest(http.MethodGet, "http://lfgw/api/v1/federate", nil)
		if err != nil {
			t.Fatal(err)
		}

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("OK"))
		})

		rr := httptest.NewRecorder()
		app.rewriteRequestMiddleware(next).ServeHTTP(rr, r)
		rs := rr.Result()

		got := rs.StatusCode
		want := http.StatusInternalServerError

		// TODO: check logs for the error message?
		assert.Equal(t, want, got)

		defer rs.Body.Close()
	})

	// TODO:rewrite once something is done with app.UpstreamURL
	upstreamURL, err := url.Parse("http://prometheus")
	assert.Nil(t, err)

	app := &application{
		logger:      &logger,
		UpstreamURL: upstreamURL,
	}

	t.Run("ACL is not in the context", func(t *testing.T) {
		r, err := http.NewRequest(http.MethodGet, "http://lfgw/api/v1/federate", nil)
		if err != nil {
			t.Fatal(err)
		}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("OK"))
		})

		rr := httptest.NewRecorder()
		app.rewriteRequestMiddleware(next).ServeHTTP(rr, r)
		rs := rr.Result()

		got := rs.StatusCode
		want := http.StatusInternalServerError

		// TODO: check logs for the error message?
		assert.Equal(t, want, got)

		defer rs.Body.Close()
	})

	t.Run("Not an API request", func(t *testing.T) {
		r, err := http.NewRequest(http.MethodGet, "http://lfgw/fakeapi/v1/query?query=kube_pod_info", nil)
		if err != nil {
			t.Fatal(err)
		}

		acl, err := querymodifier.NewACL("monitoring")
		assert.Nil(t, err)

		ctx := context.WithValue(r.Context(), contextKeyACL, acl)
		r = r.WithContext(ctx)

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, err := url.QueryUnescape(r.URL.RawQuery)
			assert.Nil(t, err)

			want := "query=kube_pod_info"
			assert.Equal(t, want, got)

			_, _ = w.Write([]byte("OK"))
		})

		rr := httptest.NewRecorder()
		app.rewriteRequestMiddleware(next).ServeHTTP(rr, r)
		rs := rr.Result()

		got := rs.StatusCode
		want := http.StatusOK

		// TODO: check logs for the error message?
		assert.Equal(t, want, got)

		defer rs.Body.Close()
	})

	t.Run("User has full access, API request is not modified", func(t *testing.T) {
		r, err := http.NewRequest(http.MethodGet, "http://lfgw/api/v1/query?query=kube_pod_info", nil)
		if err != nil {
			t.Fatal(err)
		}

		acl, err := querymodifier.NewACL(".*")
		assert.Nil(t, err)

		ctx := context.WithValue(r.Context(), contextKeyACL, acl)
		r = r.WithContext(ctx)

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, err := url.QueryUnescape(r.URL.RawQuery)
			assert.Nil(t, err)

			want := "query=kube_pod_info"
			assert.Equal(t, want, got)

			_, _ = w.Write([]byte("OK"))
		})

		rr := httptest.NewRecorder()
		app.rewriteRequestMiddleware(next).ServeHTTP(rr, r)
		rs := rr.Result()

		got := rs.StatusCode
		want := http.StatusOK

		// TODO: check logs for the message?
		assert.Equal(t, want, got)

		defer rs.Body.Close()
	})

	// TODO: merge GET & POST tests?

	t.Run("API request is modified according to an ACL (GET)", func(t *testing.T) {
		r, err := http.NewRequest(http.MethodGet, "http://lfgw/api/v1/query?query=kube_pod_info", nil)
		if err != nil {
			t.Fatal(err)
		}

		acl, err := querymodifier.NewACL("monitoring")
		assert.Nil(t, err)

		ctx := context.WithValue(r.Context(), contextKeyACL, acl)
		r = r.WithContext(ctx)

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Workaround to make r.ParseForm update r.Form and r.PostForm again
			r.Form = nil
			r.PostForm = nil

			err := r.ParseForm()
			assert.Nil(t, err)

			want := url.Values{
				"query": {`kube_pod_info{namespace="monitoring"}`},
			}
			got := r.Form

			assert.Equal(t, want, got)

			_, _ = w.Write([]byte("OK"))
		})

		rr := httptest.NewRecorder()
		app.rewriteRequestMiddleware(next).ServeHTTP(rr, r)
		rs := rr.Result()

		got := rs.StatusCode
		want := http.StatusOK

		// TODO: check logs for the error message?
		assert.Equal(t, want, got)

		defer rs.Body.Close()
	})

	t.Run("API request is modified according to an ACL (POST)", func(t *testing.T) {
		body := io.NopCloser(strings.NewReader("query=kube_pod_info"))

		r, err := http.NewRequest(http.MethodPost, "http://lfgw/api/v1/query", body)
		if err != nil {
			t.Fatal(err)
		}

		// Requests of a different type are not decoded by r.ParseForm()
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		acl, err := querymodifier.NewACL("monitoring")
		assert.Nil(t, err)

		ctx := context.WithValue(r.Context(), contextKeyACL, acl)
		r = r.WithContext(ctx)

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Workaround to make r.ParseForm update r.Form and r.PostForm again
			r.Form = nil
			r.PostForm = nil

			err := r.ParseForm()
			assert.Nil(t, err)

			want := url.Values{
				"query": {`kube_pod_info{namespace="monitoring"}`},
			}
			got := r.PostForm

			assert.Equal(t, want, got)

			postForm := r.PostForm.Encode()
			newBody := strings.NewReader(postForm)
			r.ContentLength = newBody.Size()
			r.Body = io.NopCloser(newBody)

			_, _ = w.Write([]byte("OK"))
		})

		rr := httptest.NewRecorder()
		app.rewriteRequestMiddleware(next).ServeHTTP(rr, r)
		rs := rr.Result()

		got := rs.StatusCode
		want := http.StatusOK

		// TODO: check logs for the error message?
		assert.Equal(t, want, got)

		defer rs.Body.Close()
	})

	// TODO: log fields are added (both get / post)
}
