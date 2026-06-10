package telemetry

import (
	"fmt"
	"io"
	"os"
)

const TelemetryReference = "github.com/PeterPonyu/emberforge-go/pkg/telemetry"

type Event struct {
	Name    string
	Details string
}

type TelemetrySink interface {
	Record(event Event)
}

// ConsoleTelemetrySink writes human-readable telemetry lines to a configurable
// writer. A zero-value sink (Writer == nil) writes to os.Stdout, preserving the
// original behaviour; one-shot prompt mode configures Writer to os.Stderr so
// stdout carries only the model answer.
type ConsoleTelemetrySink struct {
	Writer io.Writer
}

func (s ConsoleTelemetrySink) Record(event Event) {
	w := s.Writer
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, "[telemetry] %s: %s\n", event.Name, event.Details)
}
