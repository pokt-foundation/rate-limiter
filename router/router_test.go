package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/pokt-foundation/pocket-go/mock-client"
	"github.com/pokt-foundation/rate-limiter/client"
	"github.com/stretchr/testify/require"
)

func newTestRouter() (*Router, error) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mock.AddMockedResponseFromFile(http.MethodGet, "https://test-db.com/application/limits",
		http.StatusOK, "../samples/apps_limits.json")

	mock.AddMockedResponseFromFile(http.MethodGet, "https://test-meter.com/v0/relays/apps",
		http.StatusOK, "../samples/apps_relays.json")

	return NewRouter(client.NewClient(0, 5*time.Second))
}

func TestRouter_HealthCheck(t *testing.T) {
	c := require.New(t)

	req, err := http.NewRequest(http.MethodGet, "/", nil)
	c.NoError(err)

	rr := httptest.NewRecorder()

	router, err := newTestRouter()
	c.NoError(err)

	router.Router.ServeHTTP(rr, req)

	c.Equal(http.StatusOK, rr.Code)
}

func TestRouter_GetAppIDs(t *testing.T) {
	c := require.New(t)

	req, err := http.NewRequest(http.MethodGet, "/v0/app-ids", nil)
	c.NoError(err)

	rr := httptest.NewRecorder()

	router, err := newTestRouter()
	c.NoError(err)

	router.Router.ServeHTTP(rr, req)

	c.Equal(http.StatusOK, rr.Code)

	expectedBody, err := json.Marshal([]string{"62c267ea578dfa0039924a6b"})
	c.NoError(err)

	c.Equal(expectedBody, rr.Body.Bytes())
}
