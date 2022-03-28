package template

import (
	"fmt"
	"html"
	"html/template"
	"io"
	"log"
	"os"
)

var templateMap map[string]*template.Template

var Login *template.Template
var AlertList *template.Template
var Alert *template.Template
var Portfolio *template.Template
var Asset *template.Template

func Init() {
	Login = template.Must(template.ParseFiles(
		"template/base.tmpl",
		"template/login.tmpl",
	))
	AlertList = template.Must(template.ParseFiles(
		"template/base.tmpl",
		"template/alert-form.tmpl",
		"template/alert-list.tmpl",
	))
	Alert = template.Must(template.ParseFiles(
		"template/base.tmpl",
		"template/alert-form.tmpl",
		"template/alert.tmpl",
	))
	Portfolio = template.Must(template.ParseFiles(
		"template/base.tmpl",
		"template/portfolio.tmpl",
	))
	Asset = template.Must(template.ParseFiles(
		"template/base.tmpl",
		"template/asset.tmpl",
	))
}

func Render(tmpl *template.Template, writer io.Writer, data any) {
	if err := tmpl.ExecuteTemplate(writer, "base", data); err != nil {
		log.Printf("internal error: %s\n", err.Error())

		// Write errors to JS console when running in DEBUG mode.
		if os.Getenv("DEBUG") == "true" {
			fmt.Fprintf(
				writer,
				`<script>
					const alert = document.createElement('div')
					alert.innerHTML = '%s'
					console.error(alert.textContent)
				</script>`,
				html.EscapeString(err.Error()),
			)
		}
	}
}
