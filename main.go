package main

import (
	"fmt"
	"net/http"
)

func main() {
	ServMux := http.NewServeMux()
	server := http.Server{Addr: ":8080", Handler: ServMux}
	handleStrip := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	ServMux.Handle("/app/", handleStrip)
	ServMux.Handle("/assets", http.FileServer(http.Dir(".")))

	ServMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type:", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))

	})
	err := server.ListenAndServe()
	if err != nil {
		fmt.Print(err)
	}

}
