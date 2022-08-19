package notification

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"

	"github.com/pokt-foundation/portal-api-go/repository"
	"github.com/pokt-foundation/rate-limiter/cache"
	"github.com/pokt-foundation/utils-go/environment"
)

const (
	auth0UsersEndpoint    = "api/v2/users"
	auth0TokenEndpoint    = "oauth/token"
	auth0AudienceEndpoint = "api/v2/"

	Full          NotificationThreshold = 4
	ThreeQuarters NotificationThreshold = 3
	Half          NotificationThreshold = 2
	Quarter       NotificationThreshold = 1
	None          NotificationThreshold = 0
)

var (
	// url default values needed for unit testing
	auth0Domain       = environment.GetString("AUTH0_DOMAIN", "https://test-auth0.com")
	auth0ClientId     = environment.GetString("AUTH0_CLIENT_ID", "")
	auth0ClientSecret = environment.GetString("AUTH0_CLIENT_SECRET", "")

	errAuth0TokenResponse = errors.New("error fetching management token from Auth0")
	errAuth0UserResponse  = errors.New("error fetching user from Auth0")
	errAuth0UserNotFound  = errors.New("user not found in Auth0")

	defaultNotificationSettings = repository.NotificationSettings{SignedUp: true, Quarter: false, Half: false, ThreeQuarters: true, Full: true}
)

type (
	Notifier struct {
		cache       *cache.Cache
		emailClient EmailClient
	}

	AppUsage struct {
		Email     string
		Name      string
		Limit     int
		Usage     int
		Threshold NotificationThreshold
	}

	Auth0Token struct {
		Token string `json:"access_token"`
	}
	Auth0User struct {
		Email string `json:"email"`
	}

	NotificationThreshold int
)

// Main func called on a set interval to run the notification worker.
// Creates a map of all used applications above a notification threshold and send emails to them.
func HandleNotifications(cache *cache.Cache) error {
	notifier := newNotifier(cache)

	usageMap, err := notifier.createUsageMap()
	if err != nil {
		return fmt.Errorf("error creating notification usage map: " + err.Error())
	}

	err = notifier.sendEmails(usageMap)
	if err != nil {
		return fmt.Errorf("error sending emails for notification worker: " + err.Error())
	}

	return nil
}

func newNotifier(cache *cache.Cache) *Notifier {
	return &Notifier{
		cache:       cache,
		emailClient: *newEmailClient(),
	}
}

// Creates a map of all applications that meet the criteria for usage above their notification settings.
// Also includes a call to Auth0 to first get an OAuth toaken and then fetch each user's email address.
func (n *Notifier) createUsageMap() (map[string]AppUsage, error) {
	appLimits, relaysCount := n.cache.AppLimits, n.cache.RelaysCount

	usageMap := make(map[string]AppUsage, len(relaysCount))

	auth0Token, err := n.getAuth0MgmtToken()
	if err != nil {
		return nil, err
	}

	for _, app := range relaysCount {
		appDetails := appLimits[app.Application]
		appUsage := int(app.Count.Failure + app.Count.Success)

		if reflect.DeepEqual(appDetails, repository.AppLimits{}) {
			// TODO Add logger with info about missing AppLimits details
			continue
		}

		if reflect.DeepEqual(*appDetails.NotificationSettings, repository.NotificationSettings{}) {
			appDetails.NotificationSettings = &defaultNotificationSettings
		}

		threshold := getAppThreshold(appUsage, appDetails.DailyLimit, *appDetails.NotificationSettings)

		// TODO Only add app usage to map if >= threshold not already present in cache for app ID.
		// (ie. daily email wasn't already sent for this app for a equal or lesser threshold).
		if appUsage > 0 && threshold != None {
			auth0UserEmail, err := n.getAuth0UserEmail(appDetails.AppUserID, auth0Token)
			if err != nil {
				// TODO Add logger with info about missing Auth0 user
				continue
			}

			usageMap[appDetails.AppID] = AppUsage{
				Usage:     appUsage,
				Limit:     appDetails.DailyLimit,
				Email:     auth0UserEmail,
				Name:      appDetails.AppName,
				Threshold: threshold,
			}
		}
	}

	return usageMap, nil
}

// Fetches a fresh OAuth management token from the Auth0 API to be used for fetching user email addresses.
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

// Gets the email address for one user, by user ID.
func (n *Notifier) getAuth0UserEmail(userID string, token string) (string, error) {
	auth0Url := fmt.Sprintf("%s/%s", auth0Domain, auth0UsersEndpoint)
	params := url.Values{
		"q":              {fmt.Sprintf("user_id:*%s", userID)},
		"fields":         {"email"},
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

// Returns an enum from 0-4 to indicate whether the user has usage over their set notification limits.
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

// Sends emails using the email client to all users present in the usage map.
// Also sets the app's threshold in the cache to prevent sending the same email more than once per day.
func (n *Notifier) sendEmails(usageMap map[string]AppUsage) error {
	for appID, appToNotify := range usageMap {
		usagePercent := float64(appToNotify.Usage) / float64(appToNotify.Limit)
		usageRounded := fmt.Sprintf("%.2f", usagePercent)

		emailConfig := emailConfig{
			TemplateData: templateData{
				AppID:   appID,
				AppName: appToNotify.Name,
				Usage:   usageRounded,
			},
			TemplateName: "NotificationThresholdHit",
			ToEmail:      appToNotify.Email,
		}

		// TODO Add logger with info about app's usage

		err := n.emailClient.sendEmail(emailConfig)
		if err != nil {
			return err
		}

		// TODO Add logger with info about sent email

		// TODO Set entry in cache for highest threshold sent today for this app
		// (PSEUDO CODE) cache.set(appID, appToNotify.Threshold, keepUntilNextDayInUTCTime)

		// TODO Add logger with info about saving threshold to cache
	}

	return nil
}
