package main

import (
	"context"
	"log/slog"

	"github.com/DIYgod/pinky/internal/api"
	"github.com/DIYgod/pinky/internal/gateway"
	"github.com/DIYgod/pinky/internal/telemetry"
	"github.com/DIYgod/pinky/ui"
)

// Usage statistics, crash reports and feedback (docs/TELEMETRY.md). The app
// and the daemon capture their crashes from the start; whichever holds the
// gateway counts usage and sends it with the crash reports, while
// Settings › Privacy allows. Scripted fake phones count nothing.

// captureCrashes starts capturing this process's crashes in the data
// folder. Call Stop on a clean exit.
func captureCrashes(api.Dirs, string, *slog.Logger) *telemetry.Crashes {
	return nil
}

// startTelemetry opens usage statistics for the process that holds the
// gateway: it counts what agents and the person do and sends what is due
// until ctx ends. Close it when the gateway stops.
func startTelemetry(context.Context, *runningGateway, *telemetry.Crashes, ui.Settings, string, bool, *slog.Logger) *telemetry.Client {
	return nil
}

// clientAPI is the gateway the desktop client uses: one that counts what
// the person does, unless the phones are fake.
func clientAPI(rg *runningGateway, _ bool) gateway.API {
	return rg.G
}

// applyTelemetry passes Settings › Privacy's choice to the client.
func applyTelemetry(*telemetry.Client, ui.Settings) {}
