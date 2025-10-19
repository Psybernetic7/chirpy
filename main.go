package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

func ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))

}

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
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

func ValidateChirp(w http.ResponseWriter, r *http.Request) {
	type httpBody struct {
		Body string `json:"body"`
	}

	type httpError struct {
		Error string `json:"error"`
	}

	type httpRes struct {
		Valid bool `json:"valid"`
	}

	decoder := json.NewDecoder(r.Body)
	reqBdy := httpBody{}
	err := decoder.Decode(&reqBdy)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		dat, _ := json.Marshal(httpError{Error: "Something went wrong"})
		w.Write(dat)
		return
	}

	if len(reqBdy.Body) > 140 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		dat, _ := json.Marshal(httpError{Error: "Chirp is too long"})
		w.Write(dat)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	dat, _ := json.Marshal(httpRes{Valid: true})
	w.Write(dat)

}

func main() {
	serveMux := http.NewServeMux()
	apiCfg := apiConfig{}

	fs := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	serveMux.Handle("/app/", apiCfg.middlewareMetricsInc(fs))

	serveMux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	serveMux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	serveMux.HandleFunc("POST /api/validate_chirp", ValidateChirp)

	serveMux.HandleFunc("GET /api/healthz", ReadinessHandler)

	srv := &http.Server{Addr: ":8080", Handler: serveMux}
	err := srv.ListenAndServe()

	if err != nil {
		log.Fatal(err)
	}

}
