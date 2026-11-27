package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/DIYgod/pinky/internal/deskweb"
)

// browserHost is the browser embedded in Pinky's window. Each tab is a
// native web view (WKWebView on macOS) sharing the default website data
// store, so cookies and the login survive quitting Pinky.
type browserHost struct {
	mu     sync.Mutex
	win    *mygo.Window
	tabs   []*browserTab
	active int
	next   int
}

type browserTab struct {
	id    string
	view  *mygo.WebView
	url   string
	title string
}

func newBrowserHost(win *mygo.Window) (*browserHost, error) {
	h := &browserHost{win: win}
	if _, err := h.NewTab(context.Background(), ""); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *browserHost) activeTab() *browserTab {
	if len(h.tabs) == 0 {
		return nil
	}
	if h.active < 0 || h.active >= len(h.tabs) {
		h.active = 0
	}
	return h.tabs[h.active]
}

func (h *browserHost) find(id string) *browserTab {
	id = strings.TrimSpace(id)
	if id == "" {
		return h.activeTab()
	}
	for _, t := range h.tabs {
		if t.id == id {
			return t
		}
	}
	return nil
}

func (h *browserHost) Tabs(context.Context) ([]deskweb.Tab, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.tabsLocked(), nil
}

func (h *browserHost) tabsLocked() []deskweb.Tab {
	out := make([]deskweb.Tab, len(h.tabs))
	for i, t := range h.tabs {
		out[i] = deskweb.Tab{ID: t.id, URL: t.pageURL(), Title: t.pageTitle(), Active: i == h.active}
	}
	return out
}

func (h *browserHost) NewTab(ctx context.Context, raw string) (deskweb.Tab, error) {
	h.mu.Lock()
	h.next++
	id := fmt.Sprintf("tab%d", h.next)
	raw = strings.TrimSpace(raw)
	view, err := h.win.NewWebView(mygo.WebViewOptions{URL: raw})
	if err != nil {
		h.mu.Unlock()
		return deskweb.Tab{}, err
	}
	t := &browserTab{id: id, view: view, url: raw}
	h.tabs = append(h.tabs, t)
	h.active = len(h.tabs) - 1
	h.mu.Unlock()
	if raw != "" {
		if page, nerr := h.Content(ctx, id); nerr == nil {
			h.mu.Lock()
			t.url, t.title = page.URL, page.Title
			h.mu.Unlock()
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return deskweb.Tab{ID: id, URL: t.url, Title: t.title, Active: true}, nil
}

func (h *browserHost) CloseTab(_ context.Context, id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.tabs) <= 1 {
		return fmt.Errorf("the last tab stays open")
	}
	for i, t := range h.tabs {
		if t.id != id && id != "" {
			continue
		}
		if id == "" && i != h.active {
			continue
		}
		t.view.Destroy()
		h.tabs = append(h.tabs[:i], h.tabs[i+1:]...)
		if h.active >= len(h.tabs) {
			h.active = len(h.tabs) - 1
		}
		return nil
	}
	return fmt.Errorf("no tab %s", id)
}

func (h *browserHost) SwitchTab(_ context.Context, id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, t := range h.tabs {
		if t.id == id {
			h.active = i
			return nil
		}
	}
	return fmt.Errorf("no tab %s", id)
}

func (h *browserHost) Navigate(ctx context.Context, id, raw string) (deskweb.Page, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return deskweb.Page{}, fmt.Errorf("empty url")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	t := h.with(id)
	if t == nil {
		return deskweb.Page{}, fmt.Errorf("no tab")
	}
	if err := t.view.Page().LoadURL(raw); err != nil {
		return deskweb.Page{}, err
	}
	waitNav(ctx, t.view.Page(), raw)
	return h.Content(ctx, t.id)
}

func (h *browserHost) Back(ctx context.Context, id string) (deskweb.Page, error) {
	t := h.with(id)
	if t == nil {
		return deskweb.Page{}, fmt.Errorf("no tab")
	}
	t.view.Page().GoBack()
	waitNav(ctx, t.view.Page(), "")
	return h.Content(ctx, t.id)
}

func (h *browserHost) Forward(ctx context.Context, id string) (deskweb.Page, error) {
	t := h.with(id)
	if t == nil {
		return deskweb.Page{}, fmt.Errorf("no tab")
	}
	t.view.Page().GoForward()
	waitNav(ctx, t.view.Page(), "")
	return h.Content(ctx, t.id)
}

func (h *browserHost) Reload(ctx context.Context, id string) (deskweb.Page, error) {
	t := h.with(id)
	if t == nil {
		return deskweb.Page{}, fmt.Errorf("no tab")
	}
	t.view.Page().Reload()
	waitNav(ctx, t.view.Page(), "")
	return h.Content(ctx, t.id)
}

func (h *browserHost) Content(ctx context.Context, id string) (deskweb.Page, error) {
	t := h.with(id)
	if t == nil {
		return deskweb.Page{}, fmt.Errorf("no tab")
	}
	raw, err := t.view.Page().EvalContext(ctx, contentJS)
	if err != nil {
		u, title := t.pageURL(), t.pageTitle()
		return deskweb.Page{URL: u, Title: title}, nil
	}
	page, err := pageFromEval(raw)
	if err != nil {
		return deskweb.Page{URL: t.pageURL(), Title: t.pageTitle()}, nil
	}
	h.mu.Lock()
	t.url, t.title = page.URL, page.Title
	h.mu.Unlock()
	return page, nil
}

func (h *browserHost) Click(ctx context.Context, id, selector, text string) (deskweb.Page, error) {
	t := h.with(id)
	if t == nil {
		return deskweb.Page{}, fmt.Errorf("no tab")
	}
	expr := fmt.Sprintf(clickJS, jsString(selector), jsString(text))
	raw, err := t.view.Page().EvalContext(ctx, expr)
	if err != nil {
		return deskweb.Page{}, err
	}
	if s, _ := raw.(string); s == "missing" || fmt.Sprint(raw) == "missing" {
		return deskweb.Page{}, fmt.Errorf("no element %s%s", selector, text)
	}
	waitNav(ctx, t.view.Page(), "")
	return h.Content(ctx, t.id)
}

func (h *browserHost) Type(ctx context.Context, id, selector, text, value string) (deskweb.Page, error) {
	t := h.with(id)
	if t == nil {
		return deskweb.Page{}, fmt.Errorf("no tab")
	}
	expr := fmt.Sprintf(typeJS, jsString(selector), jsString(text), jsString(value))
	raw, err := t.view.Page().EvalContext(ctx, expr)
	if err != nil {
		return deskweb.Page{}, err
	}
	if fmt.Sprint(raw) == "missing" {
		return deskweb.Page{}, fmt.Errorf("no element %s%s", selector, text)
	}
	return h.Content(ctx, t.id)
}

func (h *browserHost) Scroll(ctx context.Context, id string, dy int) error {
	t := h.with(id)
	if t == nil {
		return fmt.Errorf("no tab")
	}
	_, err := t.view.Page().EvalContext(ctx, fmt.Sprintf("window.scrollBy(0, %d); 'ok'", dy))
	return err
}

func (h *browserHost) Eval(ctx context.Context, id, expr string) (string, error) {
	t := h.with(id)
	if t == nil {
		return "", fmt.Errorf("no tab")
	}
	raw, err := t.view.Page().EvalContext(ctx, expr)
	if err != nil {
		return "", err
	}
	if s, ok := raw.(string); ok {
		return s, nil
	}
	return fmt.Sprint(raw), nil
}

func (h *browserHost) Screenshot(ctx context.Context, id string) ([]byte, error) {
	t := h.with(id)
	if t == nil {
		return nil, fmt.Errorf("no tab")
	}
	return t.view.CapturePage()
}

func (h *browserHost) with(id string) *browserTab {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.find(id)
}

func (h *browserHost) views() (active *mygo.WebView, tabs []deskweb.Tab) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if t := h.activeTab(); t != nil {
		active = t.view
	}
	return active, h.tabsLocked()
}

