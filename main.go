package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/BradDeA/chirpy.git/internal/auth"
	"github.com/BradDeA/chirpy.git/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	Db             *database.Queries
	SecretKey      string
}

type RequestParams struct {
	Body    string    `json:"body"`
	User_id uuid.UUID `json:"user_id"`
}

type UserValues struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Token     string    `json:"token"`
}

type ChirpRes struct {
	Id         uuid.UUID `json:"id"`
	Created_at time.Time `json:"created_at"`
	Updated_at time.Time `json:"updated_at"`
	Body       string    `json:"body"`
	User_id    uuid.UUID `json:"user_id"`
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
	dbPlatform := os.Getenv("PLATFORM")
	if dbPlatform != "dev" {
		w.WriteHeader(403)
		return
	}

	cfg.fileserverHits.Store(0)
	cfg.Db.DeleteUsers(context.Background())
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hits reset to 0"))
}

func main() {

	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	ServMux := http.NewServeMux()
	server := http.Server{Addr: ":8080", Handler: ServMux}
	secretString := os.Getenv("SECRET")

	db, dberr := sql.Open("postgres", dbURL)
	if dberr != nil {
		log.Println(dberr)
	}

	dbQueries := database.New(db)
	apiCfg := apiConfig{Db: dbQueries, SecretKey: secretString}

	ServMux.Handle("GET /app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	ServMux.Handle("GET /assets", apiCfg.middlewareMetricsInc(http.FileServer(http.Dir("."))))
	ServMux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	ServMux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)

	ServMux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type:", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))

	})

	ServMux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, r *http.Request) {

		profane := []string{"kerfuffle", "sharbert", "fornax"}
		decoder := json.NewDecoder(r.Body)
		jsonParams := RequestParams{}
		err := decoder.Decode(&jsonParams)
		if err != nil {
			w.WriteHeader(500)
			return
		}

		token, tokenErr := auth.GetBearerToken(r.Header)
		if tokenErr != nil {
			w.WriteHeader(401)
			return
		}

		tokenValid, tokenErr := auth.ValidateJWT(token, secretString)
		if tokenErr != nil {
			w.WriteHeader(401)
			return
		}

		if len(jsonParams.Body) > 140 {
			values := ChirpRes{Body: "Chirp is too long"}
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

		chirp, createErr := apiCfg.Db.CreateChirp(context.Background(), database.CreateChirpParams{Body: joined, UserID: tokenValid})
		if createErr != nil {
			w.WriteHeader(500)
			w.Write([]byte(createErr.Error()))
			return
		}
		res := ChirpRes{
			Id:         chirp.ID,
			Created_at: chirp.CreatedAt,
			Updated_at: chirp.UpdatedAt,
			Body:       chirp.Body,
			User_id:    tokenValid,
		}
		marshal, err := json.Marshal(res)
		if err != nil {
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(marshal)
	})

	ServMux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {

		type JsonBody struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		decoder := json.NewDecoder(r.Body)
		params := JsonBody{}
		err := decoder.Decode(&params)
		if err != nil {
			w.WriteHeader(500)
			fmt.Print(err)
			return
		}
		hash, hashErr := auth.HashPassword(params.Password)
		if hashErr != nil {
			w.WriteHeader(500)
			return
		}
		user, userErr := apiCfg.Db.CreateUser(context.Background(), database.CreateUserParams{Email: params.Email, HashedPassword: hash})
		if userErr != nil {
			w.WriteHeader(500)
			return
		}

		marshalValues := UserValues{Id: user.ID, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt, Email: user.Email}
		returnData, marshalErr := json.Marshal(marshalValues)
		if marshalErr != nil {
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write(returnData)

	})

	ServMux.HandleFunc("GET /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		allChirps, err := apiCfg.Db.GetChirps(context.Background())
		if err != nil {
			w.WriteHeader(500)
			return
		}
		chirpStructs := []ChirpRes{}

		for _, chirp := range allChirps {
			chirpStructs = append(chirpStructs, ChirpRes{Id: chirp.ID, Created_at: chirp.CreatedAt, Updated_at: chirp.UpdatedAt, Body: chirp.Body, User_id: chirp.UserID})
		}

		chirps, chirpErr := json.Marshal(chirpStructs)
		if chirpErr != nil {
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(chirps)
	})

	ServMux.HandleFunc("GET /api/chirps/{chripID}", func(w http.ResponseWriter, r *http.Request) {
		allChirps, err := apiCfg.Db.GetChirps(context.Background())
		if err != nil {
			w.WriteHeader(500)
			return
		}
		chirpStructs := ChirpRes{}

		for _, chirp := range allChirps {
			if r.PathValue("chripID") == chirp.ID.String() {
				chirpStructs = ChirpRes{Id: chirp.ID, Created_at: chirp.CreatedAt, Updated_at: chirp.UpdatedAt, Body: chirp.Body, User_id: chirp.UserID}
			} else {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(404)
			}
		}

		chirps, chirpErr := json.Marshal(chirpStructs)
		if chirpErr != nil {
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(chirps)
	})

	ServMux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		type ValidReq struct {
			Email     string `json:"email"`
			Password  string `json:"password"`
			ExpiresIn int    `json:"expires_in_seconds,omitempty"`
		}

		decoder := json.NewDecoder(r.Body)
		params := ValidReq{}
		err := decoder.Decode(&params)
		if err != nil {
			w.WriteHeader(500)
			return
		}

		found, err := apiCfg.Db.EmailLookup(context.Background(), params.Email)
		if err != nil {
			w.WriteHeader(401)
			fmt.Println("Invalid username or password")
			return
		}

		check := auth.CheckPasswordHash(params.Password, found.HashedPassword)
		if check != nil {
			w.WriteHeader(401)
			fmt.Println("Invalid username or password")
			return
		}

		if params.ExpiresIn == 0 || params.ExpiresIn > 3600 {
			params.ExpiresIn = 3600
		}

		token, tokenErr := auth.MakeJWT(found.ID, secretString, time.Duration(params.ExpiresIn)*time.Second)
		if tokenErr != nil {
			w.WriteHeader(500)
			fmt.Println("makejwt error")
			return
		}

		marshalValues := UserValues{Id: found.ID, CreatedAt: found.CreatedAt, UpdatedAt: found.UpdatedAt, Email: found.Email, Token: token}
		returnData, marshalErr := json.Marshal(marshalValues)
		if marshalErr != nil {
			w.WriteHeader(500)
			fmt.Println("marshal error")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(returnData)
	})

	err := server.ListenAndServe()
	if err != nil {
		fmt.Print(err)
	}

}
