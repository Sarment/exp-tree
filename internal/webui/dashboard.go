package webui

import _ "embed"

//go:embed assets/index.html
var IndexHTML string

//go:embed assets/styles.css
var StylesCSS string

//go:embed assets/app.js
var AppJS string
