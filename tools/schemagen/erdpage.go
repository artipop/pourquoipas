package main

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"unicode"
)

// erdPagePath is the same diagram dressed as a page to open in a browser: a
// stylesheet, a table of contents and a few counts around the markdown's own
// sections. It is generated beside erd.md for the reason the HCL is, and it
// used to be the example of what happens otherwise — a Python script somebody
// had to remember to run after the generator.
const erdPagePath = "docs/schema/erd.html"

//go:embed erdpage.tmpl
var erdPageTemplate string

// mermaidTheme is the diagram's own palette. Every colour is spelled out,
// because inheriting was the bug: the host renders mermaid in its own theme,
// so `theme: neutral` came out as dark fills with dark text. `base` plus
// themeVariables is the documented way to own the palette outright. No
// `background`, so the plate shows through and the diagram sits on paper in
// both themes.
const mermaidTheme = "{'theme':'base','themeVariables':{" +
	"'fontFamily':'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace'," +
	"'fontSize':'13px'," +
	"'primaryColor':'#ffffff'," +
	"'primaryTextColor':'#14171d'," +
	"'primaryBorderColor':'#aab3bf'," +
	"'secondaryColor':'#f4f6f8'," +
	"'tertiaryColor':'#ffffff'," +
	"'lineColor':'#7c8493'," +
	"'textColor':'#14171d'," +
	"'nodeBorder':'#aab3bf'," +
	"'mainBkg':'#ffffff'," +
	"'attributeBackgroundColorOdd':'#ffffff'," +
	"'attributeBackgroundColorEven':'#f4f6f8'" +
	"}}"

type erdPage struct {
	TotalTables, TotalKeys, Cascades int
	Nav, Preamble, Main              string
}

// renderERDPage builds the page from the groups the markdown is built from,
// not by reading the markdown back: the preamble is the header's paragraphs
// minus the pointer to the prose documents, which becomes the page's footer.
func renderERDPage() (string, error) {
	tmpl, err := template.New("erd").Parse(erdPageTemplate)
	if err != nil {
		return "", err
	}

	var page erdPage
	var nav, sections []string
	for i, g := range erdGroups() {
		tables, keys := len(g.tables), 0
		for _, t := range g.tables {
			keys += len(t.FKs)
			for _, fk := range t.FKs {
				if strings.EqualFold(string(fk.OnDelete), "CASCADE") {
					page.Cascades++
				}
			}
		}
		page.TotalTables += tables
		page.TotalKeys += keys

		diagram := strings.TrimPrefix(renderERD(g.tables), "```mermaid\n")
		diagram = strings.TrimRightFunc(strings.TrimSuffix(diagram, "```\n"), unicode.IsSpace)
		diagram = "%%{init: " + mermaidTheme + "}%%\n" + diagram

		id := fmt.Sprintf("g%d", i)
		nav = append(nav, fmt.Sprintf(`      <li><a href="#%s"><span class="nav-name">%s</span>`+
			`<span class="nav-count">%d</span></a></li>`, id, escapeHTML(g.title), tables))
		sections = append(sections, fmt.Sprintf(`      <section class="group" id="%s">
        <p class="eyebrow">%d таблиц%s</p>
        <h2>%s</h2>
        <div class="prose">%s</div>
        <figure class="plate">
          <pre class="mermaid">%s</pre>
        </figure>
      </section>`, id, tables, keysLabel(keys), escapeHTML(g.title), paragraphs(g.why), escapeHTML(diagram)))
	}
	page.Nav = strings.Join(nav, "\n")
	page.Main = strings.Join(sections, "\n")
	page.Preamble = paragraphs(erdIntro + "\n\n" + erdKey)

	var b strings.Builder
	if err := tmpl.Execute(&b, page); err != nil {
		return "", err
	}
	return b.String(), nil
}

// keysLabel is the count of keys in a group's eyebrow, in the language the
// page is written in, and nothing when there are none.
func keysLabel(n int) string {
	switch {
	case n == 0:
		return ""
	case n == 1:
		return fmt.Sprintf(" · %d ключ", n)
	case n < 5:
		return fmt.Sprintf(" · %d ключа", n)
	}
	return fmt.Sprintf(" · %d ключей", n)
}

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#x27;")

func escapeHTML(s string) string { return htmlEscaper.Replace(s) }

var (
	codeSpan   = regexp.MustCompile("`([^`]+)`")
	strongSpan = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	blankLine  = regexp.MustCompile(`\n\s*\n`)
)

// inline turns `code` and **bold** into HTML and escapes everything else —
// the only markdown the groups' prose uses.
func inline(s string) string {
	s = escapeHTML(s)
	s = codeSpan.ReplaceAllString(s, "<code>$1</code>")
	return strongSpan.ReplaceAllString(s, "<strong>$1</strong>")
}

// paragraphs renders blank-line-separated prose: a paragraph of `- ` lines is a
// list, anything else one paragraph with its line breaks folded.
func paragraphs(block string) string {
	var out []string
	for _, para := range blankLine.Split(strings.TrimSpace(block), -1) {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if !strings.HasPrefix(para, "- ") {
			out = append(out, "<p>"+inline(strings.Join(strings.Fields(para), " "))+"</p>")
			continue
		}
		var items []string
		for _, line := range strings.Split(para, "\n") {
			if strings.HasPrefix(line, "- ") || len(items) == 0 {
				items = append(items, line)
			} else {
				items[len(items)-1] += "\n" + line
			}
		}
		var b strings.Builder
		b.WriteString("<ul>")
		for _, item := range items {
			b.WriteString("<li>" + inline(strings.TrimSpace(strings.TrimPrefix(item, "- "))) + "</li>")
		}
		b.WriteString("</ul>")
		out = append(out, b.String())
	}
	return strings.Join(out, "\n")
}
