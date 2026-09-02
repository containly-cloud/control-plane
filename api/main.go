package main

import (
	"net/http"

	"api/pkg/web"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

func main() {
	router := chi.NewMux()
	humachi.New(router, huma.DefaultConfig("Control Plane API", "1.0.0"))

	router.Handle("/*", web.HandleWeb())

	http.ListenAndServe(":8888", router)
}
