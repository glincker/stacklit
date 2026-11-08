package assets

import _ "embed"

//go:embed template.html
var TemplateHTML string

//go:embed lang-icons.js
var LangIconsJS string

//go:embed symbols.js
var SymbolsJS string
