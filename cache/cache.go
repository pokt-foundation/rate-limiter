package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/pokt-foundation/portal-api-go/repository"
	"github.com/pokt-foundation/rate-limiter/client"
	"github.com/pokt-foundation/rate-limiter/environment"
)

const (
	appLimitsEndpoint     = "/application/limits"
	appRelayMeterEndpoint = "/v0/relays/apps"
)

var (
	// url default values needed for unit testing
	httpDBURL     = environment.GetString("HTTP_DB_URL", "https://test-db.com")
	relayMeterURL = environment.GetString("RELAY_METER_URL", "https://test-meter.com")
	httpDBAPIKey  = environment.GetString("HTTP_DB_API_KEY", "")

	errUnexpectedStatusCodeInLimits = errors.New("unexpected status code in limits")
	errUnexpectedStatusCodeInRelays = errors.New("unexpected status code in relays")

	zeroTimeString = "T00:00:00Z"
)

type Cache struct {
	client            *client.Client
	mutex             sync.Mutex
	AppLimits         map[string]repository.AppLimits
	RelaysCount       []AppRelaysResponse
	appIDsPassedLimit []string
}

func NewCache(client *client.Client) *Cache {
	return &Cache{
		client: client,
	}
}

func (c *Cache) GetAppIDsPassedLimit() []string {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.appIDsPassedLimit
}

func (c *Cache) getAppLimits() (map[string]repository.AppLimits, error) {
	header := http.Header{}

	header.Add("Authorization", httpDBAPIKey)

	response, err := c.client.GetWithURLAndParams(fmt.Sprintf("%s%s", httpDBURL, appLimitsEndpoint), nil, header)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, errUnexpectedStatusCodeInLimits
	}

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	appsLimits := []repository.AppLimits{}

	err = json.Unmarshal(bodyBytes, &appsLimits)
	if err != nil {
		return nil, err
	}

	resp := make(map[string]repository.AppLimits, len(appsLimits))

	for _, appLimit := range appsLimits {
		if appLimit.PublicKey != "" {
			resp[appLimit.PublicKey] = appLimit
		}
	}

	return resp, nil
}

type RelayCounts struct {
	Success int64
	Failure int64
}

type AppRelaysResponse struct {
	Count       RelayCounts
	From        time.Time
	To          time.Time
	Application string
}

func (c *Cache) getRelaysCount() ([]AppRelaysResponse, error) {
	todayZeroDate := fmt.Sprintf("%s%s", time.Now().Format("2006-01-02"), zeroTimeString)

	params := url.Values{}

	params.Set("from", todayZeroDate)
	params.Set("to", todayZeroDate)

	response, err := c.client.GetWithURLAndParams(fmt.Sprintf("%s%s", relayMeterURL, appRelayMeterEndpoint), params, http.Header{})
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, errUnexpectedStatusCodeInRelays
	}

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	appsRelays := []AppRelaysResponse{}

	err = json.Unmarshal(bodyBytes, &appsRelays)
	if err != nil {
		return nil, err
	}

	return appsRelays, nil
}

func (c *Cache) SetCache() error {
	appLimits, err := c.getAppLimits()
	if err != nil {
		return err
	}

	relaysCount, err := c.getRelaysCount()
	if err != nil {
		return err
	}

	var appIDsPassedLimit []string

	for _, relayCount := range relaysCount {
		appLimit := appLimits[relayCount.Application]

		if appLimit.DailyLimit == 0 {
			continue
		}

		if relayCount.Count.Failure+relayCount.Count.Success >= int64(appLimit.DailyLimit) {
			appIDsPassedLimit = append(appIDsPassedLimit, appLimit.AppID)
		}
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.AppLimits = appLimits
	c.RelaysCount = relaysCount
	c.appIDsPassedLimit = appIDsPassedLimit

	return nil
}
