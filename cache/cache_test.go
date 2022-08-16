package cache

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/pokt-foundation/pocket-go/mock-client"
	"github.com/pokt-foundation/rate-limiter/client"
	"github.com/stretchr/testify/require"
)

func TestCache_SetCache(t *testing.T) {
	c := require.New(t)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", httpDBURL, appLimitsEndpoint),
		http.StatusOK, "../samples/apps_limits.json")

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", relayMeterURL, appRelayMeterEndpoint),
		http.StatusOK, "../samples/apps_relays.json")

	client := client.NewClient(0, 5*time.Second)

	cache := NewCache(client)

	err := cache.SetCache()
	c.NoError(err)

	c.Equal([]string{"62c267ea578dfa0039924a6b"}, cache.GetAppIDsPassedLimit())
}

func TestCache_SetCacheFailure(t *testing.T) {
	c := require.New(t)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	client := client.NewClient(0, 5*time.Second)

	cache := NewCache(client)

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", httpDBURL, appLimitsEndpoint),
		http.StatusInternalServerError, "../samples/apps_limits.json")

	err := cache.SetCache()
	c.Equal(errUnexpectedStatusCode, err)

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", httpDBURL, appLimitsEndpoint),
		http.StatusOK, "../samples/apps_limits.json")

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", relayMeterURL, appRelayMeterEndpoint),
		http.StatusInternalServerError, "../samples/apps_relays.json")

	err = cache.SetCache()
	c.Equal(errUnexpectedStatusCode, err)
}
