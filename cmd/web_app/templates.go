package main

import (
	"html/template"
	"io/fs"
	"path/filepath"
	"time"

	"marketier_web_app/ui"
)

type templateData struct {
	//CSRFToken       string // will come back to
	CurrentYear     int
	Flash           string
	Form            *forms.Form
	IsAuthenticated bool
	Snippet         *models.Snippet
	Snippets        []*models.Snippet
	User            *models.User
}

func humanDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.UTC().Format("02 Jan 2006 at 15:04")
}

var functions = template.FuncMap{
	"humanDate": humanDate,
}

func newTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	// Use fs.Glob() to get a slice of all filepaths in the ui.Files embedded filesystem
	// which match the pattern 'html/*.page.tmpl'. This essentially gives us a slice of
	// all the 'page' templates for the application, just like before.
	pages, err := fs.Glob(ui.Files, "html/*.page.tmpl")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		// Use the ParseFS() method to parse a specific page template from the ui.Files
		// embedded filesystem.
		ts, err := template.New(name).Funcs(functions).ParseFS(ui.Files, page)
		if err != nil {
			return nil, err
		}

		// Use ParseFS() again to add any 'layout' templates to the template set.
		ts, err = ts.ParseFS(ui.Files, "html/*.layout.tmpl")
		if err != nil {
			return nil, err
		}

		// Likewise use ParseFS() to add any 'partial' templates to the template set.
		ts, err = ts.ParseFS(ui.Files, "html/*.partial.tmpl")
		if err != nil {
			return nil, err
		}

		cache[name] = ts
	}

	return cache, nil
}
