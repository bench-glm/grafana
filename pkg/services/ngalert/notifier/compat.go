package notifier

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/alerting/definition"
	alertingModels "github.com/grafana/alerting/models"
	alertingNotify "github.com/grafana/alerting/notify"
	"github.com/grafana/alerting/templates"

	apimodels "github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
	"github.com/grafana/grafana/pkg/services/ngalert/models"
)

// Silence-specific compat functions to convert between grafana/alerting and model types.

func GettableSilenceToSilence(s alertingNotify.GettableSilence) *models.Silence {
	sil := models.Silence(s)
	return &sil
}

func GettableSilencesToSilences(silences alertingNotify.GettableSilences) []*models.Silence {
	res := make([]*models.Silence, 0, len(silences))
	for _, sil := range silences {
		res = append(res, GettableSilenceToSilence(*sil))
	}
	return res
}

func SilenceToPostableSilence(s models.Silence) *alertingNotify.PostableSilence {
	var id string
	if s.ID != nil {
		id = *s.ID
	}
	return &alertingNotify.PostableSilence{
		ID:      id,
		Silence: s.Silence,
	}
}

func IntegrationToIntegrationConfig(i models.Integration) (alertingModels.IntegrationConfig, error) {
	raw, err := json.Marshal(i.Settings)
	if err != nil {
		return alertingModels.IntegrationConfig{}, err
	}
	return alertingModels.IntegrationConfig{
		UID:                   i.UID,
		Name:                  i.Name,
		Type:                  i.Config.Type(),
		Version:               i.Config.Version,
		DisableResolveMessage: i.DisableResolveMessage,
		Settings:              raw,
		SecureSettings:        i.SecureSettings,
	}, nil
}

func PostableAPIConfigToNotificationsConfiguration(c *apimodels.PostableUserConfig, limits alertingNotify.DynamicLimits) alertingNotify.NotificationsConfiguration {
	return alertingNotify.NotificationsConfiguration{
		RoutingTree:       c.AlertmanagerConfig.Route,
		InhibitRules:      c.AlertmanagerConfig.InhibitRules,
		MuteTimeIntervals: c.AlertmanagerConfig.MuteTimeIntervals,
		TimeIntervals:     c.AlertmanagerConfig.TimeIntervals,
		Templates:         alertingNotify.PostableAPITemplatesToTemplateDefinitions(c.GetMergedTemplateDefinitions()),
		Receivers:         alertingNotify.PostableAPIReceiversToAPIReceivers(c.AlertmanagerConfig.Receivers),
		Limits:            limits,
	}
}

func NotificationsConfigurationToPostableAPIConfig(config alertingNotify.NotificationsConfiguration) apimodels.PostableApiAlertingConfig {
	return apimodels.PostableApiAlertingConfig{
		Config: apimodels.Config{
			Global:            nil, // Grafana does not have global.
			Route:             config.RoutingTree,
			InhibitRules:      config.InhibitRules,
			TimeIntervals:     config.TimeIntervals,
			MuteTimeIntervals: config.MuteTimeIntervals,
			Templates:         nil, // we do not use this.
		},
		Receivers: APIReceiversToPostableAPIReceivers(config.Receivers),
	}
}

func APIReceiversToPostableAPIReceivers(r []*alertingNotify.APIReceiver) []*definition.PostableApiReceiver {
	result := make([]*definition.PostableApiReceiver, 0, len(r))
	for _, receiver := range r {
		result = append(result, APIReceiverToPostableAPIReceiver(receiver))
	}
	return result
}

func APIReceiverToPostableAPIReceiver(r *alertingNotify.APIReceiver) *definition.PostableApiReceiver {
	receivers := make([]*definition.PostableGrafanaReceiver, 0, len(r.Integrations))
	for _, p := range r.Integrations {
		receivers = append(receivers, IntegrationConfigToPostableGrafanaReceiver(p))
	}

	return &definition.PostableApiReceiver{
		Receiver: r.ConfigReceiver,
		PostableGrafanaReceivers: definition.PostableGrafanaReceivers{
			GrafanaManagedReceivers: receivers,
		},
	}
}

