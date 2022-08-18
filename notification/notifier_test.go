package notification

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/pokt-foundation/pocket-go/mock-client"
	"github.com/pokt-foundation/portal-api-go/repository"
	"github.com/pokt-foundation/rate-limiter/cache"
	"github.com/pokt-foundation/utils-go/client"
	"github.com/stretchr/testify/require"
)

func newTestNotifier() *Notifier {
	mock.AddMockedResponseFromFile(http.MethodGet, "https://test-db.com/application/limits",
		http.StatusOK, "../samples/apps_limits.json")
	mock.AddMockedResponseFromFile(http.MethodGet, "https://test-meter.com/v0/relays/apps",
		http.StatusOK, "../samples/apps_relays.json")

	client := client.NewDefaultClient()
	cache := cache.NewCache(client)
	cache.SetCache()

	return NewNotifier(cache)
}

func TestNotifier_getAuth0MgmtToken(t *testing.T) {
	c := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	/* Should fetch a valid Auth0 management token */
	httpmock.RegisterResponder(http.MethodPost, "https://test-auth0.com/oauth/token",
		httpmock.NewStringResponder(http.StatusOK, `{"access_token": "example-auth0-management-token"}`))

	notifier := newTestNotifier()

	auth0MgmtToken, err := notifier.getAuth0MgmtToken()
	c.NoError(err)

	c.Equal("example-auth0-management-token", auth0MgmtToken)

	/* Should throw an error if the Auth0 request errors */
	httpmock.RegisterResponder(http.MethodPost, "https://test-auth0.com/oauth/token",
		httpmock.NewStringResponder(http.StatusBadRequest, ""))

	_, err = notifier.getAuth0MgmtToken()
	c.Error(err)
}

func TestNotifier_getAuth0UserEmail(t *testing.T) {
	c := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	/* Should fetch a single Auth0 user's email by their user ID */
	httpmock.RegisterResponder(http.MethodGet, "https://test-auth0.com/api/v2/users",
		httpmock.NewStringResponder(http.StatusOK, `[{"email": "test@pokt.network"}]`))

	notifier := newTestNotifier()

	auth0UserEmail, err := notifier.getAuth0UserEmail("test-user-id", "example-auth0-management-token")
	c.NoError(err)

	c.Equal("test@pokt.network", auth0UserEmail)

	/* Should throw an error if the user can't be found or the Auth0 request errors */
	httpmock.RegisterResponder(http.MethodGet, "https://test-auth0.com/api/v2/users",
		httpmock.NewStringResponder(http.StatusBadRequest, `[]`))

	_, err = notifier.getAuth0UserEmail("test-user-id", "example-auth0-management-token")

	c.Error(err)
}

func TestNotifier_createUsageMap(t *testing.T) {
	c := require.New(t)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	/* Should succesfully create usage map if no errors occur */
	httpmock.RegisterResponder(http.MethodPost, "https://test-auth0.com/oauth/token",
		httpmock.NewStringResponder(http.StatusOK, `{"access_token": "example-auth0-management-token"}`))
	httpmock.RegisterResponder(http.MethodGet, "https://test-auth0.com/api/v2/users",
		httpmock.NewStringResponder(http.StatusOK, `[{"email": "test@pokt.network"}]`))

	notifier := newTestNotifier()

	usageMap, err := notifier.createUsageMap()
	c.NoError(err)

	testApp := usageMap["f4902e71f54290b3fc88022df98fd0423823df6d24af19031a1b319d44e3ed3a"]
	c.Equal("test@pokt.network", testApp.Email)
	c.Equal(1991538, testApp.Usage)
	c.Equal(0, testApp.Limit)
	c.Equal("test@pokt.network", testApp.Email)
	c.Equal(None, testApp.Threshold)
	c.Equal(2, len(usageMap))

	/* Should throw an error if the Auth0 token fetch fails */
	httpmock.RegisterResponder(http.MethodPost, "https://test-auth0.com/oauth/token",
		httpmock.NewStringResponder(http.StatusBadRequest, ""))

	_, err = notifier.createUsageMap()
	c.Error(err)

	/* Should not throw an error if the Auth0 user fetch fails but will not map failed users to the usage map */
	httpmock.RegisterResponder(http.MethodPost, "https://test-auth0.com/oauth/token",
		httpmock.NewStringResponder(http.StatusOK, `{"access_token": "example-auth0-management-token"}`))
	httpmock.RegisterResponder(http.MethodGet, "https://test-auth0.com/api/v2/users",
		httpmock.NewStringResponder(http.StatusBadRequest, `[]`))

	usageMapWithUserFailure, err := notifier.createUsageMap()

	c.NoError(err)
	c.Equal(0, len(usageMapWithUserFailure))
}

func TestNotifier_getAppThreshold(t *testing.T) {
	c := require.New(t)

	testSettings := repository.NotificationSettings{Quarter: true, Half: true, ThreeQuarters: true, Full: true}

	/* Should return "None" for any usage if limit is 0 */
	threshold := getAppThreshold(678932, 0, testSettings)

	c.Equal(None, threshold)

	/* Should return "Quarter" for usage of 25-49% of limit if Quarter set */
	threshold = getAppThreshold(27, 100, testSettings)

	c.Equal(Quarter, threshold)

	/* Should return "Half" for usage of 50-74% of limit if Half set */
	threshold = getAppThreshold(62, 100, testSettings)

	c.Equal(Half, threshold)

	/* Should return "ThreeQuarters" for usage of 75-99% of limit if ThreeQuarters set */
	threshold = getAppThreshold(84, 100, testSettings)

	c.Equal(ThreeQuarters, threshold)

	/* Should return "Full" for usage of 100% or more of limit if Full set */
	threshold = getAppThreshold(100, 100, testSettings)

	c.Equal(Full, threshold)

	/* Should return "Half" for usage of 75-99% of limit if ThreeQuarters not set */
	testSettings.ThreeQuarters = false
	threshold = getAppThreshold(91, 100, testSettings)

	c.Equal(Half, threshold)
}
