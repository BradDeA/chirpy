package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	hitCount := cfg.fileserverHits.Load()
	htmlTemplate := `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`
	render := fmt.Sprintf(htmlTemplate, hitCount)
	w.Write([]byte(render))
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hits reset to 0"))
}

func main() {

	ServMux := http.NewServeMux()
	server := http.Server{Addr: ":8080", Handler: ServMux}
	apiCfg := apiConfig{}

	ServMux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	ServMux.Handle("/assets", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir("."))))
	ServMux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	ServMux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)

	ServMux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type:", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))

	})

	err := server.ListenAndServe()
	if err != nil {
		fmt.Print(err)
	}

}
