package agents

import (
	"testing"

	"github.com/voocel/agentcore"
)

func TestParseThinkingLevel(t *testing.T) {
	ok := map[string]agentcore.ThinkingLevel{
		"":        "",
		"auto":    "",
		"off":     agentcore.ThinkingOff,
		"minimal": agentcore.ThinkingMinimal,
		"low":     agentcore.ThinkingLow,
		"medium":  agentcore.ThinkingMedium,
		"high":    agentcore.ThinkingHigh,
		"xhigh":   agentcore.ThinkingXHigh,
		"max":     agentcore.ThinkingMax,
		"  HIGH ": agentcore.ThinkingHigh, // 大小写/空白归一
	}
	for in, want := range ok {
		got, err := ParseThinkingLevel(in)
		if err != nil {
			t.Errorf("ParseThinkingLevel(%q) 意外报错: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseThinkingLevel(%q) = %q, want %q", in, got, want)
		}
	}

	for _, bad := range []string{"ultra", "true", "0"} {
		if _, err := ParseThinkingLevel(bad); err == nil {
			t.Errorf("ParseThinkingLevel(%q) 应报错，实际通过", bad)
		}
	}
}
