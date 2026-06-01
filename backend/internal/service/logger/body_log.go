package logger

import (
	"fmt"
	"log/slog"
	"regexp"
)

const maxResponseBodyLogRunes = 4096

var (
	b64JSONFieldPattern    = regexp.MustCompile(`(?i)("b64_json"\s*:\s*")([^"]*)(")`)
	dataImageBase64Pattern = regexp.MustCompile(`(?i)(data:image/[a-z0-9.+-]+;base64,)([a-z0-9+/=_-]{128,})`)
)

// ResponseBodyLogAttrs 生成适合写入文件日志的响应体字段。
// 注意：这里仅用于 slog 文件日志，不能替代调试模式下的请求日志落库原文。
func ResponseBodyLogAttrs(body []byte) []any {
	preview, truncated, redacted := SafeResponseBodyForLog(body)
	return []any{
		slog.Int("response_body_size", len(body)),
		slog.Bool("response_body_truncated", truncated),
		slog.Bool("response_body_redacted", redacted),
		slog.String("response_body", preview),
	}
}

// SafeResponseBodyForLog 对响应体做日志安全化处理：
//  1. 脱敏图片 base64 字段，避免图片内容写入 hydra.log；
//  2. 对其他大响应做固定长度截断，避免单行日志过大。
func SafeResponseBodyForLog(body []byte) (preview string, truncated bool, redacted bool) {
	if len(body) == 0 {
		return "", false, false
	}

	text := string(body)
	text, redacted = redactImageBase64ForLog(text)

	truncatedText, wasTruncated := truncateRunes(text, maxResponseBodyLogRunes)
	if wasTruncated {
		truncated = true
		truncatedText += fmt.Sprintf("...[truncated, original_bytes=%d]", len(body))
	}

	return truncatedText, truncated, redacted
}

func redactImageBase64ForLog(text string) (string, bool) {
	redacted := false

	text = b64JSONFieldPattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := b64JSONFieldPattern.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		redacted = true
		return parts[1] + fmt.Sprintf("[omitted base64 image, chars=%d]", len(parts[2])) + parts[3]
	})

	text = dataImageBase64Pattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := dataImageBase64Pattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		redacted = true
		return parts[1] + fmt.Sprintf("[omitted base64 image, chars=%d]", len(parts[2]))
	})

	return text, redacted
}

func truncateRunes(text string, maxRunes int) (string, bool) {
	if maxRunes <= 0 {
		return "", text != ""
	}

	count := 0
	for idx := range text {
		if count == maxRunes {
			return text[:idx], true
		}
		count++
	}
	return text, false
}
