package main

import (
	"log"
	"net/http"
)

func main() {
	fileServer := http.FileServer(http.Dir("./static"))
	http.Handle("/", fileServer)

	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		log.Fatalf("Server error: %s", err)
	}
}
