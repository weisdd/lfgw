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
	msg := string(p)
	msg = strings.TrimSpace(msg)

	var errorMsg string
	var caller string
	// TODO: move logic to callerHook?
	for i := range msg {
		if msg[i] == ' ' {
			// Skip ":"
			caller = msg[:i-1]
			// length should always be fine as we trim spaces, thus there can't be a trailing space
			errorMsg = msg[i+1:]
			break
		}
	}

	s.logger.Error().
		Str("caller", caller).
		Str("error", errorMsg).
		Msgf("")

	return len(p), nil
}
