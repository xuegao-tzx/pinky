package deskweb

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// cdpBrowser drives a Chrome window through the DevTools protocol.
type cdpBrowser struct {
	cmd    *exec.Cmd
	ws     *websocket.Conn
	mu     sync.Mutex
	next   int
	closed bool
}

func startChrome(ctx context.Context, profile, openURL string) (*cdpBrowser, error) {
	bin := findChrome()
	if bin == "" {
		return nil, errors.New("deskweb: no Chrome or Chromium (set PINKY_CHROME to the program)")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	if err := os.MkdirAll(profile, 0o755); err != nil {
		return nil, err
	}
	if openURL == "" {
		openURL = "about:blank"
	}
	args := []string{
		"--remote-debugging-port=" + fmt.Sprint(port),
		"--user-data-dir=" + profile,
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
		openURL,
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	wsURL, err := waitDebugger(ctx, port)
	if err != nil {
		cmd.Process.Kill()
		return nil, err
	}
	ws, err := websocket.Dial(wsURL, "", "http://127.0.0.1/")
	if err != nil {
		cmd.Process.Kill()
		return nil, err
	}
	b := &cdpBrowser{cmd: cmd, ws: ws}
	_, _ = b.call(ctx, "Page.enable", map[string]any{})
	_, _ = b.call(ctx, "Runtime.enable", map[string]any{})
	return b, nil
}

func findChrome() string {
	if p := strings.TrimSpace(os.Getenv("PINKY_CHROME")); p != "" {
		return p
	}
	switch runtime.GOOS {
	case "darwin":
		cands := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
		for _, c := range cands {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c
			}
		}
	case "windows":
		for _, c := range []string{
			filepath.Join(os.Getenv("PROGRAMFILES"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("PROGRAMFILES(X86)"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
		} {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c
			}
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func waitDebugger(ctx context.Context, port int) (string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	deadline := time.Now().Add(8 * time.Second)
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		res, err := http.DefaultClient.Do(req)
		if err == nil {
			var v struct {
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			json.NewDecoder(res.Body).Decode(&v)
			res.Body.Close()
			if v.WebSocketDebuggerURL != "" {
				return v.WebSocketDebuggerURL, nil
			}
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			if err == nil {
				err = errors.New("no debugger socket")
			}
			return "", fmt.Errorf("deskweb: chrome did not open a debugger: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (b *cdpBrowser) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	if b.ws != nil {
		b.ws.Close()
	}
	cmd := b.cmd
	b.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
		cmd.Wait()
	}
}

type cdpResult struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (b *cdpBrowser) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || b.ws == nil {
		return nil, errors.New("deskweb: browser closed")
	}
	b.next++
	id := b.next
	body, _ := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err := websocket.Message.Send(b.ws, string(body)); err != nil {
		return nil, err
	}
	_ = b.ws.SetDeadline(time.Now().Add(20 * time.Second))
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var raw string
		if err := websocket.Message.Receive(b.ws, &raw); err != nil {
			return nil, err
		}
		var res cdpResult
		if json.Unmarshal([]byte(raw), &res) != nil || res.ID != id {
			continue
		}
		if res.Error != nil {
			return nil, errors.New(res.Error.Message)
		}
		return res.Result, nil
	}
}

func (b *cdpBrowser) Navigate(ctx context.Context, raw string) (Page, error) {
	if _, err := b.call(ctx, "Page.navigate", map[string]any{"url": raw}); err != nil {
		return Page{}, err
	}
	time.Sleep(300 * time.Millisecond)
	return b.Content(ctx)
}

func (b *cdpBrowser) Content(ctx context.Context) (Page, error) {
	expr := `(() => {
	  const els = [...document.querySelectorAll('a,button,input,textarea,select,[role=button]')].slice(0, 80).map((e, i) => ({
	    selector: e.id ? '#' + e.id : (e.name ? e.tagName.toLowerCase() + '[name=' + e.name + ']' : e.tagName.toLowerCase() + ':nth(' + (i+1) + ')'),
	    tag: e.tagName.toLowerCase(),
	    text: ((e.innerText || e.getAttribute('aria-label') || e.placeholder || e.value || '') + '').slice(0, 120),
	    href: e.href || '',
	    value: e.value || ''
	  }));
	  return {url: location.href, title: document.title, text: (document.body && document.body.innerText || '').slice(0, 8000), elements: els};
	})()`
	return b.evalPage(ctx, expr)
}

func (b *cdpBrowser) Click(ctx context.Context, selector, text string) (Page, error) {
	expr := fmt.Sprintf(`(() => {
	  const sel = %s, text = %s;
	  let el = sel ? document.querySelector(sel) : null;
	  if (!el && text) {
	    const low = text.toLowerCase();
	    el = [...document.querySelectorAll('a,button,input,textarea,select,[role=button]')].find(e => ((e.innerText||e.value||e.placeholder||'')+'').toLowerCase().includes(low));
	  }
	  if (!el) return {ok:false};
	  el.click();
	  return {ok:true};
	})()`, jsString(selector), jsString(text))
	ok, err := b.evalValue(ctx, expr)
	if err != nil {
		return Page{}, err
	}
	if !strings.Contains(ok, `"ok":true`) && !strings.Contains(ok, `"ok": true`) {
		return Page{}, errNotFound(selector, text)
	}
	time.Sleep(200 * time.Millisecond)
	return b.Content(ctx)
}

func (b *cdpBrowser) Type(ctx context.Context, selector, text, value string) (Page, error) {
	expr := fmt.Sprintf(`(() => {
	  const sel = %s, text = %s, value = %s;
	  let el = sel ? document.querySelector(sel) : null;
	  if (!el && text) {
	    const low = text.toLowerCase();
	    el = [...document.querySelectorAll('input,textarea,select')].find(e => ((e.innerText||e.placeholder||e.name||'')+'').toLowerCase().includes(low));
	  }
	  if (!el) return {ok:false};
	  el.focus();
	  el.value = value;
	  el.dispatchEvent(new Event('input', {bubbles:true}));
	  el.dispatchEvent(new Event('change', {bubbles:true}));
	  return {ok:true};
	})()`, jsString(selector), jsString(text), jsString(value))
	ok, err := b.evalValue(ctx, expr)
	if err != nil {
		return Page{}, err
	}
	if !strings.Contains(ok, `"ok":true`) && !strings.Contains(ok, `"ok": true`) {
		return Page{}, errNotFound(selector, text)
	}
	return b.Content(ctx)
}

func (b *cdpBrowser) Scroll(ctx context.Context, dy int) error {
	_, err := b.evalValue(ctx, fmt.Sprintf("window.scrollBy(0, %d); 'ok'", dy))
	return err
}

func (b *cdpBrowser) Eval(ctx context.Context, expr string) (string, error) {
	return b.evalValue(ctx, expr)
}

func (b *cdpBrowser) Screenshot(ctx context.Context) ([]byte, error) {
	raw, err := b.call(ctx, "Page.captureScreenshot", map[string]any{"format": "png"})
	if err != nil {
		return nil, err
	}
	var res struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(res.Data)
}

func (b *cdpBrowser) evalPage(ctx context.Context, expr string) (Page, error) {
	val, err := b.evalValue(ctx, expr)
	if err != nil {
		return Page{}, err
	}
	var p Page
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return Page{Text: val}, nil
	}
	return p, nil
}

func (b *cdpBrowser) evalValue(ctx context.Context, expr string) (string, error) {
	raw, err := b.call(ctx, "Runtime.evaluate", map[string]any{
		"expression": expr, "returnByValue": true, "awaitPromise": true,
	})
	if err != nil {
		return "", err
	}
	var res struct {
		Result struct {
			Value json.RawMessage `json:"value"`
			Type  string          `json:"type"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	if res.ExceptionDetails != nil {
		return "", errors.New(res.ExceptionDetails.Text)
	}
	if len(res.Result.Value) == 0 {
		return "", nil
	}
	var s string
	if json.Unmarshal(res.Result.Value, &s) == nil && res.Result.Type == "string" {
		return s, nil
	}
	return string(bytes.TrimSpace(res.Result.Value)), nil
}

func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
