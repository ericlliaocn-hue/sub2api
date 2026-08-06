package service

import (
	"context"
	"strconv"
)

// CreationSettings controls the user-facing Creation Studio. It deliberately
// does not contain provider credentials: image/video requests still use the
// existing Sub2API API-key -> group -> account routing chain.
type CreationSettings struct {
	Enabled      bool `json:"enabled"`
	ImageEnabled bool `json:"image_enabled"`
	VideoEnabled bool `json:"video_enabled"`
}

var defaultCreationSettings = CreationSettings{
	Enabled:      true,
	ImageEnabled: true,
	VideoEnabled: false,
}

func (s *SettingService) GetCreationSettings(ctx context.Context) (CreationSettings, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyCreationCenterEnabled,
		SettingKeyCreationImageEnabled,
		SettingKeyCreationVideoEnabled,
	})
	if err != nil {
		return CreationSettings{}, err
	}
	return CreationSettings{
		Enabled:      boolSettingOrDefault(values[SettingKeyCreationCenterEnabled], defaultCreationSettings.Enabled),
		ImageEnabled: boolSettingOrDefault(values[SettingKeyCreationImageEnabled], defaultCreationSettings.ImageEnabled),
		VideoEnabled: boolSettingOrDefault(values[SettingKeyCreationVideoEnabled], defaultCreationSettings.VideoEnabled),
	}, nil
}

func (s *SettingService) UpdateCreationSettings(ctx context.Context, settings CreationSettings) error {
	return s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyCreationCenterEnabled: strconv.FormatBool(settings.Enabled),
		SettingKeyCreationImageEnabled:  strconv.FormatBool(settings.ImageEnabled),
		SettingKeyCreationVideoEnabled:  strconv.FormatBool(settings.VideoEnabled),
	})
}

func boolSettingOrDefault(value string, fallback bool) bool {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
