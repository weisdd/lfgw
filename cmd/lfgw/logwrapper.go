package main

import (
	"strings"

	"github.com/rs/zerolog"
)

type stdErrorLogWrapper struct {
	logger *zerolog.Logger
}

// TODO: new?

// Write implements io.Writer interface to redirect standard logger entries to zerolog. Also, it cuts caller from a log entry and passes it to zerolog's caller.
func (s stdErrorLogWrapper) Write(p []byte) (n int, err error) {
	caller, errorMsg, _ := strings.Cut(string(p), " ")
	caller = strings.TrimRight(caller, ":")

	s.logger.Error().
		Str("caller", caller).
		Str("error", errorMsg).
		Msg("")

	return len(p), nil
}
