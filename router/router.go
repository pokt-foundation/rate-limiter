package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pokt-foundation/rate-limiter/cache"
	"github.com/pokt-foundation/utils-go/client"
	jsonresponse "github.com/pokt-foundation/utils-go/json-response"
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

func (rt *Router) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("Rate Limiter is up and running!"))
	if err != nil {
		panic(err)
	}
}

func (rt *Router) GetAppIDs(w http.ResponseWriter, r *http.Request) {
	jsonresponse.RespondWithJSON(w, http.StatusOK, rt.Cache.GetAppIDsPassedLimit())
}
