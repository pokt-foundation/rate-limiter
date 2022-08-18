package notification

import (
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/pokt-foundation/pocket-go/mock-client"
	"github.com/pokt-foundation/portal-api-go/repository"
	"github.com/pokt-foundation/rate-limiter/cache"
	"github.com/pokt-foundation/rate-limiter/client"
	"github.com/stretchr/testify/require"
)

func newTestNotifier() *Notifier {
	mock.AddMockedResponseFromFile(http.MethodGet, "https://test-db.com/application/limits",
		http.StatusOK, "../samples/apps_limits.json")
	mock.AddMockedResponseFromFile(http.MethodGet, "https://test-meter.com/v0/relays/apps",
		http.StatusOK, "../samples/apps_relays.json")

	client := client.NewClient(0, 5*time.Second)
	cache := cache.NewCache(client)
	cache.SetCache()

	return NewNotifier(cache)
}

func TestNotifier_getAuth0MgmtToken(t *testing.T) {
	c := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodPost, "https://test-auth0.com/oauth/token",
		httpmock.NewStringResponder(http.StatusOK, `{"access_token": "example-auth0-management-token"}`))

	notifier := newTestNotifier()

	auth0MgmtToken, err := notifier.getAuth0MgmtToken()
	c.NoError(err)

	c.Equal("example-auth0-management-token", auth0MgmtToken)

	httpmock.RegisterResponder(http.MethodPost, "https://test-auth0.com/oauth/token",
		httpmock.NewStringResponder(http.StatusBadRequest, ""))

	_, err = notifier.getAuth0MgmtToken()
	c.Error(err)
}

func TestNotifier_getAuth0Users(t *testing.T) {
	c := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder(http.MethodGet, "https://test-auth0.com/api/v2/users",
		httpmock.NewStringResponder(http.StatusOK, `[{"email": "test@pokt.network"}]`))

	notifier := newTestNotifier()

	auth0UserEmail, err := notifier.getAuth0UserEmail("test-user-id", "example-auth0-management-token")
	c.NoError(err)

	c.Equal("test@pokt.network", auth0UserEmail)

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
	c.Equal(repository.NotificationSettings{
		SignedUp:      true,
		Quarter:       false,
		Half:          true,
		ThreeQuarters: false,
		Full:          true,
	}, testApp.NotificationSettings)
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
