package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Psybernetic7/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))

}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	x := cfg.fileserverHits.Load()
	f := fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, x)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(f))
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := cfg.db.DeleteAllUsers(r.Context()); err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (cfg *apiConfig) handlerUserCreate(w http.ResponseWriter, r *http.Request) {
	type userCreateParams struct {
		Email string `json:"email"`
	}
	var params userCreateParams

	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(params.Email)
	if email == "" {
		http.Error(w, "email required", http.StatusBadRequest)
		return
	}

	dbUser, err := cfg.db.CreateUser(r.Context(), email)

	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	resp := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (cfg *apiConfig) handlerChirpsCreate(w http.ResponseWriter, r *http.Request) {
	type req struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}
	var in req
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if len(in.Body) > 140 {
		http.Error(w, "Chirp is too long", http.StatusBadRequest)
		return
	}

	banned := []string{"kerfuffle", "sharbert", "fornax"}
	words := strings.Split(in.Body, " ")
	for i, w := range words {
		if contains(w, banned) {
			words[i] = "****"
		}
	}
	cleaned := strings.Join(words, " ")

	row, err := cfg.db.CreateChirp(
		r.Context(),
		database.CreateChirpParams{Body: cleaned, UserID: in.UserID},
	)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	type chirpResp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}

	resp := chirpResp{
		ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Body: row.Body, UserID: row.UserID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (cfg *apiConfig) handlerChirpsGetAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := cfg.db.GetChirps(ctx)
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	type chirpResp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}

	out := make([]chirpResp, 0, len(rows))
	for _, c := range rows {
		out = append(out, chirpResp{
			ID: c.ID, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
			Body: c.Body, UserID: c.UserID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(out)
}

func (cfg *apiConfig) handlerChirpsGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("chirpID")
	uid, err := uuid.Parse(id)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	chirp, err := cfg.db.GetChirpsByID(ctx, uid)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Chirp not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	type chirpResp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}

	resp := chirpResp{
		ID: chirp.ID, CreatedAt: chirp.CreatedAt, UpdatedAt: chirp.UpdatedAt,
		Body: chirp.Body, UserID: chirp.UserID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func contains(s string, str []string) bool {
	s = strings.ToLower(s)
	for _, cmp := range str {
		if s == cmp {
			return true
		}
	}
	return false
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Cannot open database")
	}

	dbQueries := database.New(db)

	serveMux := http.NewServeMux()
	apiCfg := apiConfig{db: dbQueries, platform: platform}

	fs := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	serveMux.Handle("/app/", apiCfg.middlewareMetricsInc(fs))

	serveMux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	serveMux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	serveMux.HandleFunc("POST /api/chirps", apiCfg.handlerChirpsCreate)
	serveMux.HandleFunc("GET /api/chirps", apiCfg.handlerChirpsGetAll)
	serveMux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handlerChirpsGet)
	serveMux.HandleFunc("POST /api/users", apiCfg.handlerUserCreate)

	serveMux.HandleFunc("GET /api/healthz", ReadinessHandler)

	srv := &http.Server{Addr: ":8080", Handler: serveMux}
	err = srv.ListenAndServe()

	if err != nil {
		log.Fatal(err)
	}

}
