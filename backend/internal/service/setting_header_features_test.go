//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestHeaderFeaturesPublicDefaultsAndInjection(t *testing.T) {
	repo := &settingPublicRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.PelicanEnabled)
	require.Equal(t, "all", settings.PelicanVisibility)
	require.True(t, settings.FeedbackEnabled)
	require.True(t, settings.QQGroupEnabled)
	require.Equal(t, DefaultQQGroupURL, settings.QQGroupURL)

	repo.values = map[string]string{
		SettingKeyPelicanEnabled: "false", SettingKeyPelicanVisibility: "admin",
		SettingKeyFeedbackEnabled: "false", SettingKeyQQGroupEnabled: "false",
		SettingKeyQQGroupName: "开发者社区", SettingKeyQQGroupURL: "https://example.com/join",
	}
	payload, err := svc.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	var values map[string]any
	require.NoError(t, json.Unmarshal(encoded, &values))
	require.Equal(t, false, values["pelican_enabled"])
	require.Equal(t, "admin", values["pelican_visibility"])
	require.Equal(t, false, values["feedback_enabled"])
	require.Equal(t, false, values["qq_group_enabled"])
	require.Equal(t, "开发者社区", values["qq_group_name"])
	require.Equal(t, "https://example.com/join", values["qq_group_url"])
	repo.values[SettingKeyQQGroupURL] = ""
	settings, err = svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Empty(t, settings.QQGroupURL, "explicitly clearing the link must not restore the legacy URL")
}

func TestHeaderFeaturesRuntime(t *testing.T) {
	repo := &settingPublicRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	features, err := svc.GetHeaderFeaturesRuntime(context.Background())
	require.NoError(t, err)
	require.Equal(t, HeaderFeaturesRuntime{PelicanEnabled: true, PelicanVisibility: "all", FeedbackEnabled: true}, features)
	repo.values[SettingKeyPelicanVisibility] = "invalid"
	features, err = svc.GetHeaderFeaturesRuntime(context.Background())
	require.NoError(t, err)
	require.Equal(t, "admin", features.PelicanVisibility)
	repo.err = errors.New("unavailable")
	features, err = svc.GetHeaderFeaturesRuntime(context.Background())
	require.Error(t, err)
	require.False(t, features.PelicanEnabled)
}

func TestHeaderFeaturesValidation(t *testing.T) {
	for _, value := range []string{"javascript:alert(1)", "//example.com/join", "https://", "https://user:pass@example.com"} {
		t.Run(value, func(t *testing.T) {
			require.Error(t, validateHeaderFeatureSettings(&SystemSettings{QQGroupURL: value}))
		})
	}
	require.Error(t, validateHeaderFeatureSettings(&SystemSettings{PelicanVisibility: "everyone"}))
	require.NoError(t, validateHeaderFeatureSettings(&SystemSettings{PelicanVisibility: "admin", QQGroupName: "交流群", QQGroupURL: "https://qm.qq.com/q/test"}))
}
