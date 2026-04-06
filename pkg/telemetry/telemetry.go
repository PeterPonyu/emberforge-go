package telemetry

import "fmt"

const RustTelemetryReference = "/home/zeyufu/Desktop/emberforge/crates/telemetry/src/lib.rs"

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
