package service

import (
	"context"
	"net/url"
	"strings"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DefaultQQGroupName = "技术交流群"
	DefaultQQGroupURL  = "https://qm.qq.com/cgi-bin/qm/qr?k=z77tL_WEPI5-c3fdp_oJDFXJPnXVp-LF&jump_from=webapi&authKey=zR8OIdt3V5nYIkcXWBb70JeuoBtdsLIjNOMUM4+1uO9VaXPcidkgKvdn6DU4q0J5"
)

func normalizePelicanVisibility(value string) string {
	if value == "" || value == "all" {
		return "all"
	}
	return "admin"
}

func qqGroupURLFromSettings(settings map[string]string) string {
	if value, exists := settings[SettingKeyQQGroupURL]; exists {
		return value
	}
	return DefaultQQGroupURL
}

func validateHeaderFeatureSettings(settings *SystemSettings) error {
	if settings.PelicanVisibility != "" && settings.PelicanVisibility != "all" && settings.PelicanVisibility != "admin" {
		return infraerrors.BadRequest("INVALID_PELICAN_VISIBILITY", "鹈鹕测智可见范围必须为 all 或 admin")
	}
	if utf8.RuneCountInString(strings.TrimSpace(settings.QQGroupName)) > 50 {
		return infraerrors.BadRequest("INVALID_QQ_GROUP_NAME", "技术交流群显示名称不能超过 50 字")
	}
	value := strings.TrimSpace(settings.QQGroupURL)
	if value != "" {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || len(value) > 2048 {
			return infraerrors.BadRequest("INVALID_QQ_GROUP_URL", "技术交流群链接必须为有效的 HTTP 或 HTTPS 地址，且不超过 2048 字符")
		}
	}
	return nil
}

type HeaderFeaturesRuntime struct {
	PelicanEnabled    bool
	PelicanVisibility string
	FeedbackEnabled   bool
}

// Missing settings keep existing deployments enabled. Storage failures deny access.
func (s *SettingService) GetHeaderFeaturesRuntime(ctx context.Context) (HeaderFeaturesRuntime, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyPelicanEnabled, SettingKeyPelicanVisibility, SettingKeyFeedbackEnabled})
	if err != nil {
		return HeaderFeaturesRuntime{}, err
	}
	return HeaderFeaturesRuntime{
		PelicanEnabled:    !isFalseSettingValue(values[SettingKeyPelicanEnabled]),
		PelicanVisibility: normalizePelicanVisibility(values[SettingKeyPelicanVisibility]),
		FeedbackEnabled:   !isFalseSettingValue(values[SettingKeyFeedbackEnabled]),
	}, nil
}
