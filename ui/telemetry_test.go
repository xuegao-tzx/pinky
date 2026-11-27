package ui

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	mg "github.com/egoist/mygo/ui"

	"github.com/DIYgod/pinky/internal/telemetry"
	"github.com/DIYgod/pinky/ui/fakeapi"
)

// reports is a telemetry backend that keeps what it is sent.
type reports struct {
	mu      sync.Mutex
	usage   []telemetry.Usage
	crashes []telemetry.CrashReport
}

func (r *reports) Usage(_ context.Context, u telemetry.Usage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.usage = append(r.usage, u)
	return nil
}

func (r *reports) Crash(_ context.Context, c telemetry.CrashReport) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.crashes = append(r.crashes, c)
	return nil
}

type telApp struct {
	*testApp
	tel     *telemetry.Client
	dir     string
	mu      sync.Mutex
	changes []Settings
}

// newTelApp is the client on the fake with a telemetry client that sends
// to b (nil: a build with nowhere to send).
func newTelApp(t *testing.T, b telemetry.Backend) *telApp {
	t.Helper()
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv(telemetry.EndpointEnv, "")
	dir := t.TempDir()
	cr := telemetry.CaptureCrashes(telemetry.CrashOptions{Dir: dir, Process: "app"})
	t.Cleanup(cr.Stop)
	opt := telemetry.Options{Dir: dir, Version: "1.2.3", Backend: b, Crashes: cr, Usage: true}
	if b == nil {
		opt.Version = telemetry.DevVersion
	}
	tel := telemetry.Open(opt)
	ta := &telApp{testApp: newTestApp(t, fakeapi.OnePhone), tel: tel, dir: dir}
	ta.app.opt.Telemetry = tel
	ta.app.settings.UsageStats = true
	ta.app.opt.OnSettings = func(_, s Settings) {
		ta.mu.Lock()
		ta.changes = append(ta.changes, s)
		ta.mu.Unlock()
	}
	ta.sync()
	return ta
}

func (ta *telApp) lastChange() (Settings, bool) {
	ta.mu.Lock()
	defer ta.mu.Unlock()
	if len(ta.changes) == 0 {
		return Settings{}, false
	}
	return ta.changes[len(ta.changes)-1], true
}

// eventually syncs until cond holds.
func (ta *telApp) eventually(t *testing.T, tt *mg.Tester, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 250; i++ {
		ta.sync()
		if tt != nil {
			tt.Frame()
		}
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestNoNoteAboutUsageStatistics(t *testing.T) {
	ta := newTelApp(t, &reports{})
	tt := mg.NewTester(ta.view, 1280, 820)
	if hasText(tt, "usage statistics") || !ta.settings.UsageStats || !ta.tel.Usage() || !DefaultSettings().UsageStats {
		t.Fatalf("a note, or statistics off by default: %q", tt.Texts())
	}
}

func TestPrivacySettings(t *testing.T) {
	ta := newTelApp(t, &reports{})
	ta.tel.Count("runs.started")
	ta.page, ta.set.sec = pageSettings, secPrivacy
	tt := mg.NewTester(ta.view, 1280, 820)
	for _, gone := range []string{"Share anonymous usage statistics and crash reports", "What's shared", "Send feedback…"} {
		if hasText(tt, gone) {
			t.Errorf("%q is still in Settings: %q", gone, tt.Texts())
		}
	}
}

func TestFeedbackOpensAGitHubIssue(t *testing.T) {
	ta := newTelApp(t, &reports{})
	ta.page, ta.set.sec = pageSettings, secAdvanced
	tt := mg.NewTester(ta.view, 1280, 820)
	if hasText(tt, "Send feedback…") {
		t.Fatal("Send feedback is still in Settings")
	}
}

func TestFeedbackWithoutDiagnostics(t *testing.T) {
	ta := newTelApp(t, nil)
	tt := mg.NewTester(ta.view, 1280, 820)
	if hasText(tt, "Send feedback…") {
		t.Fatal("Send feedback is still shown")
	}
}

func TestFeedbackWhenTheBrowserFails(t *testing.T) {
	ta := newTelApp(t, &reports{})
	tt := mg.NewTester(ta.view, 1280, 820)
	if hasText(tt, "Send feedback…") {
		t.Fatal("Send feedback is still shown")
	}
}

func TestMenusOfferFeedback(t *testing.T) {
	ta := newTelApp(t, &reports{})
	if help := ta.helpMenu(); help != nil {
		t.Fatalf("help menu still offers feedback: %+v", help)
	}
	plain := newTestApp(t, fakeapi.OnePhone)
	if plain.helpMenu() != nil {
		t.Fatal("a Help menu without telemetry")
	}
	tt := mg.NewTester(plain.view, 1280, 820)
	plain.page = pageSettings
	tt.Frame()
	if hasText(tt, "Send feedback…") {
		t.Fatal("Send feedback without telemetry")
	}
}

// TestTelemetryScreenshots renders Send Feedback, What Pinky shares and
// Settings › Privacy in light and dark into $PINKY_SCREENSHOTS.
func TestTelemetryScreenshots(t *testing.T) {
	dir := os.Getenv("PINKY_SCREENSHOTS")
	if dir == "" {
		t.Skip("set PINKY_SCREENSHOTS to a folder to write the screenshots")
	}
	shoot := func(ta *telApp, name string, setup func()) {
		t.Helper()
		for _, dark := range []bool{false, true} {
			tt := mg.NewTester(ta.view, 1280, 820)
			tt.SetScale(2)
			tt.SetDark(dark)
			if setup != nil {
				setup()
			}
			ta.eventually(t, tt, "posted work", func() bool { return !ta.t.fb.reading })
			tt.Frame()
			suffix := "-light.png"
			if dark {
				suffix = "-dark.png"
			}
			f, err := os.Create(filepath.Join(dir, name+suffix))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
	ta := newTelApp(t, &reports{})
	ta.tel.Count("runs.started")
	ta.tel.Count("tools.phone_tap")
	shoot(ta, "feedback", func() {
		if !ta.t.fb.open {
			ta.openFeedback()
			ta.t.fb.title = "Mirror froze"
			ta.t.fb.message = "The mirror froze after I unlocked the phone from the computer."
			ta.t.fb.review = true
		}
	})
	ta.t.fb.open = false
	ta.page, ta.set.sec = pageSettings, secPrivacy
	shoot(ta, "settings-privacy", nil)
	shoot(ta, "whats-shared", func() { ta.openShared() })
}
