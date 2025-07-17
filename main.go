package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

	ServMux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
		profane := []string{"kerfuffle", "sharbert", "fornax"}

		type RequestParams struct {
			Body string `json:"body"`
		}

		type ReturnValues struct {
			Error        string `json:"error"`
			Cleaned_body string `json:"cleaned_body"`
		}

		decoder := json.NewDecoder(r.Body)
		jsonParams := RequestParams{}
		err := decoder.Decode(&jsonParams)
		if err != nil {
			w.WriteHeader(500)
			return
		}

		if len(jsonParams.Body) > 140 {
			values := ReturnValues{Error: "Chirp is too long"}
			data, err := json.Marshal(values)
			if err != nil {
				w.WriteHeader(500)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			w.Write(data)
			return
		}
		words := strings.Split(jsonParams.Body, " ")

		for i, word := range words {
			lowerWord := strings.ToLower(word)
			for _, profaneWord := range profane {
				if lowerWord == profaneWord {
					words[i] = "****"
				}
			}
		}
		joined := strings.Join(words, " ")
		successValue := ReturnValues{Error: "", Cleaned_body: joined}
		successData, err := json.Marshal(successValue)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(successData)
	})

	err := server.ListenAndServe()
	if err != nil {
		fmt.Print(err)
	}

}
