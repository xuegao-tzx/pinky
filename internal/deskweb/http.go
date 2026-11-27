package deskweb

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// httpBrowser fetches pages and keeps a small DOM the tools can click and
// fill. Scripts on the page do not run; eval answers the document fields
// the tools ask for, and a short expression over them.
type httpBrowser struct {
	client *http.Client
	page   Page
}

func newHTTPBrowser() *httpBrowser {
	return &httpBrowser{client: &http.Client{}}
}

func (b *httpBrowser) Navigate(ctx context.Context, raw string) (Page, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return Page{}, err
	}
	res, err := b.client.Do(req)
	if err != nil {
		return Page{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return Page{}, err
	}
	final := raw
	if res.Request != nil && res.Request.URL != nil {
		final = res.Request.URL.String()
	}
	b.page = parseHTML(final, string(body))
	return b.page, nil
}

func (b *httpBrowser) Content() Page { return b.page }

func (b *httpBrowser) Click(selector, text string) (Page, error) {
	el, ok := match(b.page.Elements, selector, text)
	if !ok {
		return b.page, errNotFound(selector, text)
	}
	if el.Href != "" {
		return b.Navigate(context.Background(), resolveURL(b.page.URL, el.Href))
	}
	return b.page, nil
}

func (b *httpBrowser) Type(selector, text, value string) (Page, error) {
	for i := range b.page.Elements {
		e := &b.page.Elements[i]
		if !matches(*e, selector, text) {
			continue
		}
		e.Value = value
		b.page.HTML = setInputValue(b.page.HTML, e.Selector, value)
		return b.page, nil
	}
	return b.page, errNotFound(selector, text)
}

func (b *httpBrowser) Scroll(dy int) Page {
	return b.page
}

func (b *httpBrowser) Eval(expr string) (string, error) {
	expr = strings.TrimSpace(expr)
	switch {
	case expr == "document.title" || expr == "document.title;":
		return b.page.Title, nil
	case expr == "document.URL" || expr == "location.href" || expr == "document.location.href":
		return b.page.URL, nil
	case expr == "document.body.innerText" || expr == "document.body.textContent":
		return b.page.Text, nil
	}
	if m := selText.FindStringSubmatch(expr); m != nil {
		el, ok := match(b.page.Elements, m[1], "")
		if !ok {
			return "", errNotFound(m[1], "")
		}
		if strings.Contains(m[0], ".value") {
			return el.Value, nil
		}
		if el.Text != "" {
			return el.Text, nil
		}
		return el.Value, nil
	}
	return "", errEval(expr)
}

var selText = regexp.MustCompile(`document\.querySelector\(\s*['"]([^'"]+)['"]\s*\)\.(?:textContent|innerText|value)`)

func match(els []Element, selector, text string) (Element, bool) {
	for _, e := range els {
		if matches(e, selector, text) {
			return e, true
		}
	}
	return Element{}, false
}

func matches(e Element, selector, text string) bool {
	if selector != "" && e.Selector == selector {
		return true
	}
	if text != "" && strings.Contains(strings.ToLower(e.Text), strings.ToLower(text)) {
		return true
	}
	return selector == "" && text == ""
}

func errNotFound(selector, text string) error {
	what := selector
	if what == "" {
		what = text
	}
	return &pageError{msg: "no element " + what}
}

func errEval(expr string) error {
	return &pageError{msg: "eval supports document.title, document.URL, document.body.innerText and document.querySelector(sel).textContent or .value; got " + expr}
}

type pageError struct{ msg string }

func (e *pageError) Error() string { return e.msg }