func (t *browserTab) pageURL() string {
	if t.view == nil || t.view.Page() == nil {
		return t.url
	}
	if u := t.view.Page().URL(); u != "" {
		return u
	}
	return t.url
}

func (t *browserTab) pageTitle() string {
	if t.title != "" {
		return t.title
	}
	return t.url
}

func waitNav(ctx context.Context, page *mygo.Page, want string) {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return
		}
		u := page.URL()
		if u != "" && (want == "" || strings.Contains(u, trimScheme(want)) || u == want) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func trimScheme(s string) string {
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	return s
}

func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func pageFromEval(raw any) (deskweb.Page, error) {
	s, ok := raw.(string)
	if !ok {
		s = fmt.Sprint(raw)
	}
	var body struct {
		URL      string            `json:"url"`
		Title    string            `json:"title"`
		Text     string            `json:"text"`
		Elements []deskweb.Element `json:"elements"`
	}
	if err := json.Unmarshal([]byte(s), &body); err != nil {
		return deskweb.Page{}, err
	}
	return deskweb.Page{URL: body.URL, Title: body.Title, Text: body.Text, Elements: body.Elements}, nil
}

const contentJS = `(() => {
  const els = [...document.querySelectorAll('a,button,input,textarea,select,[role=button]')].slice(0, 80).map((e, i) => ({
    selector: e.id ? '#' + CSS.escape(e.id) : (e.name ? e.tagName.toLowerCase() + '[name=' + e.name + ']' : e.tagName.toLowerCase() + ':nth(' + (i+1) + ')'),
    tag: e.tagName.toLowerCase(),
    text: ((e.innerText || e.getAttribute('aria-label') || e.placeholder || e.value || '') + '').slice(0, 120),
    href: e.href || '',
    value: e.value || ''
  }));
  return JSON.stringify({url: location.href, title: document.title, text: (document.body && document.body.innerText || '').slice(0, 8000), elements: els});
})()`

const clickJS = `(() => {
  const sel = %s, text = %s;
  let el = sel ? document.querySelector(sel) : null;
  if (!el && text) {
    const low = text.toLowerCase();
    el = [...document.querySelectorAll('a,button,input,textarea,select,[role=button]')].find(e => ((e.innerText||e.value||e.placeholder||'')+'').toLowerCase().includes(low));
  }
  if (!el) return 'missing';
  el.click();
  return 'ok';
})()`

const typeJS = `(() => {
  const sel = %s, text = %s, value = %s;
  let el = sel ? document.querySelector(sel) : null;
  if (!el && text) {
    const low = text.toLowerCase();
    el = [...document.querySelectorAll('input,textarea,select')].find(e => ((e.placeholder||e.name||e.getAttribute('aria-label')||'')+'').toLowerCase().includes(low));
  }
  if (!el) return 'missing';
  el.focus();
  el.value = value;
  el.dispatchEvent(new Event('input', {bubbles:true}));
  el.dispatchEvent(new Event('change', {bubbles:true}));
  return 'ok';
})()`