func IntegrationConfigToPostableGrafanaReceiver(r *alertingModels.IntegrationConfig) *definition.PostableGrafanaReceiver {
	return &definition.PostableGrafanaReceiver{
		UID:                   r.UID,
		Name:                  r.Name,
		Type:                  string(r.Type),
		Version:               string(r.Version),
		DisableResolveMessage: r.DisableResolveMessage,
		Settings:              definition.RawMessage(r.Settings),
		SecureSettings:        r.SecureSettings,
	}
}

// TemplateDefinitionToPostableAPITemplate converts a templates.TemplateDefinition to a definition.PostableApiTemplate
func TemplateDefinitionToPostableAPITemplate(t templates.TemplateDefinition) definition.PostableApiTemplate {
	var kind definition.TemplateKind
	switch t.Kind {
	case templates.GrafanaKind:
		kind = definition.GrafanaTemplateKind
	case templates.MimirKind:
		kind = definition.MimirTemplateKind
	}
	return definition.PostableApiTemplate{
		Name:    t.Name,
		Content: t.Template,
		Kind:    kind,
	}
}

func TemplateDefinitionsToPostableAPITemplates(ts []templates.TemplateDefinition) []definition.PostableApiTemplate {
	defs := make([]definition.PostableApiTemplate, 0, len(ts))
	for _, t := range ts {
		defs = append(defs, TemplateDefinitionToPostableAPITemplate(t))
	}
	return defs
}

// PostableMimirReceiverToIntegrations converts all legacy models to apimodels.PostableGrafanaReceiver.
// If receiver does not have any legacy receivers, returns the original receiver.
// Otherwise, returns a copy that contains converted integrations (and shallow copy of existing Grafana integrations).
func PostableMimirReceiverToIntegrations(r *apimodels.PostableApiReceiver) (*apimodels.PostableApiReceiver, error) {
	if !r.HasMimirIntegrations() {
		return r, nil
	}
	v0, err := alertingNotify.ConfigReceiverToMimirIntegrations(r.Receiver)
	if err != nil {
		return nil, fmt.Errorf("failed to convert v0 receiver to integrations: %w", err)
	}
	result := &apimodels.PostableApiReceiver{
		Receiver: apimodels.Receiver{
			Name: r.Receiver.Name,
		},
		PostableGrafanaReceivers: apimodels.PostableGrafanaReceivers{
			GrafanaManagedReceivers: make([]*apimodels.PostableGrafanaReceiver, 0, len(v0)+len(r.GrafanaManagedReceivers)),
		},
	}
	result.GrafanaManagedReceivers = append(result.GrafanaManagedReceivers, r.GrafanaManagedReceivers...)
	for idx, config := range v0 {
		integration, err := MimirIntegrationConfigToPostableGrafanaReceiver(config, r.Name, idx)
		if err != nil {
			return nil, fmt.Errorf("failed to convert Mimir integration config to PostableGrafanaReceiver: %w", err)
		}
		result.GrafanaManagedReceivers = append(result.GrafanaManagedReceivers, integration)
	}
	return result, nil
}

// MimirIntegrationConfigToPostableGrafanaReceiver Converts a Mimir integration configuration to a PostableGrafanaReceiver. All settings are unencrypted. Needs to be encrypted later.
func MimirIntegrationConfigToPostableGrafanaReceiver(config alertingNotify.MimirIntegrationConfig, receiverName string, idx int) (*definition.PostableGrafanaReceiver, error) {
	raw, err := config.ConfigJSON()
	if err != nil {
		return nil, err
	}

	return &definition.PostableGrafanaReceiver{
		UID:                   fmt.Sprintf("%s-%d", models.NameToUid(receiverName), idx),
		Name:                  receiverName,
		Type:                  string(config.Schema.Type()),
		Version:               string(config.Schema.Version),
		DisableResolveMessage: false, // V0 ignore this flag as they have their own SendResolved one.
		Settings:              raw,
		SecureSettings:        nil,
	}, nil
}
