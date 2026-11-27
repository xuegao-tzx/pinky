package deskweb

import (
	"strings"

	"golang.org/x/net/html"
)

func parseHTML(pageURL, src string) Page {
	p := Page{URL: pageURL, HTML: src}
	root, err := html.Parse(strings.NewReader(src))
	if err != nil {
		p.Text = cleanText(src)
		return p
	}
	var text strings.Builder
	n := 0
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "script", "style":
				return
			case "title":
				if p.Title == "" {
					p.Title = cleanText(nodeText(node))
				}
			case "a", "button", "input", "textarea", "select":
				n++
				if n <= 80 {
					p.Elements = append(p.Elements, elementOf(node, n))
				}
			}
		}
		if node.Type == html.TextNode && !hiddenText(node) {
			text.WriteString(node.Data)
			text.WriteByte(' ')
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	p.Text = cleanText(text.String())
	if len(p.Text) > 8000 {
		p.Text = p.Text[:8000]
	}
	return p
}

func elementOf(node *html.Node, n int) Element {
	attrs := map[string]string{}
	for _, a := range node.Attr {
		attrs[strings.ToLower(a.Key)] = a.Val
	}
	text := cleanText(nodeText(node))
	if text == "" {
		text = attrs["aria-label"]
	}
	if text == "" {
		text = attrs["placeholder"]
	}
	if text == "" {
		text = attrs["value"]
	}
	if text == "" {
		text = attrs["name"]
	}
	el := Element{Selector: selector(node.Data, attrs, n), Tag: node.Data, Text: text, Value: attrs["value"]}
	if node.Data == "a" {
		el.Href = attrs["href"]
	}
	return el
}

func nodeText(node *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(node)
	return b.String()
}

func hiddenText(n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && (p.Data == "script" || p.Data == "style" || p.Data == "title") {
			return true
		}
	}
	return false
}

func selector(tag string, attrs map[string]string, n int) string {
	if id := attrs["id"]; id != "" && !strings.ContainsAny(id, " \"'[]") {
		return "#" + id
	}
	if name := attrs["name"]; name != "" && !strings.ContainsAny(name, " \"'[]") {
		return tag + "[name=" + name + "]"
	}
	return tag + ":nth(" + itoa(n) + ")"
}

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.Join(strings.Fields(s), " ")
}

func setInputValue(htmlSrc, selector, value string) string {
	return htmlSrc
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
