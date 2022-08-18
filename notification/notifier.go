package notification

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/pokt-foundation/portal-api-go/repository"
	"github.com/pokt-foundation/rate-limiter/cache"
	"github.com/pokt-foundation/utils-go/environment"
)

const (
	auth0UsersEndpoint    = "api/v2/users"
	auth0TokenEndpoint    = "oauth/token"
	auth0AudienceEndpoint = "api/v2/"

	Full          NotificationThreshold = "Full"
	ThreeQuarters NotificationThreshold = "ThreeQuarters"
	Half          NotificationThreshold = "Half"
	Quarter       NotificationThreshold = "Quarter"
	None          NotificationThreshold = "None"
)

var (
	// url default values needed for unit testing
	auth0Domain       = environment.GetString("AUTH0_DOMAIN", "https://test-auth0.com")
	auth0ClientId     = environment.GetString("AUTH0_CLIENT_ID", "")
	auth0ClientSecret = environment.GetString("AUTH0_CLIENT_SECRET", "")

	errAuth0TokenResponse = errors.New("error fetching management token from Auth0")
	errAuth0UserResponse  = errors.New("error fetching user from Auth0")
	errAuth0UserNotFound  = errors.New("user not found in Auth0")
)

type (
	Notifier struct {
		cache *cache.Cache
	}

	AppUsage struct {
		Email     string
		Name      string
		Limit     int
		Usage     int
		Threshold NotificationThreshold
	}

	Auth0Token struct {
		Token string `json:"access_token,omitempty"`
	}
	Auth0User struct {
		Email string `json:"email,omitempty"`
	}

	NotificationThreshold string
)

func NewNotifier(cache *cache.Cache) *Notifier {
	return &Notifier{
		cache: cache,
	}
}

func (n *Notifier) getAuth0MgmtToken() (string, error) {
	auth0Url := fmt.Sprintf("%s/%s", auth0Domain, auth0TokenEndpoint)
	reqBody := fmt.Sprintf(
		"grant_type=client_credentials&client_id=%s&client_secret=%s&audience=%s/%s",
		auth0ClientId, auth0ClientSecret, auth0Domain, auth0AudienceEndpoint,
	)
	headers := http.Header{"content-type": {"application/x-www-form-urlencoded"}}

	response, err := n.cache.Client.PostWithURLJSONParams(auth0Url, reqBody, headers)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", errAuth0TokenResponse
	}

	resBody, _ := ioutil.ReadAll(response.Body)

	var tokenResponse Auth0Token
	err = json.Unmarshal(resBody, &tokenResponse)
	if err != nil {
		return "", err
	}

	return tokenResponse.Token, nil
}

func (n *Notifier) getAuth0UserEmail(userID string, token string) (string, error) {
	auth0Url := fmt.Sprintf("%s/%s", auth0Domain, auth0UsersEndpoint)
	params := url.Values{
		"q":              {fmt.Sprintf("user_id:*%s", userID)},
		"fields":         {"user_id,email"},
		"include_fields": {"true"},
	}
	headers := http.Header{"Authorization": {fmt.Sprintf("Bearer %s", token)}}

	response, err := n.cache.Client.GetWithURLAndParams(auth0Url, params, headers)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return "", errAuth0UserResponse
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	auth0Users := []Auth0User{}
	err = json.Unmarshal(body, &auth0Users)
	if err != nil {
		return "", err
	}

	if len(auth0Users) > 0 && auth0Users[0].Email != "" {
		userEmail := auth0Users[0].Email
		return userEmail, nil
	} else {
		return "", errAuth0UserNotFound
	}
}

func (n *Notifier) createUsageMap() (map[string]AppUsage, error) {
	appLimits, relaysCount := n.cache.AppLimits, n.cache.RelaysCount

	usageMap := make(map[string]AppUsage, len(relaysCount))

	auth0Token, err := n.getAuth0MgmtToken()
	if err != nil {
		return nil, err
	}

	for _, app := range relaysCount {
		appPubKey := app.Application
		appUsage := int(app.Count.Failure + app.Count.Success)
		appDetails := appLimits[appPubKey]

		auth0UserEmail, err := n.getAuth0UserEmail(appDetails.AppUserID, auth0Token)
		if err != nil {
			// TODO Log error getting user email
			continue
		}

		threshold := getAppThreshold(appUsage, appDetails.DailyLimit, *appDetails.NotificationSettings)

		usageMap[appPubKey] = AppUsage{
			Usage:     appUsage,
			Limit:     appDetails.DailyLimit,
			Email:     auth0UserEmail,
			Name:      appDetails.AppName,
			Threshold: threshold,
		}
	}

	return usageMap, nil
}

func getAppThreshold(usage int, limit int, notificationSettings repository.NotificationSettings) NotificationThreshold {
	usageFloat, limitFloat := float64(usage), float64(limit)

	if limit == 0 {
		return None
	}
	if notificationSettings.Full && usageFloat >= limitFloat {
		return Full
	}
	if notificationSettings.ThreeQuarters && usageFloat >= limitFloat*0.75 {
		return ThreeQuarters
	}
	if notificationSettings.Half && usageFloat >= limitFloat*0.5 {
		return Half
	}
	if notificationSettings.Quarter && usageFloat >= limitFloat*0.25 {
		return Quarter
	}

	return None
}
