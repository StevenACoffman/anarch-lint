package web

import "net/http"

// Routes serves the list of invoices, which audit requests over HTTP: a
// contract between the two, with no import between them.
func Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /invoices", func(http.ResponseWriter, *http.Request) {})
}
