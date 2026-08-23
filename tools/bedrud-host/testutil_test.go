package main

import (
	"net/http"
	"net/http/httptest"
)

func httptestServer(h http.Handler) *httptest.Server {
	return httptest.NewServer(h)
}
