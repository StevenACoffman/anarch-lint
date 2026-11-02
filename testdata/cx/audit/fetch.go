package audit

import "net/http"

// Invoices asks web for the invoices, over the route web serves.
func Invoices() (*http.Response, error) { return http.Get("http://web.internal/invoices") }
