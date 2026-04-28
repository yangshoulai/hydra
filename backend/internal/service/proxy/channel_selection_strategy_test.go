package proxy

import (
	"testing"

	"github.com/yangshoulai/hydra/internal/models"
)

func TestRoundRobinChannelSelectionStrategySelectCyclesByRouteKey(t *testing.T) {
	strategy := newRoundRobinChannelSelectionStrategy()
	candidates := []*channelRouteCandidate{
		newChannelRouteCandidateForTest(10, 100),
		newChannelRouteCandidateForTest(3, 100),
		newChannelRouteCandidateForTest(7, 100),
	}
	ctx := channelSelectionContext{
		Model:        "gpt-4o",
		EndpointType: "OpenAIChatCompletions",
	}

	expectedChannelIDs := []uint{3, 7, 10, 3}
	for index, expectedChannelID := range expectedChannelIDs {
		selectedCandidate := strategy.Select(candidates, ctx)
		if selectedCandidate == nil {
			t.Fatalf("第 %d 次选择返回 nil", index+1)
		}
		if selectedCandidate.channel.ID != expectedChannelID {
			t.Fatalf("第 %d 次选择错误，期望渠道 %d，实际渠道 %d", index+1, expectedChannelID, selectedCandidate.channel.ID)
		}
	}
}

func TestRoundRobinChannelSelectionStrategySelectRetryUsesNextAvailableCandidate(t *testing.T) {
	strategy := newRoundRobinChannelSelectionStrategy()
	candidates := []*channelRouteCandidate{
		newChannelRouteCandidateForTest(2, 100),
		newChannelRouteCandidateForTest(9, 100),
		newChannelRouteCandidateForTest(5, 100),
	}

	selectedCandidate := strategy.Select(candidates, channelSelectionContext{
		Model:         "gpt-4o",
		EndpointType:  "OpenAIChatCompletions",
		LastChannelID: 5,
	})
	if selectedCandidate == nil {
		t.Fatal("重试选择返回 nil")
	}
	if selectedCandidate.channel.ID != 9 {
		t.Fatalf("重试后应选择下一个可用渠道，期望渠道 9，实际渠道 %d", selectedCandidate.channel.ID)
	}

	wrappedCandidate := strategy.Select(candidates[:2], channelSelectionContext{
		Model:         "gpt-4o",
		EndpointType:  "OpenAIChatCompletions",
		LastChannelID: 9,
	})
	if wrappedCandidate == nil {
		t.Fatal("重试回绕选择返回 nil")
	}
	if wrappedCandidate.channel.ID != 2 {
		t.Fatalf("重试回绕后应选择最小渠道 ID，期望渠道 2，实际渠道 %d", wrappedCandidate.channel.ID)
	}
}

func TestWeightedRandomChannelSelectionStrategySelectSingleCandidate(t *testing.T) {
	strategy := &weightedRandomChannelSelectionStrategy{}
	candidate := newChannelRouteCandidateForTest(11, 200)

	selectedCandidate := strategy.Select([]*channelRouteCandidate{candidate}, channelSelectionContext{})
	if selectedCandidate == nil {
		t.Fatal("单候选选择返回 nil")
	}
	if selectedCandidate.channel.ID != candidate.channel.ID {
		t.Fatalf("单候选选择错误，期望渠道 %d，实际渠道 %d", candidate.channel.ID, selectedCandidate.channel.ID)
	}
}

func TestNormalizeChannelSelectionStrategyNameDefaultsToWeightedRandom(t *testing.T) {
	if got := normalizeChannelSelectionStrategyName("unexpected"); got != models.ProxyLoadBalanceStrategyWeightedRandom {
		t.Fatalf("未知策略应回退到加权随机，实际值 %s", got)
	}
	if got := normalizeChannelSelectionStrategyName(models.ProxyLoadBalanceStrategyRoundRobin); got != models.ProxyLoadBalanceStrategyRoundRobin {
		t.Fatalf("轮询策略归一化错误，实际值 %s", got)
	}
}

func newChannelRouteCandidateForTest(channelID uint, weight int) *channelRouteCandidate {
	return &channelRouteCandidate{
		channel: &models.Channel{
			ID:     channelID,
			Weight: weight,
		},
	}
}
