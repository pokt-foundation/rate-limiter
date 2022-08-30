package router

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pokt-foundation/rate-limiter/cache"
	"github.com/pokt-foundation/utils-go/client"
	"github.com/pokt-foundation/utils-go/environment"
	jsonresponse "github.com/pokt-foundation/utils-go/json-response"
)

var (
	apiKeys = environment.GetStringMap("API_KEYS", "", ",")
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

	rt.Router.Use(rt.AuthorizationHandler)

	return rt, nil
}

func (rt *Router) AuthorizationHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This is the path of the health check endpoint
		if r.URL.Path == "/" {
			h.ServeHTTP(w, r)

			return
		}

		if !apiKeys[r.Header.Get("Authorization")] {
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte("Unauthorized"))
			if err != nil {
				panic(err)
			}

			return
		}

		h.ServeHTTP(w, r)
	})
}

func (rt *Router) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("Rate Limiter is up and running!"))
	if err != nil {
		panic(err)
	}
}

type getAppIDsOutput struct {
	ApplicationIDs []string `json:"applicationIDs"`
}

func (rt *Router) GetAppIDs(w http.ResponseWriter, r *http.Request) {
	jsonresponse.RespondWithJSON(w, http.StatusOK, getAppIDsOutput{
		ApplicationIDs: rt.Cache.GetAppIDsPassedLimit(),
	})
}
