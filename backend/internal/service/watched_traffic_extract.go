package service

import (
	"bytes"
	"strings"

	"github.com/tidwall/gjson"
)

func ExtractWatchedPrompt(body []byte) (model, prompt string) {
	if len(bytes.TrimSpace(body)) == 0 {
		return "", ""
	}
	if !gjson.ValidBytes(body) {
		return "", clipWatchedText(string(body))
	}
	root := gjson.ParseBytes(body)
	model = strings.TrimSpace(root.Get("model").String())
	parts := make([]string, 0, 8)
	if sys := flattenWatchedContent(root.Get("system")); sys != "" {
		parts = append(parts, "system: "+sys)
	}
	if instr := flattenWatchedContent(root.Get("instructions")); instr != "" {
		parts = append(parts, "instructions: "+instr)
	}
	if msgs := root.Get("messages"); msgs.IsArray() {
		msgs.ForEach(func(_, msg gjson.Result) bool {
			text := flattenWatchedContent(msg.Get("content"))
			if text == "" {
				return true
			}
			if role := strings.TrimSpace(msg.Get("role").String()); role != "" {
				parts = append(parts, role+": "+text)
				return true
			}
			parts = append(parts, text)
			return true
		})
	}
	if len(parts) == 0 {
		if input := flattenWatchedContent(root.Get("input")); input != "" {
			parts = append(parts, input)
		}
	}
	if len(parts) == 0 {
		if contents := root.Get("contents"); contents.IsArray() {
			contents.ForEach(func(_, content gjson.Result) bool {
				role := strings.TrimSpace(content.Get("role").String())
				text := flattenWatchedContent(content.Get("parts"))
				if text == "" {
					text = flattenWatchedContent(content)
				}
				if text == "" {
					return true
				}
				if role != "" {
					parts = append(parts, role+": "+text)
					return true
				}
				parts = append(parts, text)
				return true
			})
		}
	}
	if len(parts) == 0 {
		if p := strings.TrimSpace(root.Get("prompt").String()); p != "" {
			parts = append(parts, p)
		}
	}
	prompt = strings.TrimSpace(strings.Join(parts, "\n"))
	if prompt == "" {
		prompt = clipWatchedText(string(body))
	}
	return model, clipWatchedText(prompt)
}

func ExtractWatchedResponse(raw []byte, statusCode int) (responseText, errorText string) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", ""
	}
	if looksLikeSSE(raw) {
		responseText, errorText = extractWatchedSSE(raw)
	} else if gjson.ValidBytes(raw) {
		root := gjson.ParseBytes(raw)
		errorText = firstWatchedNonEmpty(
			root.Get("error.message").String(),
			root.Get("error").String(),
			root.Get("message").String(),
		)
		responseText = firstWatchedNonEmpty(
			flattenWatchedContent(root.Get("choices.0.message.content")),
			root.Get("choices.0.text").String(),
			flattenWatchedContent(root.Get("output")),
			root.Get("output_text").String(),
			flattenWatchedContent(root.Get("content")),
		)
		if responseText == "" {
			responseText = string(raw)
		}
	} else {
		responseText = string(raw)
	}
	if statusCode >= 400 && errorText == "" {
		errorText = responseText
	}
	return clipWatchedText(strings.TrimSpace(responseText)), clipWatchedText(strings.TrimSpace(errorText))
}

func looksLikeSSE(raw []byte) bool {
	return bytes.Contains(raw, []byte("data:")) || bytes.Contains(raw, []byte("event:"))
}

func extractWatchedSSE(raw []byte) (responseText, errorText string) {
	var text strings.Builder
	for _, line := range bytes.Split(raw, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || bytes.Equal(line, []byte("data: [DONE]")) {
			continue
		}
		payload := line
		if bytes.HasPrefix(payload, []byte("data:")) {
			payload = bytes.TrimSpace(payload[5:])
		} else if bytes.HasPrefix(payload, []byte("event:")) {
			continue
		}
		if len(payload) == 0 || !gjson.ValidBytes(payload) {
			continue
		}
		root := gjson.ParseBytes(payload)
		if msg := firstWatchedNonEmpty(
			root.Get("error.message").String(),
			root.Get("error").String(),
			root.Get("response.error.message").String(),
		); msg != "" {
			errorText = msg
			continue
		}
		chunk := firstWatchedNonEmpty(
			root.Get("choices.0.delta.content").String(),
			root.Get("choices.0.text").String(),
			root.Get("delta.text").String(),
			root.Get("delta").String(),
			root.Get("text").String(),
		)
		if chunk != "" && chunk[0] != '{' && chunk[0] != '[' {
			text.WriteString(chunk)
		}
	}
	responseText = text.String()
	if responseText == "" && errorText == "" {
		responseText = string(raw)
	}
	return responseText, errorText
}

func flattenWatchedContent(value gjson.Result) string {
	if !value.Exists() {
		return ""
	}
	if value.Type == gjson.String {
		return strings.TrimSpace(value.String())
	}
	if value.IsArray() {
		parts := make([]string, 0, 4)
		value.ForEach(func(_, item gjson.Result) bool {
			if text := flattenWatchedContent(item); text != "" {
				parts = append(parts, text)
			}
			return true
		})
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	if value.IsObject() {
		return firstWatchedNonEmpty(
			strings.TrimSpace(value.Get("text").String()),
			strings.TrimSpace(value.Get("content").String()),
			flattenWatchedContent(value.Get("parts")),
		)
	}
	return strings.TrimSpace(value.String())
}

func firstWatchedNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
