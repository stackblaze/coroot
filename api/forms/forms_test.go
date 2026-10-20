package forms

import (
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/utils"
	"github.com/stretchr/testify/assert"
)

func TestWebhookFormMasksSecrets(t *testing.T) {
	p := &db.Project{}
	p.Settings.Integrations.Webhook = &db.IntegrationWebhook{
		Url:           "https://hooks.example.com/x",
		BasicAuth:     &utils.BasicAuth{User: "user", Password: "secret"},
		CustomHeaders: []utils.Header{{Key: "Authorization", Value: "Bearer token"}},
		CustomFields:  map[string]string{"api_key": "value"},
	}

	var masked IntegrationFormWebhook
	masked.Get(p, true)
	assert.Equal(t, "<hidden>", masked.Url)
	assert.Equal(t, "<hidden>", masked.BasicAuth.User)
	assert.Equal(t, "<hidden>", masked.BasicAuth.Password)
	assert.Equal(t, []utils.Header{{Key: "Authorization", Value: "<hidden>"}}, masked.CustomHeaders)
	assert.Equal(t, map[string]string{"api_key": "<hidden>"}, masked.CustomFields)

	// The stored configuration is not modified.
	cfg := p.Settings.Integrations.Webhook
	assert.Equal(t, "secret", cfg.BasicAuth.Password)
	assert.Equal(t, "Bearer token", cfg.CustomHeaders[0].Value)
	assert.Equal(t, "value", cfg.CustomFields["api_key"])

	var full IntegrationFormWebhook
	full.Get(p, false)
	assert.Equal(t, "secret", full.BasicAuth.Password)
	assert.Equal(t, "Bearer token", full.CustomHeaders[0].Value)
}
