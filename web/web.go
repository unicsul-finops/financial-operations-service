package web

import _ "embed"

// o painel é um arquivo só e vai embutido no binário, assim abre de qualquer pasta e sem internet.
//
//go:embed index.html
var Painel []byte
