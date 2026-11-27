package deskweb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Bookmark is an address the person saved.
type Bookmark struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type stored struct {
	Bookmarks []Bookmark `json:"bookmarks,omitempty"`
	Cookies   []cookie   `json:"cookies,omitempty"`
}

type cookie struct {
	Host  string `json:"host"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Session is the embedded browser.
type Session struct {
	mu        sync.Mutex
	dir       string
	bookmarks []Bookmark
	hist      []string
	at        int
	page      Page
	http      *httpBrowser
	cdp       *cdpBrowser
	engine    Engine
	headless  bool
	desktop   bool
	engineErr error
	lastShot  string
}

// Open prepares the browser state in dir. It does not start Chrome until
// UseChrome or a navigation with PINKY_WEBVIEW=chrome.
func Open(dir string) (*Session, error) {
	s := &Session{dir: dir, http: newHTTPBrowser(), at: -1}
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if data, err := os.ReadFile(s.statePath()); err == nil {
			var st stored
			if json.Unmarshal(data, &st) == nil {
				s.bookmarks = st.Bookmarks
			}
		}
	}
	if strings.EqualFold(os.Getenv("PINKY_WEBVIEW"), "chrome") {
		_ = s.UseChrome(context.Background())
	}
	return s, nil
}

// Mode is "embedded" while the Pinky window's web view is connected,
// "chrome" for a DevTools window, otherwise "http".
func (s *Session) Mode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.engine != nil:
		return "embedded"
	case s.cdp != nil:
		return "chrome"
	case s.headless || s.desktop:
		return "window"
	}
	return "http"
}

// UseEngine connects the session to the browser embedded in Pinky's window.
// Tools then drive that web view instead of fetching pages.
func (s *Session) UseEngine(e Engine) {
	s.mu.Lock()
	s.engine = e
	s.mu.Unlock()
}

// MarkHeadless tells tools there is no Pinky window. They say so instead
// of fetching pages. The desktop app does not call this.
func (s *Session) MarkHeadless() {
	s.mu.Lock()
	s.headless = true
	s.mu.Unlock()
}

// MarkDesktop tells tools the browser lives in Pinky's window. Until that
// window's web view is connected, they say so instead of fetching pages.
func (s *Session) MarkDesktop() {
	s.mu.Lock()
	s.desktop = true
	s.mu.Unlock()
}

// FailEngine records that the window could not create its web view, so
// tools report that instead of fetching pages.
func (s *Session) FailEngine(err error) {
	s.mu.Lock()
	s.engineErr = err
	s.mu.Unlock()
}

func (s *Session) engineNow() Engine {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.engine
}

func (s *Session) noWindow() error {
	s.mu.Lock()
	headless := s.headless
	desktop := s.desktop
	engine := s.engine
	fail := s.engineErr
	s.mu.Unlock()
	if engine != nil {
		return nil
	}
	if fail != nil {
		return fail
	}
	if headless {
		return errors.New("the browser is embedded in Pinky's window; open Pinky to use it")
	}
	if desktop {
		return errors.New("the browser embedded in Pinky's window is still starting")
	}
	return nil
}

// UseChrome opens a Chrome or Chromium window Pinky controls. The person
// can use that window; the tools use the same page.
func (s *Session) UseChrome(ctx context.Context) error {
	s.mu.Lock()
	dir := s.dir
	url := s.page.URL
	s.mu.Unlock()
	if dir == "" {
		dir = os.TempDir()
	}
	b, err := startChrome(ctx, filepath.Join(dir, "chrome-profile"), url)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.cdp != nil {
		s.cdp.Close()
	}
	s.cdp = b
	s.mu.Unlock()
	return nil
}

// Close stops a Chrome window Pinky started.
func (s *Session) Close() {
	s.mu.Lock()
	c := s.cdp
	s.cdp = nil
	s.mu.Unlock()
	if c != nil {
		c.Close()
	}
}

// Navigate opens url in the active tab. The history gains it.
func (s *Session) Navigate(ctx context.Context, raw string) (Page, error) {
	return s.NavigateTab(ctx, "", raw)
}

// NavigateTab opens url in tab id, or the active tab when id is empty.
func (s *Session) NavigateTab(ctx context.Context, id, raw string) (Page, error) {
	if e := s.engineNow(); e != nil {
		return e.Navigate(ctx, id, raw)
	}
	if err := s.noWindow(); err != nil {
		return Page{}, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Page{}, errors.New("deskweb: empty url")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	page, err := s.navigate(ctx, raw)
	if err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	if s.at < len(s.hist)-1 {
		s.hist = s.hist[:s.at+1]
	}
	s.hist = append(s.hist, page.URL)
	s.at = len(s.hist) - 1
	s.page = page
	s.mu.Unlock()
	return page, nil
}

func (s *Session) navigate(ctx context.Context, raw string) (Page, error) {
	s.mu.Lock()
	cdp := s.cdp
	s.mu.Unlock()
	if cdp != nil {
		return cdp.Navigate(ctx, raw)
	}
	return s.http.Navigate(ctx, raw)
}

// Back and Forward move through the addresses this session opened.
func (s *Session) Back(ctx context.Context) (Page, error)    { return s.BackTab(ctx, "") }
func (s *Session) Forward(ctx context.Context) (Page, error) { return s.ForwardTab(ctx, "") }

// BackTab and ForwardTab move one tab, or the active tab when id is empty.
func (s *Session) BackTab(ctx context.Context, id string) (Page, error) {
	return s.shift(ctx, id, -1)
}
func (s *Session) ForwardTab(ctx context.Context, id string) (Page, error) {
	return s.shift(ctx, id, 1)
}

func (s *Session) shift(ctx context.Context, id string, d int) (Page, error) {
	if e := s.engineNow(); e != nil {
		if d < 0 {
			return e.Back(ctx, id)
		}
		return e.Forward(ctx, id)
	}
	if err := s.noWindow(); err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	next := s.at + d
	if next < 0 || next >= len(s.hist) {
		s.mu.Unlock()
		return s.page, errors.New("deskweb: no further page")
	}
	raw := s.hist[next]
	s.mu.Unlock()
	page, err := s.navigate(ctx, raw)
	if err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	s.at = next
	s.page = page
	s.mu.Unlock()
	return page, nil
}

// Reload fetches the current address again.
func (s *Session) Reload(ctx context.Context) (Page, error) { return s.ReloadTab(ctx, "") }

// ReloadTab reloads tab id, or the active tab when id is empty.
func (s *Session) ReloadTab(ctx context.Context, id string) (Page, error) {
	if e := s.engineNow(); e != nil {
		return e.Reload(ctx, id)
	}
	if err := s.noWindow(); err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	raw := s.page.URL
	s.mu.Unlock()
	if raw == "" {
		return Page{}, errors.New("deskweb: no page")
	}
	page, err := s.navigate(ctx, raw)
	if err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	s.page = page
	s.mu.Unlock()
	return page, nil
}

// State is the active tab's address and title.
func (s *Session) State() (url, title string) {
	if e := s.engineNow(); e != nil {
		if tabs, err := e.Tabs(context.Background()); err == nil {
			for _, t := range tabs {
				if t.Active {
					return t.URL, t.Title
				}
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.page.URL, s.page.Title
}

// Content is the current page.
func (s *Session) Content(ctx context.Context) (Page, error) { return s.ContentTab(ctx, "") }

// ContentTab reads tab id, or the active tab when id is empty.
func (s *Session) ContentTab(ctx context.Context, id string) (Page, error) {
	if e := s.engineNow(); e != nil {
		return e.Content(ctx, id)
	}
	if err := s.noWindow(); err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	cdp := s.cdp
	page := s.page
	s.mu.Unlock()
	if cdp != nil {
		p, err := cdp.Content(ctx)
		if err != nil {
			return Page{}, err
		}
		s.mu.Lock()
		s.page = p
		s.mu.Unlock()
		return p, nil
	}
	if page.URL == "" {
		return s.http.Content(), nil
	}
	return page, nil
}

// Click activates an element by CSS selector or by its text. A link it
// opens is added to the session history, so Back returns to this page.
func (s *Session) Click(ctx context.Context, selector, text string) (Page, error) {
	return s.ClickTab(ctx, "", selector, text)
}

// ClickTab clicks in tab id, or the active tab when id is empty.
func (s *Session) ClickTab(ctx context.Context, id, selector, text string) (Page, error) {
	if e := s.engineNow(); e != nil {
		return e.Click(ctx, id, selector, text)
	}
	if err := s.noWindow(); err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	cdp := s.cdp
	before := s.page.URL
	s.mu.Unlock()
	var page Page
	var err error
	if cdp != nil {
		page, err = cdp.Click(ctx, selector, text)
	} else {
		page, err = s.http.Click(selector, text)
	}
	if err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	s.page = page
	if page.URL != "" && page.URL != before {
		if s.at < len(s.hist)-1 {
			s.hist = s.hist[:s.at+1]
		}
		s.hist = append(s.hist, page.URL)
		s.at = len(s.hist) - 1
	}
	s.mu.Unlock()
	return page, nil
}

// Type writes value into the element.
func (s *Session) Type(ctx context.Context, selector, text, value string) (Page, error) {
	return s.TypeTab(ctx, "", selector, text, value)
}

// TypeTab types into tab id, or the active tab when id is empty.
func (s *Session) TypeTab(ctx context.Context, id, selector, text, value string) (Page, error) {
	if e := s.engineNow(); e != nil {
		return e.Type(ctx, id, selector, text, value)
	}
	if err := s.noWindow(); err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	cdp := s.cdp
	s.mu.Unlock()
	var page Page
	var err error
	if cdp != nil {
		page, err = cdp.Type(ctx, selector, text, value)
	} else {
		page, err = s.http.Type(selector, text, value)
	}
	if err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	s.page = page
	s.mu.Unlock()
	return page, nil
}

// Scroll moves the page by dy pixels.
func (s *Session) Scroll(ctx context.Context, dy int) error { return s.ScrollTab(ctx, "", dy) }

// ScrollTab scrolls tab id, or the active tab when id is empty.
func (s *Session) ScrollTab(ctx context.Context, id string, dy int) error {
	if e := s.engineNow(); e != nil {
		return e.Scroll(ctx, id, dy)
	}
	if err := s.noWindow(); err != nil {
		return err
	}
	s.mu.Lock()
	cdp := s.cdp
	s.mu.Unlock()
	if cdp != nil {
		return cdp.Scroll(ctx, dy)
	}
	s.http.Scroll(dy)
	return nil
}

// Eval runs JavaScript in the page and returns its value as text. Without
// Chrome, only the document fields documented on the tool are answered.
func (s *Session) Eval(ctx context.Context, expr string) (string, error) {
	return s.EvalTab(ctx, "", expr)
}

// EvalTab evaluates JavaScript in tab id, or the active tab when id is empty.
func (s *Session) EvalTab(ctx context.Context, id, expr string) (string, error) {
	if e := s.engineNow(); e != nil {
		return e.Eval(ctx, id, expr)
	}
	if err := s.noWindow(); err != nil {
		return "", err
	}
	s.mu.Lock()
	cdp := s.cdp
	s.mu.Unlock()
	if cdp != nil {
		return cdp.Eval(ctx, expr)
	}
	return s.http.Eval(expr)
}

// Screenshot writes a PNG of the page and returns its path. Without Chrome
// the file is a note of the page text, still a path the task can report.
func (s *Session) Screenshot(ctx context.Context) (string, error) {
	return s.ScreenshotTab(ctx, "")
}

// ScreenshotTab writes a PNG of tab id, or of the active tab when id is empty.
func (s *Session) ScreenshotTab(ctx context.Context, id string) (string, error) {
	if e := s.engineNow(); e != nil {
		return s.saveShot(e, ctx, id)
	}
	if err := s.noWindow(); err != nil {
		return "", err
	}
	s.mu.Lock()
	dir := s.dir
	cdp := s.cdp
	page := s.page
	s.mu.Unlock()
	if dir == "" {
		dir = os.TempDir()
	}
	shots := filepath.Join(dir, "shots")
	if err := os.MkdirAll(shots, 0o755); err != nil {
		return "", err
	}
	name := filepath.Join(shots, time.Now().Format("20060102-150405.000")+".png")
	var data []byte
	var err error
	if cdp != nil {
		data, err = cdp.Screenshot(ctx)
		if err != nil {
			return "", err
		}
	} else {
		data = []byte(page.Summary())
		name = strings.TrimSuffix(name, ".png") + ".txt"
	}
	if err := os.WriteFile(name, data, 0o600); err != nil {
		return "", err
	}
	s.mu.Lock()
	s.lastShot = name
	s.mu.Unlock()
	return name, nil
}

// LastScreenshot is the path of the last screenshot, or "".
func (s *Session) LastScreenshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastShot
}

// Bookmarks lists saved addresses.
func (s *Session) Bookmarks() []Bookmark {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Bookmark(nil), s.bookmarks...)
}

// SaveBookmark remembers the current page, or url when it is not empty.
func (s *Session) SaveBookmark(title, raw string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if raw == "" {
		raw = s.page.URL
	}
	if title == "" {
		title = s.page.Title
	}
	if raw == "" {
		return errors.New("deskweb: no address to save")
	}
	if title == "" {
		title = raw
	}
	s.bookmarks = append(s.bookmarks, Bookmark{Title: title, URL: raw})
	return s.saveLocked()
}

func (s *Session) statePath() string {
	return filepath.Join(s.dir, "webview.json")
}

func (s *Session) saveLocked() error {
	if s.dir == "" {
		return nil
	}
	data, err := json.MarshalIndent(stored{Bookmarks: s.bookmarks}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.statePath() + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.statePath())
}

// PageError is a page that has no such element, or an eval the fetch
// browser cannot run.
func PageError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (s *Session) String() string {
	u, t := s.State()
	return fmt.Sprintf("%s %s", u, t)
}
