package fixtures

import "net/http"

func allowAllOrigins(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}
