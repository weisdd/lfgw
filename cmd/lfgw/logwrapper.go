package main

import (
	"strings"

	"github.com/rs/zerolog"
)

type stdErrorLogWrapper struct {
	logger *zerolog.Logger
}

// TODO: new?

func (s stdErrorLogWrapper) Write(p []byte) (n int, err error) {
	caller, errorMsg, _ := strings.Cut(string(p), " ")
	caller = strings.TrimRight(caller, ":")

	s.logger.Error().
		Str("caller", caller).
		Str("error", errorMsg).
		Msgf("")

	return len(p), nil
}
