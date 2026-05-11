package notification

import (
	"strings"
	"testing"
	"time"
)

func TestEscapeTelegramMarkdownV2(t *testing.T) {
	input := `_ * [ ] ( ) ~ ` + "`" + ` > # + - = | { } . ! \`
	want := `\_ \* \[ \] \( \) \~ \` + "`" + ` \> \# \+ \- \= \| \{ \} \. \! \\`

	if got := escapeTelegramMarkdownV2(input); got != want {
		t.Fatalf("escapeTelegramMarkdownV2() = %q, want %q", got, want)
	}
}

func TestFormatMessageUsesMarkdownV2Escaping(t *testing.T) {
	msg := formatMessage(Event{
		Type:      "test",
		Title:     "测试_通知[v2]!",
		CreatedAt: time.Date(2026, 5, 11, 18, 57, 42, 0, time.FixedZone("CST", 8*3600)),
		Fields: []Field{
			{Name: "Trace.ID", Value: "abc-123_def!"},
			{Name: "错误", Value: "HTTP 403: bot can't send messages to the bot."},
		},
	})

	expectedFragments := []string{
		"🔔 *Hydra 通知*",
		"📌 *事件*：测试\\_通知\\[v2\\]\\!",
		"🕒 *时间*：2026\\-05\\-11 18:57:42",
		"• *Trace\\.ID*：abc\\-123\\_def\\!",
		"HTTP 403: bot can't send messages to the bot\\.",
	}
	for _, fragment := range expectedFragments {
		if !strings.Contains(msg, fragment) {
			t.Fatalf("formatMessage() missing %q in:\n%s", fragment, msg)
		}
	}
}
