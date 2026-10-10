package api

import (
	"encoding/json"
	"net/http"
)

type Request struct {
	Name string `json:"name"`
}

func Handle(w http.ResponseWriter, r *http.Request) {
	// decode the request
	var req Request
	json.NewDecoder(r.Body).Decode(&req)

	// set the status code
	w.WriteHeader(http.StatusOK)

	// write the body
	w.Write([]byte("ok"))
}
