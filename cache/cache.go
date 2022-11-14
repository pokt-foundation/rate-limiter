package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	appsEndpoint               = "/application"
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

func (c *Cache) getAppLimits() (map[string]repository.Application, error) {
	header := http.Header{}

	header.Add("Authorization", httpDBAPIKey)

	response, err := c.client.GetWithURLAndParams(fmt.Sprintf("%s%s", httpDBURL, appsEndpoint), nil, header)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, errUnexpectedStatusCodeInLimits
	}

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	applications := []repository.Application{}

	err = json.Unmarshal(bodyBytes, &applications)
	if err != nil {
		return nil, err
	}

	resp := make(map[string]repository.Application, len(applications))

	for _, app := range applications {
		if app.GatewayAAT.ApplicationPublicKey != "" {
			resp[app.GatewayAAT.ApplicationPublicKey] = app
		}
	}

	return resp, nil
}

func (c *Cache) setFirstDateSurpassed(appIDs []string) error {
	if len(appIDs) > 0 {
		header := http.Header{}

		header.Add("Authorization", httpDBAPIKey)

		response, err := c.client.PostWithURLJSONParams(
			fmt.Sprintf("%s%s", httpDBURL, firstDateSurpassedEndpoint),
			&repository.UpdateFirstDateSurpassed{
				FirstDateSurpassed: time.Now(),
				ApplicationIDs:     appIDs,
			},
			header,
		)
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

	bodyBytes, err := io.ReadAll(response.Body)
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

func passedLimit(count, dailyLimit int, firstSurpassedDate time.Time) bool {
	return count >= dailyLimit && !firstSurpassedDate.IsZero() && time.Since(firstSurpassedDate) >= gracePeriod
}

func passedLimitForTheFirstTime(count, dailyLimit int, firstSurpassedDate time.Time) bool {
	return count >= dailyLimit && firstSurpassedDate.IsZero()
}

func (c *Cache) SetCache() error {
	applications, err := c.getAppLimits()
	if err != nil {
		return fmt.Errorf("err in getAppLimits: %w", err)
	}

	relaysCount, err := c.getRelaysCount()
	if err != nil {
		return fmt.Errorf("err in getRelaysCount: %w", err)
	}

	var appIDsPassedLimit []string
	var appIDsToAddFirstSurpassedDate []string

	for _, relayCount := range relaysCount {
		app := applications[relayCount.Application]

		if app.DailyLimit() == 0 {
			continue
		}

		count := int(relayCount.Count.Success)

		fields := logrus.Fields{
			"daily_app_limit":      app.DailyLimit(),
			"app_id":               app.ID,
			"count":                count,
			"first_date_surpassed": app.FirstDateSurpassed,
		}

		if passedLimit(count, app.DailyLimit(), app.FirstDateSurpassed) {
			log.WithFields(fields).Info(fmt.Sprintf("app: %s passed daily limit with %d of %d", app.ID, count, app.DailyLimit()))
			appIDsPassedLimit = append(appIDsPassedLimit, app.ID)
		}

		if passedLimitForTheFirstTime(count, app.DailyLimit(), app.FirstDateSurpassed) {
			fields["first_date_surpassed"] = time.Now()
			log.WithFields(fields).Info(fmt.Sprintf("app: %s passed first daily limit at: %s", app.ID, time.Now().Format("2006-01-02T15:04:05")))

			appIDsToAddFirstSurpassedDate = append(appIDsToAddFirstSurpassedDate, app.ID)
		}

	}

	err = c.setFirstDateSurpassed(appIDsToAddFirstSurpassedDate)
	if err != nil {
		return fmt.Errorf("err in setFirstDateSurpassed: %w", err)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.appIDsPassedLimit = appIDsPassedLimit

	return nil
}
