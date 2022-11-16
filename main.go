package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/pokt-foundation/rate-limiter/router"
	"github.com/pokt-foundation/utils-go/client"
	"github.com/pokt-foundation/utils-go/environment"
	"github.com/sirupsen/logrus"
)

const (
	cacheRefresh = "CACHE_REFRESH"
	httpTimeout  = "HTTP_TIMEOUT"
	httpRetries  = "HTTP_RETRIES"
	port         = "PORT"

	defaultCacheRefreshMinutes = 10
	defaultHTTPTimeoutSeconds  = 5
	defaultHTTPRetries         = 0
	defaultPort                = "8080"
)

type options struct {
	cacheRefresh, timeout time.Duration
	retries               int
	port                  string
}

func gatherOptions() options {
	return options{
		cacheRefresh: time.Duration(environment.GetInt64(cacheRefresh, defaultCacheRefreshMinutes)) * time.Minute,
		timeout:      time.Duration(environment.GetInt64(httpTimeout, defaultHTTPTimeoutSeconds)) * time.Second,
		retries:      int(environment.GetInt64(httpRetries, defaultHTTPRetries)),
		port:         environment.GetString(port, defaultPort),
	}
}

func cacheHandler(router *router.Router, cacheRefresh time.Duration, log *logrus.Logger) {
	for {
		time.Sleep(time.Duration(cacheRefresh) * time.Minute)

		err := router.Cache.SetCache()
		if err != nil {
			log.Error(fmt.Sprintf("Cache refresh failed with error: %s", err.Error()))
		}
	}
}

func httpHandler(router *router.Router, port string, log *logrus.Logger) {
	http.Handle("/", router.Router)

	log.Printf("Rate Limiter running in port: %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func main() {
	log := logrus.New()
	// log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&logrus.JSONFormatter{})

	options := gatherOptions()

	client := client.NewCustomClient(options.retries, options.timeout)

	router, err := router.NewRouter(client)
	if err != nil {
		log.Error(fmt.Sprintf("Create router failed with error: %s", err.Error()))

		panic(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go httpHandler(router, options.port, log)
	go cacheHandler(router, options.cacheRefresh, log)

	wg.Wait()
}
