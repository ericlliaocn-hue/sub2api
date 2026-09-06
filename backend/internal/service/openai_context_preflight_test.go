package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckOpenAIChatCompletionsContextUsesAccountMetadata(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			UpstreamModelMetadataExtraKey: map[string]any{
				"models": map[string]any{
					"gpt-5.6-luna": map[string]any{"context_window": 10_000},
				},
			},
		},
	}

	within, err := CheckOpenAIChatCompletionsContext(account, []byte(`{"model":"gpt-5.6-luna","messages":[{"role":"user","content":"hello"}],"max_tokens":100}`), "")
	require.NoError(t, err)
	require.NotNil(t, within)
	require.False(t, within.Exceeds())
	require.Equal(t, int64(10_000), within.ContextWindow)

	tooLargeBody := `{"model":"gpt-5.6-luna","messages":[{"role":"user","content":"` + strings.Repeat("word ", 7000) + `"}],"max_tokens":100}`
	tooLarge, err := CheckOpenAIChatCompletionsContext(account, []byte(tooLargeBody), "")
	require.NoError(t, err)
	require.NotNil(t, tooLarge)
	require.True(t, tooLarge.Exceeds())
	require.Greater(t, tooLarge.EstimatedInputTokens, 0)
}

func TestCheckOpenAIChatCompletionsContextSupportsResponsesShape(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"model":"gpt-5.6-luna","input":"hello","max_output_tokens":100}`)
	check, err := CheckOpenAIChatCompletionsContext(account, body, "")
	require.NoError(t, err)
	require.NotNil(t, check)
	require.Equal(t, "gpt-5.6-luna", check.Model)
	require.Equal(t, 100, check.RequestedOutputTokens)
	require.False(t, check.Exceeds())
}

func TestCheckOpenAIChatCompletionsContextLeavesUnknownModelsUnchanged(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	check, err := CheckOpenAIChatCompletionsContext(account, []byte(`{"model":"custom-model","messages":[{"role":"user","content":"hello"}]}`), "")
	require.NoError(t, err)
	require.Nil(t, check)
}
