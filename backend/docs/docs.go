// Package docs holds the policy and help pages served under /docs/.
//
// They are written as policy text with real numbers, so a search can ask
// "how much can be refunded without a pharmacist?" and get 200 000.
package docs

import (
	"bytes"
	"embed"
	"html/template"
	"regexp"
	"strings"
)

//go:embed pages/*.html
var pages embed.FS

// Order is the navigation order; titles come from each page's <h1>.
var Order = []string{"index", "authentication", "roles", "ordering", "payments", "refunds", "delivery", "prescriptions", "errors", "faq"}

var layout = template.Must(template.New("layout").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · Dorixona</title>
<style>
:root{--bg:#fbfcfc;--text:#1d2329;--muted:#5d6872;--line:#e2e6ea;--accent:#0b7a5a;--code:#f0f3f5}
@media (prefers-color-scheme:dark){:root{--bg:#14171a;--text:#e6e9ec;--muted:#98a2ad;--line:#2c3338;--accent:#3ccf9a;--code:#1f2429}}
*{box-sizing:border-box}body{margin:0;font:16px/1.6 system-ui,sans-serif;background:var(--bg);color:var(--text)}
.wrap{display:flex;gap:40px;max-width:1080px;margin:0 auto;padding:32px 16px}
nav{flex:0 0 200px;font-size:14px}nav a{display:block;padding:4px 0;color:var(--muted);text-decoration:none}nav a.on{color:var(--accent);font-weight:600}
main{flex:1;min-width:0}h1{font-size:28px;line-height:1.2;margin:0 0 16px}h2{font-size:20px;margin:32px 0 8px}
a{color:var(--accent)}code{background:var(--code);padding:1px 5px;border-radius:4px;font-size:.9em}
pre{background:var(--code);padding:12px;border-radius:8px;overflow-x:auto;font-size:14px}pre code{padding:0;background:none}
table{border-collapse:collapse;width:100%;font-size:15px;margin:8px 0}td,th{border-bottom:1px solid var(--line);padding:6px 8px;text-align:left;vertical-align:top}
.note{border-left:3px solid var(--accent);padding:4px 12px;color:var(--muted)}
@media (max-width:720px){.wrap{flex-direction:column;gap:16px}nav{display:flex;flex-wrap:wrap;gap:4px 14px}}
</style></head><body><div class="wrap">
<nav>{{range .Nav}}<a href="/docs/{{if ne .Slug "index"}}{{.Slug}}{{end}}"{{if .On}} class="on"{{end}}>{{.Title}}</a>{{end}}</nav>
<main>{{.Body}}</main></div></body></html>`))

var h1 = regexp.MustCompile(`<h1>(.*?)</h1>`)

func title(body []byte) string {
	if m := h1.FindSubmatch(body); m != nil {
		return string(m[1])
	}
	return "Docs"
}

// Page renders one page with the navigation around it.
func Page(slug string) ([]byte, bool) {
	if strings.ContainsAny(slug, "/.") {
		return nil, false
	}
	body, err := pages.ReadFile("pages/" + slug + ".html")
	if err != nil {
		return nil, false
	}
	type link struct {
		Slug, Title string
		On          bool
	}
	var nav []link
	for _, s := range Order {
		b, err := pages.ReadFile("pages/" + s + ".html")
		if err != nil {
			continue
		}
		nav = append(nav, link{s, title(b), s == slug})
	}
	var out bytes.Buffer
	err = layout.Execute(&out, map[string]any{"Title": title(body), "Nav": nav, "Body": template.HTML(body)})
	return out.Bytes(), err == nil
}
