package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pokt-foundation/rate-limiter/router"
	"github.com/pokt-foundation/utils-go/client"
	"github.com/pokt-foundation/utils-go/environment"
)

var (
	retries      = environment.GetInt64("HTTP_RETRIES", 0)
	timeout      = environment.GetInt64("HTTP_TIMEOUT", 5)
	port         = environment.GetString("PORT", "8080")
	cacheRefresh = environment.GetInt64("CACHE_REFRESH", 10)
)

func cacheHandler(router *router.Router) {
	for {
		time.Sleep(time.Duration(cacheRefresh) * time.Minute)

		err := router.Cache.SetCache()
		if err != nil {
			fmt.Printf("Cache refresh failed with error: %s", err.Error())
		}
	}
}

func httpHandler(router *router.Router) {
	http.Handle("/", router.Router)

	log.Printf("Rate Limiter running in port: %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func main() {
	client := client.NewCustomClient(int(retries), time.Duration(timeout)*time.Second)

	router, err := router.NewRouter(client)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	wg.Add(1)

	go httpHandler(router)
	go cacheHandler(router)

	wg.Wait()
}
