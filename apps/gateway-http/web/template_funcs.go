package web

import "html/template"

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"markdown": markdown,
	}
}
