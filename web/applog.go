package web

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/styper/qbit-filematcher-go/config"
)

var (
	defaultOut    io.Writer = os.Stdout
	defaultFormat           = config.LogFormatText
)

// Setup sets the log destination with minimum level INFO and text format.
// Pass nil for os.Stdout.
func Setup(out io.Writer) {
	SetupWith(out, slog.LevelInfo, config.LogFormatText)
}

// SetupWith sets destination, minimum log level, and format ("text" or "json"),
// installing a package slog default.
// Empty format defaults to text. Invalid format falls back to text.
func SetupWith(out io.Writer, minLevel slog.Level, format string) {
	if out == nil {
		out = os.Stdout
	}
	format, err := config.NormalizeLogFormat(format)
	if err != nil {
		format = config.LogFormatText
	}
	defaultOut = out
	defaultFormat = format

	opts := &slog.HandlerOptions{Level: minLevel}
	var h slog.Handler
	switch format {
	case config.LogFormatJSON:
		h = slog.NewJSONHandler(out, opts)
	default:
		h = newLineHandler(out, opts)
	}
	slog.SetDefault(slog.New(h))
}

// SetMinLevel changes the minimum level, keeping the current writer and format.
func SetMinLevel(minLevel slog.Level) {
	SetupWith(defaultOut, minLevel, defaultFormat)
}

// ParseLogLevel converts a config level name to slog.Level.
// Empty or unknown values become LevelInfo.
func ParseLogLevel(level string) slog.Level {
	name, err := config.NormalizeLogLevel(level)
	if err != nil {
		return slog.LevelInfo
	}
	switch name {
	case config.LogLevelDebug:
		return slog.LevelDebug
	case config.LogLevelWarn:
		return slog.LevelWarn
	case config.LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

const loggerAttr = "logger"

// GetLogger returns a slog logger that tags records with logger=<name>
// (rendered as "name: …" by lineHandler).
func GetLogger(name string) *slog.Logger {
	return slog.Default().With(slog.String(loggerAttr, name))
}

// logAt writes a record with an explicit timestamp (e.g. access start time).
// args are alternating key/value pairs as in slog.Logger.Log.
func logAt(l *slog.Logger, level slog.Level, t time.Time, msg string, args ...any) {
	ctx := context.Background()
	h := l.Handler()
	if !h.Enabled(ctx, level) {
		return
	}
	r := slog.NewRecord(t, level, msg, 0)
	r.Add(args...)
	_ = h.Handle(ctx, r)
}

// lineHandler formats records as:
//
//	2026-07-27T04:02:15.123Z [INFO ] scan: This is the message
//
// Extra slog attributes are ignored in the text line (available to a JSON
// handler later). The logger attribute sets the name before the colon.
type lineHandler struct {
	opts  slog.HandlerOptions
	mu    *sync.Mutex
	out   io.Writer
	attrs []slog.Attr
}

func newLineHandler(out io.Writer, opts *slog.HandlerOptions) *lineHandler {
	h := &lineHandler{mu: &sync.Mutex{}, out: out}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}
	return h
}

func (h *lineHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *lineHandler) Handle(_ context.Context, r slog.Record) error {
	name := "app"
	findLogger := func(a slog.Attr) {
		if a.Key == loggerAttr && a.Value.Kind() == slog.KindString {
			name = a.Value.String()
		}
	}
	for _, a := range h.attrs {
		findLogger(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		findLogger(a)
		return true
	})

	line := fmt.Sprintf("%s %s %s: %s\n",
		r.Time.UTC().Format("2006-01-02T15:04:05.000Z"),
		levelBracket(r.Level),
		name,
		r.Message,
	)

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.out, line)
	return err
}

func (h *lineHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.attrs = append(slices.Clip(h.attrs), attrs...)
	return &h2
}

func (h *lineHandler) WithGroup(string) slog.Handler {
	// Groups are unused; logger name is a flat attribute.
	return h
}

func levelBracket(level slog.Level) string {
	return fmt.Sprintf("[%-5s]", level.String())
}
