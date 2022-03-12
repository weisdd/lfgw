package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// serve starts a web server and ensures graceful shutdown
func (app *application) serve() error {
	// TODO: somehow pass more context to ErrorLog
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
		Msgf("Successfully stopped server")

	return nil
}
