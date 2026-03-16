package htmlminifier

import (
	"os"
	"path/filepath"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
)

// MinifyDir walks dir and minifies all .html files in-place.
// Non-.html files (including .md files) are not touched.
func MinifyDir(dir string) error {
	m := minify.New()
	m.Add("text/html", &html.Minifier{KeepDefaultAttrVals: true, KeepDocumentTags: true, KeepEndTags: true})
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("application/javascript", js.Minify)

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".html" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		minified, err := m.String("text/html", string(data))
		if err != nil {
			return err
		}

		return os.WriteFile(path, []byte(minified), info.Mode())
	})
}
