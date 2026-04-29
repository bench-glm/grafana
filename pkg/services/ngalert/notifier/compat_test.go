package notifier

import (
	"fmt"
	"testing"

	alertingNotify "github.com/grafana/alerting/notify"
	emailV0 "github.com/grafana/alerting/receivers/email/v0mimir1"
	webhookV0 "github.com/grafana/alerting/receivers/webhook/v0mimir1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apimodels "github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
	"github.com/grafana/grafana/pkg/services/ngalert/models"
)

func TestPostableMimirReceiverToIntegrations(t *testing.T) {
	t.Run("returns original pointer when receiver has only Grafana integrations", func(t *testing.T) {
		receiver := &apimodels.PostableApiReceiver{
			Receiver: apimodels.Receiver{Name: "test"},
			PostableGrafanaReceivers: apimodels.PostableGrafanaReceivers{
				GrafanaManagedReceivers: []*apimodels.PostableGrafanaReceiver{
					{UID: "grafana-uid", Name: "test", Type: "email"},
				},
			},
		}
		result, err := PostableMimirReceiverToIntegrations(receiver)
		require.NoError(t, err)
		assert.Same(t, receiver, result)
	})

	t.Run("converts Mimir integrations to Grafana integrations", func(t *testing.T) {
		wh := webhookV0.GetFullValidConfig()
		receiver := &apimodels.PostableApiReceiver{
			Receiver: apimodels.Receiver{
				Name:           "test-receiver",
				WebhookConfigs: []*webhookV0.Config{&wh},
			},
		}

		mimirConfigs, err := alertingNotify.ConfigReceiverToMimirIntegrations(receiver.Receiver)
		require.NoError(t, err)
		require.Len(t, mimirConfigs, 1)
		expectedJSON, err := mimirConfigs[0].ConfigJSON()
		require.NoError(t, err)

		result, err := PostableMimirReceiverToIntegrations(receiver)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotSame(t, receiver, result)
		require.Len(t, result.GrafanaManagedReceivers, 1)

		converted := result.GrafanaManagedReceivers[0]
		assert.Equal(t, "test-receiver", result.Name)
		assert.Equal(t, "test-receiver", converted.Name)
		assert.Equal(t, fmt.Sprintf("%s-0", models.NameToUid("test-receiver")), converted.UID)
		assert.JSONEq(t, string(expectedJSON), string(converted.Settings))
		assert.False(t, converted.DisableResolveMessage)
		assert.Nil(t, converted.SecureSettings)
		assert.False(t, result.HasMimirIntegrations())
	})

	t.Run("existing Grafana integrations appear before converted Mimir ones", func(t *testing.T) {
		wh := webhookV0.GetFullValidConfig()
		grafanaRecv := &apimodels.PostableGrafanaReceiver{
			UID:  "existing-uid",
			Name: "existing",
			Type: "email",
		}
		receiver := &apimodels.PostableApiReceiver{
			Receiver: apimodels.Receiver{
				Name:           "mixed-receiver",
				WebhookConfigs: []*webhookV0.Config{&wh},
			},
			PostableGrafanaReceivers: apimodels.PostableGrafanaReceivers{
				GrafanaManagedReceivers: []*apimodels.PostableGrafanaReceiver{grafanaRecv},
			},
		}

		result, err := PostableMimirReceiverToIntegrations(receiver)
		require.NoError(t, err)
		require.Len(t, result.GrafanaManagedReceivers, 2)
		assert.Same(t, grafanaRecv, result.GrafanaManagedReceivers[0])
		assert.Equal(t, "webhook", result.GrafanaManagedReceivers[1].Type)
	})

	t.Run("assigns sequential UIDs to converted Mimir integrations", func(t *testing.T) {
		// email field comes before webhook in the Receiver struct, so after conversion:
		// index 0 = email, index 1 = webhook
		em := emailV0.GetFullValidConfig()
		wh := webhookV0.GetFullValidConfig()
		receiver := &apimodels.PostableApiReceiver{
			Receiver: apimodels.Receiver{
				Name:           "multi-receiver",
				EmailConfigs:   []*emailV0.Config{&em},
				WebhookConfigs: []*webhookV0.Config{&wh},
			},
		}

		result, err := PostableMimirReceiverToIntegrations(receiver)
		require.NoError(t, err)
		require.Len(t, result.GrafanaManagedReceivers, 2)

		uid0 := fmt.Sprintf("%s-0", models.NameToUid("multi-receiver"))
		uid1 := fmt.Sprintf("%s-1", models.NameToUid("multi-receiver"))
		assert.Equal(t, uid0, result.GrafanaManagedReceivers[0].UID)
		assert.Equal(t, uid1, result.GrafanaManagedReceivers[1].UID)
	})
}
