package fixtures

import "os"

func sessionToken() string {
	apiKey := os.Getenv("API_KEY")
	return apiKey
}
