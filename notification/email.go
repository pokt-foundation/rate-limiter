package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mailgun/mailgun-go/v4"
	"github.com/pokt-foundation/utils-go/environment"
)

const (
	domain    = "pokt.network"
	fromEmail = "Pocket Portal <portal@pokt.network>"
)

var (
	mailgunApiKey        = environment.GetString("MAILGUN_API_KEY", "")
	whitelistedTemplates = map[string][2]string{
		"NotificationChange": {
			"pocket-dashboard-notifications-changed",
			"Pocket Portal: Notification settings",
		},
		"NotificationSignup": {
			"pocket-dashboard-notifications-signup",
			"Pocket Portal: You've signed up for notifications",
		},
		"NotificationThresholdHit": {
			"pocket-dashboard-notifications-threshold-hit",
			"Pocket Portal: Endpoint Notification",
		},
		"PasswordReset": {
			"pocket-dashboard-password-reset",
			"Pocket Portal: Reset your password",
		},
		"SignUp": {
			"pocket-dashboard-signup",
			"Pocket Portal: Sign up",
		},
		"Unstake": {
			"pocket-portal-unstake-notification",
			"Pocket Portal: Unstake Notification",
		},
		"FeedBack": {
			"pocket-portal-feedback-box",
			"Pocket Portal: Feedback Box",
		},
	}
)

type (
	EmailClient struct {
		mailgun *mailgun.MailgunImpl
	}

	emailConfig struct {
		TemplateData templateData
		TemplateName string
		ToEmail      string
	}

	templateData struct {
		AppID   string `json:"app_id"`
		AppName string `json:"app_name"`
		Usage   string `json:"usage"`
	}
)

func newEmailClient() *EmailClient {
	mailgun := mailgun.NewMailgun(domain, mailgunApiKey)

	return &EmailClient{mailgun: mailgun}
}

// TODO Email client needs unit tests (and then integration tests actually using Mailgun)

// Sends an email using the email client (currently Mailgun)
func (e *EmailClient) sendEmail(config emailConfig) error {
	sender := fromEmail
	recipient := config.ToEmail
	subject := whitelistedTemplates[config.TemplateName][0]
	template := whitelistedTemplates[config.TemplateName][1]
	body := ""

	message := e.mailgun.NewMessage(sender, subject, body, recipient)
	message.SetTemplate(template)

	if config.TemplateData.AppID != "" {
		jsonData, err := json.Marshal(config.TemplateData)
		if err != nil {
			return fmt.Errorf("error marshalling json data for email: " + err.Error())
		}

		templateErr := message.AddTemplateVariable("h:X-Mailgun-Variables", jsonData)
		if templateErr != nil {
			return fmt.Errorf("error adding template variable to email: " + templateErr.Error())
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	resp, id, err := e.mailgun.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("error sending email: " + err.Error())
	}

	fmt.Printf("ID: %s Resp: %s\n", id, resp)
	return nil
}
