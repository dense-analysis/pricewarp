package template

import (
	"io"
	"html/template"
)

var templateMap map[string]*template.Template

var Login *template.Template
var AlertList *template.Template
var Alert *template.Template

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
}

func Render(tmpl *template.Template, writer io.Writer, data interface{}) {
	tmpl.ExecuteTemplate(writer, "base", data)
}
