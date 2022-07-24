package lfgw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// serve starts a web server and ensures graceful shutdown
func (app *application) serve() error {
	// Just to make sure our logging calls are always safe
	if app.logger == nil {
		app.configureLogging()
	}

	app.proxy = httputil.NewSingleHostReverseProxy(app.UpstreamURL)
	// TODO: somehow pass more context to ErrorLog (unsafe?)
	app.proxy.ErrorLog = app.errorLog
	app.proxy.FlushInterval = time.Millisecond * 200

	// TODO: somehow pass more context to ErrorLog
	//#nosec G112 -- false positive, may be removed after gosec v2.12.0+ is released
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.Port),
		ErrorLog:     app.errorLog,
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  app.ReadTimeout,
		WriteTimeout: app.WriteTimeout,
	}

	shutdownError := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		app.logger.Info().Caller().
			Msgf("Caught %s signal, waiting for all connections to be closed within %s", s, app.GracefulShutdownTimeout)

		ctx, cancel := context.WithTimeout(context.Background(), app.GracefulShutdownTimeout)
		defer cancel()

		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
		}

		shutdownError <- nil
	}()

	app.logger.Info().Caller().
		Msgf("Starting server on %d", app.Port)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	err = <-shutdownError
	if err != nil {
		return err
	}

	app.logger.Info().Caller().
		Msg("Successfully stopped server")

	return nil
}
