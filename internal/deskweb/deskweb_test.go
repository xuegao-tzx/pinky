package deskweb

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func fmtHTML(w http.ResponseWriter, html string) {
	_, _ = w.Write([]byte(html))
}
