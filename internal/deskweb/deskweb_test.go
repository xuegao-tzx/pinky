package deskweb

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestBrowserTools(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmtHTML(w, `<html><head><title>Simulator</title></head><body>
			<a id="lock" href="/lock">Lock bike</a>
			<a href="/unlock">Unlock bike</a>
			<form><input name="code" placeholder="Code" value=""></form>
			<p>Vehicle 42 is unlocked.</p>
		</body></html>`)
	})
	mux.HandleFunc("/lock", func(w http.ResponseWriter, r *http.Request) {
		fmtHTML(w, `<html><head><title>Locked</title></head><body><p>Bike locked.</p></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	page, err := s.Navigate(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Simulator" || !strings.Contains(page.Text, "Vehicle 42") {
		t.Fatalf("%+v", page)
	}
	if _, title := s.State(); title != "Simulator" {
		t.Fatal(title)
	}
	page, err = s.Click(context.Background(), "#lock", "")
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Locked" || !strings.Contains(page.Text, "Bike locked") {
		t.Fatalf("click: %+v", page)
	}
	if _, err := s.Back(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, title := s.State(); title != "Simulator" {
		t.Fatal(title)
	}
	if _, err := s.Type(context.Background(), "", "Code", "1234"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Eval(context.Background(), `document.querySelector("input[name=code]").value`)
	if err != nil || got != "1234" {
		t.Fatalf("eval %q %v", got, err)
	}
	title, err := s.Eval(context.Background(), "document.title")
	if err != nil || title != "Simulator" {
		t.Fatalf("title %q %v", title, err)
	}
	path, err := s.Screenshot(context.Background())
	if err != nil || path == "" {
		t.Fatal(err)
	}
	if s.LastScreenshot() != path {
		t.Fatal(s.LastScreenshot())
	}
	if err := s.SaveBookmark("sim", ""); err != nil {
		t.Fatal(err)
	}
	if len(s.Bookmarks()) != 1 || s.Bookmarks()[0].URL == "" {
		t.Fatalf("%+v", s.Bookmarks())
	}
	if err := s.Scroll(context.Background(), 200); err != nil {
		t.Fatal(err)
	}
}

func TestEmbeddedEngineAndHeadless(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.MarkDesktop()
	if _, err := s.Navigate(context.Background(), "https://example.com"); err == nil {
		t.Fatal("the desktop app fetched a page before its web view was ready")
	}
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 1}
	eng := &fakeEngine{page: Page{URL: "https://example.com/sim", Title: "Sim", Text: "ready"}, png: png}
	s.UseEngine(eng)
	if s.Mode() != "embedded" {
		t.Fatal(s.Mode())
	}
	page, err := s.NavigateTab(context.Background(), "tab2", "https://example.com/sim")
	if err != nil || page.Title != "Sim" || eng.navID != "tab2" {
		t.Fatalf("%v %+v id=%s", err, page, eng.navID)
	}
	if u, title := s.State(); u != "https://example.com/sim" || title != "Sim" {
		t.Fatalf("state %s %s", u, title)
	}
	path, err := s.ScreenshotTab(context.Background(), "tab2")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}) || !strings.HasSuffix(path, ".png") {
		t.Fatalf("screenshot %s %v", path, err)
	}
	s.UseEngine(nil)
	s.MarkHeadless()
	if _, err := s.Navigate(context.Background(), "https://example.com"); err == nil || !strings.Contains(err.Error(), "open Pinky") {
		t.Fatal(err)
	}
}

type fakeEngine struct {
	navID string
	page  Page
	png   []byte
}

func (f *fakeEngine) Tabs(context.Context) ([]Tab, error) {
	return []Tab{{ID: "tab2", URL: f.page.URL, Title: f.page.Title, Active: true}}, nil
}
func (f *fakeEngine) NewTab(context.Context, string) (Tab, error) {
	return Tab{ID: "tab3", Active: true}, nil
}
func (f *fakeEngine) CloseTab(context.Context, string) error  { return nil }
func (f *fakeEngine) SwitchTab(context.Context, string) error { return nil }
func (f *fakeEngine) Navigate(_ context.Context, id, url string) (Page, error) {
	f.navID = id
	f.page.URL = url
	return f.page, nil
}
func (f *fakeEngine) Back(context.Context, string) (Page, error)    { return f.page, nil }
func (f *fakeEngine) Forward(context.Context, string) (Page, error) { return f.page, nil }
func (f *fakeEngine) Reload(context.Context, string) (Page, error)  { return f.page, nil }
func (f *fakeEngine) Content(context.Context, string) (Page, error) { return f.page, nil }
func (f *fakeEngine) Click(context.Context, string, string, string) (Page, error) {
	return f.page, nil
}
func (f *fakeEngine) Type(context.Context, string, string, string, string) (Page, error) {
	return f.page, nil
}
func (f *fakeEngine) Scroll(context.Context, string, int) error { return nil }
func (f *fakeEngine) Eval(context.Context, string, string) (string, error) {
	return "ok", nil
}
func (f *fakeEngine) Screenshot(context.Context, string) ([]byte, error) { return f.png, nil }

func fmtHTML(w http.ResponseWriter, html string) {
	_, _ = w.Write([]byte(html))
}
