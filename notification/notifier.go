package notification

import (
	"github.com/pokt-foundation/portal-api-go/repository"
	"github.com/pokt-foundation/rate-limiter/cache"
)

// const (
// 	appLimitsEndpoint     = "/application/limits"
// 	appRelayMeterEndpoint = "/v0/relays/apps"
// )

type Notifier struct {
	cache *cache.Cache
}

type AppUsage struct {
	email                string
	name                 string
	limit                int
	usage                int
	notificationSettings repository.NotificationSettings
}

func NewNotifier(cache *cache.Cache) *Notifier {
	return &Notifier{
		cache: cache,
	}
}

func (n *Notifier) createUsageMap() {
	appLimits := n.cache.AppLimits
	relaysCount := n.cache.RelaysCount

	usageMap := make(map[string]AppUsage, len(relaysCount))

	for _, app := range relaysCount {
		appPubKey := app.Application

		usageMap[appPubKey] = AppUsage{
			email: "",
			name:  "",
			limit: appLimits[appPubKey].DailyLimit,
			usage: int(app.Count.Failure + app.Count.Success),
		}

	}
}
