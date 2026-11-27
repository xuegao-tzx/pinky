package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportEnableReadRun(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "webview-cdp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: WebView CDP\ndescription: Drive a page over CDP.\n---\n\nOpen the simulator, then lock the bike.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ping.sh"), []byte("#!/bin/sh\necho pong\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	lib, err := Open(filepath.Join(t.TempDir(), "skills.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.Import(root); err != nil {
		t.Fatal(err)
	}
	list := lib.List()
	if len(list) != 1 || list[0].ID != "webview-cdp" || !list[0].Enabled || !strings.Contains(list[0].Description, "CDP") {
		t.Fatalf("%+v", list)
	}
	if !strings.Contains(lib.Prompt(), "webview-cdp") {
		t.Fatal(lib.Prompt())
	}
	text, files, err := lib.Read("webview-cdp")
	if err != nil || !strings.Contains(text, "lock the bike") || len(files) < 2 {
		t.Fatalf("%s %v %v", text, files, err)
	}
	if err := lib.SetEnabled("webview-cdp", false); err != nil {
		t.Fatal(err)
	}
	if lib.Prompt() != "" {
		t.Fatal("a disabled skill is still in the prompt")
	}
	if err := lib.SetEnabled("webview-cdp", true); err != nil {
		t.Fatal(err)
	}
	res, err := lib.Run(context.Background(), "webview-cdp", "sh ping.sh")
	if err != nil || res.Exit != 0 || !strings.Contains(res.Stdout, "pong") {
		t.Fatalf("%+v %v", res, err)
	}
	again, err := Open(lib.path)
	if err != nil || len(again.List()) != 1 {
		t.Fatalf("%v %+v", err, again)
	}
}
