package notification

import (
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/pokt-foundation/rate-limiter/cache"
	"github.com/pokt-foundation/rate-limiter/client"
	"github.com/stretchr/testify/require"
)

func newTestNotifier() *Notifier {
	client := client.NewClient(0, 5*time.Second)
	cache := cache.NewCache(client)
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
