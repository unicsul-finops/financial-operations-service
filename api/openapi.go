package api

import _ "embed"

// o spec vai embutido no binário, assim o swagger funciona de qualquer pasta em que a api for executada.
//
//go:embed openapi.yaml
var OpenAPI []byte
