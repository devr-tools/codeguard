package fixtures

import "net/http"

const buildTag = "v10.0.0.0"

func serveLocal(mux *http.ServeMux) error {
	return http.ListenAndServe("127.0.0.1:8080", mux)
}
