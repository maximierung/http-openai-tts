package api

import (
	"net/http"
)

func Ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("{\"status\": \"OK\",\"message\": \"Pong!\"}"))
}
