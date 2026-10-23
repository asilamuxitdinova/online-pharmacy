// Package openapi embeds the API specification. It is Frege's tool list, so
// it is written by hand with care rather than generated.
package openapi

import _ "embed"

//go:embed openapi.yaml
var Spec string

// ServerPlaceholder is replaced by PUBLIC_URL when the spec is served.
const ServerPlaceholder = "https://pharmacy.example.test"
