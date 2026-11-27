// Package deskweb is Pinky's embedded browser: a page the person can drive
// from the window and the model can drive with the webview tools. Cookies
// and saved addresses stay in Pinky's data folder. When Chrome or Chromium
// is available, the same page opens in a window Pinky owns and the tools
// speak Chrome's DevTools protocol; otherwise Pinky fetches the page itself.
package deskweb

import (
	"fmt"
	"net/url"
	"strings"
)

// Element is one control on the page.
type Element struct {
	Selector string `json:"selector"`
	Tag      string `json:"tag"`
	Text     string `json:"text"`
	Href     string `json:"href,omitempty"`
	Value    string `json:"value,omitempty"`
}

// Page is what the tools see.
type Page struct {
	URL      string
	Title    string
	Text     string
	HTML     string
	Elements []Element
}

// Summary is the page as text for a model or the panel.
func (p Page) Summary() string {
	if p.URL == "" && p.Title == "" && p.Text == "" {
		return "No page open."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "url: %s\ntitle: %s\n", p.URL, p.Title)
	if p.Text != "" {
		text := p.Text
		if len(text) > 4000 {
			text = text[:4000] + "…"
		}
		fmt.Fprintf(&b, "text:\n%s\n", text)
	}
	if len(p.Elements) > 0 {
		b.WriteString("elements:\n")
		for _, e := range p.Elements {
			fmt.Fprintf(&b, "- %s %q", e.Selector, one(e.Text))
			if e.Href != "" {
				fmt.Fprintf(&b, " href=%s", e.Href)
			}
			if e.Value != "" {
				fmt.Fprintf(&b, " value=%q", one(e.Value))
			}
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func one(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}

func resolveURL(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return base
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	if u.IsAbs() {
		return u.String()
	}
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	return b.ResolveReference(u).String()
}
