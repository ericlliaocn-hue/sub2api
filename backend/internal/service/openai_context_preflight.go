package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// OpenAIContextWindowCheck is the result of the best-effort input-size check
// performed after an account has been selected and before an upstream request
// is sent. A nil result means that no trustworthy context limit is known.
type OpenAIContextWindowCheck struct {
	Model                 string
	EstimatedInputTokens  int
	RequestedOutputTokens int
	ContextWindow         int64
}

func (c *OpenAIContextWindowCheck) Exceeds() bool {
	if c == nil || c.ContextWindow <= 0 {
		return false
	}
	return int64(c.EstimatedInputTokens+c.RequestedOutputTokens)+openAIContextWindowSafetyMargin > c.ContextWindow
}

const openAIContextWindowSafetyMargin int64 = 4096

// CheckOpenAIChatCompletionsContext estimates a Chat Completions request using
// the same tiktoken estimator as the input_tokens endpoint. It understands both
// ordinary messages bodies and Responses-shaped bodies sent to the CC route.
// Unknown account limits fail open so custom providers are not blocked by
// incomplete metadata.
func CheckOpenAIChatCompletionsContext(account *Account, body []byte, defaultMappedModel string) (*OpenAIContextWindowCheck, error) {
	if account == nil || len(body) == 0 {
		return nil, nil
	}

	requestedModel := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if requestedModel == "" {
		return nil, nil
	}
	billingModel := resolveOpenAIForwardModel(account, requestedModel, defaultMappedModel)
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)
	contextWindow := openAIContextWindowForAccount(account, upstreamModel)
	if contextWindow <= 0 {
		return nil, nil
	}

	countReq, err := openAIChatCompletionsCountRequest(body, upstreamModel)
	if err != nil {
		return nil, err
	}
	estimated, err := estimateOpenAIInputTokens(countReq.Request)
	if err != nil {
		return nil, err
	}

	return &OpenAIContextWindowCheck{
		Model:                 upstreamModel,
		EstimatedInputTokens:  estimated,
		RequestedOutputTokens: countReq.RequestedOutputTokens,
		ContextWindow:         contextWindow,
	}, nil
}

type openAIChatCompletionsCount struct {
	RequestedOutputTokens int
	Request               openAIInputTokensCountRequest
}

func openAIChatCompletionsCountRequest(body []byte, upstreamModel string) (*openAIChatCompletionsCount, error) {
	if !gjson.GetBytes(body, "messages").Exists() && gjson.GetBytes(body, "input").Exists() {
		var responsesReq apicompat.ResponsesRequest
		if err := json.Unmarshal(body, &responsesReq); err != nil {
			return nil, fmt.Errorf("parse responses-shaped chat completions request for context check: %w", err)
		}
		return &openAIChatCompletionsCount{
			RequestedOutputTokens: positiveIntPointerValue(responsesReq.MaxOutputTokens),
			Request: openAIInputTokensCountRequest{
				Model:        upstreamModel,
				Instructions: responsesReq.Instructions,
				Input:        responsesReq.Input,
				Tools:        responsesReq.Tools,
				ToolChoice:   responsesReq.ToolChoice,
			},
		}, nil
	}

	var chatReq apicompat.ChatCompletionsRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		return nil, fmt.Errorf("parse chat completions request for context check: %w", err)
	}
	converted, err := apicompat.ChatCompletionsToResponses(&chatReq)
	if err != nil {
		return nil, fmt.Errorf("convert chat completions request for context check: %w", err)
	}
	return &openAIChatCompletionsCount{
		RequestedOutputTokens: positiveIntPointerValue(converted.MaxOutputTokens),
		Request: openAIInputTokensCountRequest{
			Model:        upstreamModel,
			Instructions: converted.Instructions,
			Input:        converted.Input,
			Tools:        converted.Tools,
			ToolChoice:   converted.ToolChoice,
		},
	}, nil
}

func positiveIntPointerValue(value *int) int {
	if value == nil || *value <= 0 {
		return 0
	}
	return *value
}

func openAIContextWindowForAccount(account *Account, upstreamModel string) int64 {
	if account == nil {
		return 0
	}
	if metadata, ok := account.GetUpstreamModelMetadata(upstreamModel); ok && metadata.ContextWindow > 0 {
		return metadata.ContextWindow
	}
	// OAuth accounts may not have a synced model snapshot. Keep a conservative
	// GPT-5.6 guardrail aligned with the Codex manifest until the account reports
	// a more specific context window.
	if isOpenAIGPT56Model(upstreamModel) {
		return configuredCodexGPT56MaxContext
	}
	return 0
}

// WriteOpenAIContextWindowError returns a client-visible 400 with the same
// machine-readable error code as the upstream, while keeping the request out
// of the upstream account entirely.
func WriteOpenAIContextWindowError(c *gin.Context) {
	if c == nil {
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{
		"error": gin.H{
			"type":    "invalid_request_error",
			"code":    "context_length_exceeded",
			"param":   "input",
			"message": "Your input exceeds the context window of this model. Please start a new conversation or reduce the input.",
		},
	})
}
