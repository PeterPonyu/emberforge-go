package telemetry

import "fmt"

const TelemetryReference = "github.com/PeterPonyu/emberforge-go/pkg/telemetry"

type Event struct {
	Name    string
	Details string
}

type TelemetrySink interface {
	Record(event Event)
}

type ConsoleTelemetrySink struct{}

func (ConsoleTelemetrySink) Record(event Event) {
	fmt.Printf("[telemetry] %s: %s\n", event.Name, event.Details)
}
