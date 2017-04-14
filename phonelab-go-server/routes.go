package main

import (
	"github.com/gorilla/mux"
	"net/http"
	"time"
)

// API Goo

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

var routes = []Route{
	Route{
		"conf",
		"GET",
		"/conf/{meta_id}/{bean_id}",
		routeConf,
	},
	Route{
		"plugin",
		"GET",
		"/plugin/{meta_id}",
		routePlugin,
	},
	Route{
		"submit",
		"POST",
		"/submit",
		routeSubmit,
	},
}

func NewRouter() *mux.Router {

	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler

		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)

		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}

	return router
}

func Logger(inner http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		inner.ServeHTTP(w, r)

		logger.Printf(
			"%s\t%s\t%s\t%s",
			r.Method,
			r.RequestURI,
			name,
			time.Since(start),
		)
	})
}
