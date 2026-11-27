package deskweb

import (
	"context"
	"os"
	"path/filepath"
	"time"
)

// Tab is one page in the embedded browser.
type Tab struct {
	ID     string
	URL    string
	Title  string
	Active bool
}

// Engine is the browser embedded in Pinky's window. On macOS it is a
// WKWebView, on Windows WebView2 and on Linux WebKit. Cookies live in the
// web engine's persistent store, so a login survives quitting Pinky.
type Engine interface {
	Tabs(ctx context.Context) ([]Tab, error)
	NewTab(ctx context.Context, url string) (Tab, error)
	CloseTab(ctx context.Context, id string) error
	SwitchTab(ctx context.Context, id string) error
	Navigate(ctx context.Context, id, url string) (Page, error)
	Back(ctx context.Context, id string) (Page, error)
	Forward(ctx context.Context, id string) (Page, error)
	Reload(ctx context.Context, id string) (Page, error)
	Content(ctx context.Context, id string) (Page, error)
	Click(ctx context.Context, id, selector, text string) (Page, error)
	Type(ctx context.Context, id, selector, text, value string) (Page, error)
	Scroll(ctx context.Context, id string, dy int) error
	Eval(ctx context.Context, id, expr string) (string, error)
	// Screenshot is a PNG of the rendered page.
	Screenshot(ctx context.Context, id string) ([]byte, error)
}

// Tabs lists the embedded browser's tabs. Without an embedded browser the
// fetch session reports its one page.
func (s *Session) Tabs(ctx context.Context) ([]Tab, error) {
	if e := s.engineNow(); e != nil {
		return e.Tabs(ctx)
	}
	if err := s.noWindow(); err != nil {
		return nil, err
	}
	u, title := s.State()
	return []Tab{{ID: "page", URL: u, Title: title, Active: true}}, nil
}

// NewTab opens a tab. url may be empty.
func (s *Session) NewTab(ctx context.Context, url string) (Tab, error) {
	if e := s.engineNow(); e != nil {
		return e.NewTab(ctx, url)
	}
	if err := s.noWindow(); err != nil {
		return Tab{}, err
	}
	if url != "" {
		if _, err := s.Navigate(ctx, url); err != nil {
			return Tab{}, err
		}
	}
	u, title := s.State()
	return Tab{ID: "page", URL: u, Title: title, Active: true}, nil
}

// SwitchTab selects a tab.
func (s *Session) SwitchTab(ctx context.Context, id string) error {
	if e := s.engineNow(); e != nil {
		return e.SwitchTab(ctx, id)
	}
	if err := s.noWindow(); err != nil {
		return err
	}
	return nil
}

// CloseTab closes a tab.
func (s *Session) CloseTab(ctx context.Context, id string) error {
	if e := s.engineNow(); e != nil {
		return e.CloseTab(ctx, id)
	}
	if err := s.noWindow(); err != nil {
		return err
	}
	return nil
}

func (s *Session) saveShot(e Engine, ctx context.Context, id string) (string, error) {
	data, err := e.Screenshot(ctx, id)
	if err != nil {
		return "", err
	}
	dir := s.dir
	if dir == "" {
		dir = os.TempDir()
	}
	shots := filepath.Join(dir, "shots")
	if err := os.MkdirAll(shots, 0o755); err != nil {
		return "", err
	}
	name := filepath.Join(shots, time.Now().Format("20060102-150405.000")+".png")
	if err := os.WriteFile(name, data, 0o600); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.lastShot = name
	s.mu.Unlock()
	return name, nil
}
