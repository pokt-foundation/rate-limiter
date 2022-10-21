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
	"github.com/pokt-foundation/utils-go/client"
	"github.com/pokt-foundation/utils-go/environment"
	"github.com/sirupsen/logrus"
)

const (
	appLimitsEndpoint          = "/application/limits"
	firstDateSurpassedEndpoint = "/application/first_date_surpassed"
	appRelayMeterEndpoint      = "/v0/relays/apps"
)

var (
	// url default values needed for unit testing
	httpDBURL     = environment.GetString("HTTP_DB_URL", "https://test-db.com")
	relayMeterURL = environment.GetString("RELAY_METER_URL", "https://test-meter.com")
	httpDBAPIKey  = environment.GetString("HTTP_DB_API_KEY", "")
	gracePeriod   = time.Duration(environment.GetInt64("GRACE_PERIOD", 48)) * time.Hour

	errUnexpectedStatusCodeInLimits        = errors.New("unexpected status code in limits")
	errUnexpectedStatusCodeInRelays        = errors.New("unexpected status code in relays")
	errUnexpectedStatusCodeInDateSurpassed = errors.New("unexpected status code in first date surpassed")

	zeroTimeString = "T00:00:00Z"

	log = logrus.New()
)

type Cache struct {
	client            *client.Client
	mutex             sync.Mutex
	appIDsPassedLimit []string
}

func init() {
	// log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&logrus.JSONFormatter{})
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

func (c *Cache) setFirstDateSurpassed(appIDs []string) error {
	if len(appIDs) > 0 {
		header := http.Header{}

		header.Add("Authorization", httpDBAPIKey)

		response, err := c.client.PostWithURLJSONParams(fmt.Sprintf("%s%s", httpDBURL, firstDateSurpassedEndpoint),
			&repository.UpdateFirstDateSurpassed{
				FirstDateSurpassed: time.Now(),
				ApplicationIDs:     appIDs,
			}, header)
		if err != nil {
			return err
		}

		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			return errUnexpectedStatusCodeInDateSurpassed
		}
	}

	return nil
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

func passedLimit(count, dailyLimit int, firstSurpassedDate *time.Time) bool {
	return count >= dailyLimit && firstSurpassedDate != nil && time.Since(*firstSurpassedDate) >= gracePeriod
}

func passedLimitForTheFirstTime(count, dailyLimit int, firstSurpassedDate *time.Time) bool {
	return count >= dailyLimit && firstSurpassedDate == nil
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
	var appIDsToAddFirstSurpassedDate []string

	for _, relayCount := range relaysCount {
		appLimit := appLimits[relayCount.Application]

		if appLimit.DailyLimit == 0 {
			continue
		}

		count := int(relayCount.Count.Success)

		fields := logrus.Fields{
			"daily_app_limit":      appLimit.DailyLimit,
			"app_id":               appLimit.AppID,
			"count":                count,
			"first_date_surpassed": appLimit.FirstDateSurpassed,
		}

		if passedLimit(count, appLimit.DailyLimit, appLimit.FirstDateSurpassed) {
			log.WithFields(fields).Info(fmt.Sprintf("app: %s passed his daily limit with %d of %d", appLimit.AppID, count, appLimit.DailyLimit))
			appIDsPassedLimit = append(appIDsPassedLimit, appLimit.AppID)
		}

		if passedLimitForTheFirstTime(count, appLimit.DailyLimit, appLimit.FirstDateSurpassed) {
			fields["first_date_surpassed"] = time.Now()
			log.WithFields(fields).Info(fmt.Sprintf("app: %s passed his first daily limit at: %s", appLimit.AppID, time.Now().Format("2006-01-02T15:04:05")))

			appIDsToAddFirstSurpassedDate = append(appIDsToAddFirstSurpassedDate, appLimit.AppID)
		}
	}

	err = c.setFirstDateSurpassed(appIDsToAddFirstSurpassedDate)
	if err != nil {
		return err
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.appIDsPassedLimit = appIDsPassedLimit

	return nil
}
