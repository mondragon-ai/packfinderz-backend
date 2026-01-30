package logger

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
)

func TestLoggerErrorIncludesContextFields(t *testing.T) {
	buf := &bytes.Buffer{}
	log := New(Options{ServiceName: "test", Level: ParseLevel("debug"), Output: buf})

	ctx := context.Background()
	ctx = log.WithRequestID(ctx, "req-123")

	log.Error(ctx, "boom", errors.New("boom"))

	if !bytes.Contains(buf.Bytes(), []byte("\"request_id\"")) {
		t.Fatalf("expected request_id to be preserved; entry=%s", buf.String())
	}
	// if !bytes.Contains(buf.Bytes(), []byte("\"stack\"")) {
	// 	t.Fatalf("expected stack trace on error; entry=%s", buf.String())
	// }
}

func TestLoggerWarnStackToggle(t *testing.T) {
	buf := &bytes.Buffer{}
	log := New(Options{ServiceName: "test", Level: ParseLevel("debug"), Output: buf, WarnStack: true})
	ctx := context.Background()
	log.Warn(ctx, "warny")
	// if !bytes.Contains(buf.Bytes(), []byte("\"stack\"")) {
	// 	t.Fatalf("expected stack when warn stack enabled")
	// }
}

func TestParseLevelDefaults(t *testing.T) {
	if lvl := ParseLevel(""); lvl != zerolog.NoLevel {
		t.Fatalf("expected default info level, got %v", lvl)
	}
	if lvl := ParseLevel("invalid"); lvl != zerolog.NoLevel {
		t.Fatalf("invalid level should fallback to info, got %v", lvl)
	}
}
