package cache

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/pokt-foundation/utils-go/client"
	"github.com/pokt-foundation/utils-go/mock-client"
	"github.com/stretchr/testify/require"
)

func TestCache_SetCache(t *testing.T) {
	c := require.New(t)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", httpDBURL, appsEndpoint),
		http.StatusOK, "../testdata/apps.json")

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", relayMeterURL, appRelayMeterEndpoint),
		http.StatusOK, "../testdata/apps_relays.json")

	mock.AddMockedResponse(http.MethodPost, fmt.Sprintf("%s%s", httpDBURL, firstDateSurpassedEndpoint),
		http.StatusOK, "ok")

	client := client.NewCustomClient(0, 5*time.Second)

	cache := NewCache(client)

	err := cache.SetCache()
	c.NoError(err)

	c.Equal([]string{"test_id_803b3f0e5430b17d"}, cache.GetAppIDsPassedLimit())
}

func TestCache_SetCacheFailure(t *testing.T) {
	c := require.New(t)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	client := client.NewCustomClient(0, 5*time.Second)

	cache := NewCache(client)

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", httpDBURL, appsEndpoint),
		http.StatusInternalServerError, "../testdata/apps.json")

	err := cache.SetCache()
	c.ErrorIs(err, errUnexpectedStatusCodeInLimits)

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", httpDBURL, appsEndpoint),
		http.StatusOK, "../testdata/apps.json")

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", relayMeterURL, appRelayMeterEndpoint),
		http.StatusInternalServerError, "../testdata/apps_relays.json")

	err = cache.SetCache()
	c.ErrorIs(err, errUnexpectedStatusCodeInRelays)

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", httpDBURL, appsEndpoint),
		http.StatusOK, "../testdata/apps.json")

	mock.AddMockedResponseFromFile(http.MethodGet, fmt.Sprintf("%s%s", relayMeterURL, appRelayMeterEndpoint),
		http.StatusOK, "../testdata/apps_relays.json")

	mock.AddMockedResponse(http.MethodPost, fmt.Sprintf("%s%s", httpDBURL, firstDateSurpassedEndpoint),
		http.StatusInternalServerError, "not ok")

	err = cache.SetCache()
	c.ErrorIs(err, errUnexpectedStatusCodeInDateSurpassed)
}
