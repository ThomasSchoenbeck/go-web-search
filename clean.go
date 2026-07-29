package main

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type imageRef struct {
	URL    string
	Alt    string
	Width  int
	Height int
}

type cleanedDoc struct {
	Title     string
	CleanHTML string
	Text      string
	Images    []imageRef
}

// Elements carrying no content worth keeping. Removing chrome (nav, header,
// footer, aside) is what makes the text usable for an LLM rather than a wall of
// menu items repeated on every page of a site.
var dropTags = map[atom.Atom]bool{
	atom.Script:   true,
	atom.Style:    true,
	atom.Noscript: true,
	atom.Iframe:   true,
	atom.Svg:      true,
	atom.Canvas:   true,
	atom.Template: true,
	atom.Form:     true,
	atom.Nav:      true,
	atom.Header:   true,
	atom.Footer:   true,
	atom.Aside:    true,
	atom.Object:   true,
	atom.Embed:    true,
}

// Elements that imply a line break when flattening to text.
var blockTags = map[atom.Atom]bool{
	atom.P: true, atom.Div: true, atom.Br: true, atom.Li: true,
	atom.Tr: true, atom.Section: true, atom.Article: true,
	atom.H1: true, atom.H2: true, atom.H3: true,
	atom.H4: true, atom.H5: true, atom.H6: true,
	atom.Blockquote: true, atom.Pre: true, atom.Td: true, atom.Th: true,
}

// cleanHTML parses a document and returns the body stripped of noise, a plain
// text rendering, and every image referenced in the body.
//
// resolve turns relative image URLs absolute; pass the page's final URL.
func cleanHTML(raw string, resolve func(string) string) (*cleanedDoc, error) {
	root, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return nil, err
	}

	doc := &cleanedDoc{}
	doc.Title = findTitle(root)

	body := findBody(root)
	if body == nil {
		body = root
	}

	// Images are collected before pruning: a hero image inside <header> is
	// still a picture of the thing the page is about.
	collectImages(body, resolve, &doc.Images)
	prune(body)

	var out strings.Builder
	if err := html.Render(&out, body); err != nil {
		return nil, err
	}
	doc.CleanHTML = out.String()
	doc.Text = extractText(body)
	return doc, nil
}

func findTitle(n *html.Node) string {
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if title != "" {
			return
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Title && n.FirstChild != nil {
			title = strings.TrimSpace(n.FirstChild.Data)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return title
}

func findBody(n *html.Node) *html.Node {
	var body *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if body != nil {
			return
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Body {
			body = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return body
}

func collectImages(n *html.Node, resolve func(string) string, out *[]imageRef) {
	seen := make(map[string]bool)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Img {
			img := imageRef{}
			for _, a := range n.Attr {
				switch strings.ToLower(a.Key) {
				case "src":
					img.URL = a.Val
				case "data-src":
					if img.URL == "" {
						img.URL = a.Val
					}
				case "alt":
					img.Alt = a.Val
				case "width":
					img.Width, _ = strconv.Atoi(strings.TrimSuffix(a.Val, "px"))
				case "height":
					img.Height, _ = strconv.Atoi(strings.TrimSuffix(a.Val, "px"))
				}
			}
			if img.URL != "" {
				img.URL = resolve(img.URL)
				if img.URL != "" && !seen[img.URL] {
					seen[img.URL] = true
					*out = append(*out, img)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
}

// prune removes noise nodes in place.
func prune(n *html.Node) {
	var next *html.Node
	for c := n.FirstChild; c != nil; c = next {
		next = c.NextSibling
		switch {
		case c.Type == html.CommentNode:
			n.RemoveChild(c)
		case c.Type == html.ElementNode && dropTags[c.DataAtom]:
			n.RemoveChild(c)
		default:
			prune(c)
		}
	}
}

func extractText(n *html.Node) string {
	var b strings.Builder

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			if dropTags[n.DataAtom] {
				return
			}
			if blockTags[n.DataAtom] {
				b.WriteString("\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && blockTags[n.DataAtom] {
			b.WriteString("\n")
		}
	}
	walk(n)
	return collapseWhitespace(b.String())
}

// collapseWhitespace squeezes runs of spaces and blank lines, which HTML
// indentation produces in enormous quantities and which cost tokens for nothing.
func collapseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := false

	for _, line := range lines {
		trimmed := strings.Join(strings.Fields(line), " ")
		if trimmed == "" {
			if blank || len(out) == 0 {
				continue
			}
			blank = true
			out = append(out, "")
			continue
		}
		blank = false
		out = append(out, trimmed)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
