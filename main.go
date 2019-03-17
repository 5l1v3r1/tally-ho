package main

import (
	"flag"
	"log"

	"hawx.me/code/mux"
	"hawx.me/code/route"
	"hawx.me/code/serve"
	"hawx.me/code/tally-ho/config"
	"hawx.me/code/tally-ho/data"
	"hawx.me/code/tally-ho/handler"
)

func main() {
	var (
		port    = flag.String("port", "8080", "")
		socket  = flag.String("socket", "", "")
		me      = flag.String("me", "", "")
		dbPath  = flag.String("db", "file::memory:", "")
		baseURL = flag.String("base-url", "http://localhost:8080/", "")
	)
	flag.Parse()

	if *me == "" {
		log.Fatal("--me must be provided")
	}

	store, err := data.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	config, err := config.New(*baseURL, "p")
	if err != nil {
		log.Fatal(err)
	}

	route.Handle("/micropub", handler.Authenticate(*me, "create", mux.Method{
		"POST": handler.Post(store, config),
		"GET":  handler.Configuration(store, config),
	}))

	route.Handle("/webmention", mux.Method{
		// "POST":
	})

	serve.Serve(*port, *socket, route.Default)
}
