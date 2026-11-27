package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	mg "github.com/egoist/mygo/ui"

	"github.com/DIYgod/pinky/internal/approutes"
	"github.com/DIYgod/pinky/internal/deskweb"
	"github.com/DIYgod/pinky/internal/skills"
	"github.com/DIYgod/pinky/ui/fakeapi"
)

func TestAutomationPage(t *testing.T) {
	ta := newTestApp(t, fakeapi.NoPhones)
	lib, err := skills.Open("")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := writeSkill(dir); err != nil {
		t.Fatal(err)
	}
	if err := lib.Import(dir); err != nil {
		t.Fatal(err)
	}
	cat, err := approutes.Open("")
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.Add(approutes.App{Name: "HelloBike", Scheme: "hellobike://hellobike.com/", AndroidPackage: "com.jingyao.easybike"}); err != nil {
		t.Fatal(err)
	}
	web, err := deskweb.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ta.opt.Automation = &Automation{Skills: lib, Routes: cat, Web: web}
	ta.page, ta.set.sec = pageSettings, secAutomation
	tt := mg.NewTester(ta.view, 1100, 800)
	tt.Frame()
	text := strings.Join(tt.Texts(), "\n")
	for _, want := range []string{"Automation", "WebView CDP", "HelloBike", "Require the agent token"} {
		if !strings.Contains(text, want) {
			t.Errorf("settings missing %q\n%s", want, text)
		}
	}
	ta.page = pageBrowser
	tt = mg.NewTester(ta.view, 1100, 800)
	tt.Frame()
	if text = strings.Join(tt.Texts(), "\n"); !strings.Contains(text, "Browser") {
		t.Fatalf("browser page:\n%s", text)
	}
}

func writeSkill(dir string) error {
	skill := filepath.Join(dir, "webview-cdp")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: WebView CDP\ndescription: Drive a page.\n---\n\nDo the thing.\n"), 0o644)
}
