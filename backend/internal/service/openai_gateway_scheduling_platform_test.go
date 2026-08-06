package service

import "testing"

func TestNormalizeOpenAICompatiblePlatformPreservesMediaPlatforms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform string
		want     string
	}{
		{name: "openai", platform: PlatformOpenAI, want: PlatformOpenAI},
		{name: "grok", platform: PlatformGrok, want: PlatformGrok},
		{name: "seedance", platform: PlatformSeedance, want: PlatformSeedance},
		{name: "unknown falls back to openai", platform: "unknown", want: PlatformOpenAI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeOpenAICompatiblePlatform(tt.platform); got != tt.want {
				t.Fatalf("normalizeOpenAICompatiblePlatform(%q) = %q, want %q", tt.platform, got, tt.want)
			}
		})
	}
}
