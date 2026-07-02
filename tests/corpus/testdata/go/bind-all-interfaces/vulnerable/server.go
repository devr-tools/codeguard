package fixtures

import "net/http"

func serve(mux *http.ServeMux) error {
	return http.ListenAndServe("0.0.0.0:8080", mux)
}
