package httpapi

import (
	"io"
	"log/slog"
	"testing"

	"github.com/itsmangooo/Silicon/backend/internal/config"
)

func TestHandlerRegistersRuntimeAndSecretRoutesWithoutConflict(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if handler := New(config.Config{}, nil, logger).Handler(); handler == nil {
		t.Fatal("handler is nil")
	}
}
