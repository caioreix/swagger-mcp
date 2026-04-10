package app

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log/slog"
)

func serveStdio(handler jsonHandler, logger *slog.Logger, stdin io.Reader, stdout io.Writer) int { //nolint:gocognit
	if logger == nil {
		logger = slog.Default()
	}

	reader := bufio.NewReader(stdin)
	writer := bufio.NewWriter(stdout)
	defer writer.Flush()

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			componentLogger(logger, "app.stdio").Error("failed to read stdin", "error", err)
			return 1
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			if errors.Is(err, io.EOF) {
				componentLogger(logger, "app.stdio").Info("stdin closed, shutting down")
				return 0
			}
			continue
		}

		response, handleErr := handler.HandleJSON(line)
		if handleErr != nil {
			componentLogger(logger, "app.stdio").Error("failed to handle MCP message", "error", handleErr)
			if errors.Is(err, io.EOF) {
				componentLogger(logger, "app.stdio").Info("stdin closed after handler failure")
				return 0
			}
			continue
		}

		if len(response) > 0 {
			if _, writeErr := writer.Write(append(response, '\n')); writeErr != nil {
				componentLogger(logger, "app.stdio").Error("failed to write stdout", "error", writeErr)
				return 1
			}
			if flushErr := writer.Flush(); flushErr != nil {
				componentLogger(logger, "app.stdio").Error("failed to flush stdout", "error", flushErr)
				return 1
			}
		}

		if errors.Is(err, io.EOF) {
			componentLogger(logger, "app.stdio").Info("processed final MCP message before shutdown")
			return 0
		}
	}
}
