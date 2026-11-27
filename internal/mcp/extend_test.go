package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DIYgod/pinky/internal/approutes"
	"github.com/DIYgod/pinky/internal/deskweb"
	"github.com/DIYgod/pinky/internal/skills"
)

func TestOpenRouteAndBrowserTools(t *testing.T) {
	h := newHarness(t)
	cat, err := approutes.Open("")
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.Add(approutes.App{
		Name: "HelloBike", Scheme: "hellobike://hellobike.com/",
		AndroidPackage: "com.jingyao.easybike", IOSBundleID: "com.jingyao.easybike",
		Routes: []approutes.Route{{Name: "ride", Path: "{scheme}ride?bike={bikeId}"}},
	}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Sim</title></head><body><a id="lock" href="/ok">Lock</a><p>ready</p></body></html>`))
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Locked</title></head><body><p>locked</p></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	web, err := deskweb.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	lib, err := skills.Open("")
	if err != nil {
		t.Fatal(err)
	}
	h.srv.SetAutomation(Automation{Skills: lib, Routes: cat, Web: web})
	c := h.connect(t, "claude-code")
	if text, isErr := c.call(t, "phone_connect", map[string]any{"task": "routes"}); isErr {
		t.Fatal(text)
	}
	text, isErr := c.call(t, "open_app_route", map[string]any{
		"device": "Pixel 9", "app": "HelloBike", "route": "ride", "params": []any{"bikeId=42"},
	})
	if isErr || !strings.Contains(text, "hellobike://hellobike.com/ride?bike=42") || !strings.Contains(text, "com.jingyao.easybike") {
		t.Fatalf("route:\n%s", text)
	}
	uri, pkg := h.dev.LastURI()
	if uri != "hellobike://hellobike.com/ride?bike=42" || pkg != "com.jingyao.easybike" {
		t.Fatalf("device got %q %q", uri, pkg)
	}
	text, isErr = c.call(t, "webview_navigate", map[string]any{"url": srv.URL})
	if isErr || !strings.Contains(text, "title: Sim") {
		t.Fatalf("navigate:\n%s", text)
	}
	text, isErr = c.call(t, "webview_click", map[string]any{"selector": "#lock"})
	if isErr || !strings.Contains(text, "title: Locked") {
		t.Fatalf("click:\n%s", text)
	}
	text, isErr = c.call(t, "webview_eval", map[string]any{"expression": "document.title"})
	if isErr || text != "Locked" {
		t.Fatalf("eval: %q err=%v", text, isErr)
	}
	text, isErr = c.call(t, "webview_screenshot", map[string]any{})
	if isErr || !strings.Contains(text, "screenshot: ") {
		t.Fatalf("shot:\n%s", text)
	}
	_ = root
	_ = context.Background()
}
