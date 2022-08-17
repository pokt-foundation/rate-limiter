package router

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pokt-foundation/rate-limiter/cache"
	"github.com/pokt-foundation/rate-limiter/client"
)

// Router struct handler for router requests
type Router struct {
	Cache  *cache.Cache
	Router *mux.Router
}

func NewRouter(client *client.Client) (*Router, error) {
	cache := cache.NewCache(client)

	err := cache.SetCache()
	if err != nil {
		return nil, err
	}

	rt := &Router{
		Cache:  cache,
		Router: mux.NewRouter(),
	}

	rt.Router.HandleFunc("/", rt.HealthCheck).Methods(http.MethodGet)
	rt.Router.HandleFunc("/v0/app-ids", rt.GetAppIDs).Methods(http.MethodGet)

	return rt, nil
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err := w.Write(response)
	if err != nil {
		panic(err)
	}
}

func (rt *Router) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("Rate Limiter is up and running!"))
	if err != nil {
		panic(err)
	}
}

func (rt *Router) GetAppIDs(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, rt.Cache.GetAppIDsPassedLimit())
}
