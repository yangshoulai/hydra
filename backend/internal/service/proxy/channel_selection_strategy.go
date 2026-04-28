package proxy

import (
	"sort"
	"sync"

	"github.com/yangshoulai/hydra/internal/models"
)

type channelSelectionContext struct {
	Model         string
	EndpointType  string
	IsStream      bool
	LastChannelID uint
}

func (c channelSelectionContext) routeKey() string {
	streamFlag := "0"
	if c.IsStream {
		streamFlag = "1"
	}
	return c.Model + "|" + c.EndpointType + "|" + streamFlag
}

type channelSelectionStrategy interface {
	Name() string
	Select(candidates []*channelRouteCandidate, ctx channelSelectionContext) *channelRouteCandidate
}

type weightedRandomChannelSelectionStrategy struct{}

func (s *weightedRandomChannelSelectionStrategy) Name() string {
	return models.ProxyLoadBalanceStrategyWeightedRandom
}

func (s *weightedRandomChannelSelectionStrategy) Select(
	candidates []*channelRouteCandidate,
	_ channelSelectionContext,
) *channelRouteCandidate {
	if len(candidates) == 0 {
		return nil
	}

	weights := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		weights = append(weights, channelWeight(candidate.channel))
	}

	selectedIndex := weightedRandomIndex(weights)
	if selectedIndex < 0 || selectedIndex >= len(candidates) {
		return nil
	}

	return candidates[selectedIndex]
}

type roundRobinChannelSelectionStrategy struct {
	mu       sync.Mutex
	counters map[string]uint64
}

func newRoundRobinChannelSelectionStrategy() *roundRobinChannelSelectionStrategy {
	return &roundRobinChannelSelectionStrategy{
		counters: make(map[string]uint64),
	}
}

func (s *roundRobinChannelSelectionStrategy) Name() string {
	return models.ProxyLoadBalanceStrategyRoundRobin
}

func (s *roundRobinChannelSelectionStrategy) Select(
	candidates []*channelRouteCandidate,
	ctx channelSelectionContext,
) *channelRouteCandidate {
	orderedCandidates := sortChannelRouteCandidatesByID(candidates)
	if len(orderedCandidates) == 0 {
		return nil
	}

	// 重试时优先顺着上一次失败渠道的后继继续尝试，避免同一请求内跳号。
	if ctx.LastChannelID != 0 {
		return nextChannelRouteCandidate(orderedCandidates, ctx.LastChannelID)
	}

	routeKey := ctx.routeKey()

	s.mu.Lock()
	counter := s.counters[routeKey]
	s.counters[routeKey] = counter + 1
	s.mu.Unlock()

	return orderedCandidates[int(counter%uint64(len(orderedCandidates)))]
}

func newChannelSelectionStrategy(strategyName string) channelSelectionStrategy {
	switch normalizeChannelSelectionStrategyName(strategyName) {
	case models.ProxyLoadBalanceStrategyRoundRobin:
		return newRoundRobinChannelSelectionStrategy()
	default:
		return &weightedRandomChannelSelectionStrategy{}
	}
}

func normalizeChannelSelectionStrategyName(strategyName string) string {
	switch strategyName {
	case models.ProxyLoadBalanceStrategyRoundRobin:
		return models.ProxyLoadBalanceStrategyRoundRobin
	default:
		return models.ProxyLoadBalanceStrategyWeightedRandom
	}
}

func sortChannelRouteCandidatesByID(candidates []*channelRouteCandidate) []*channelRouteCandidate {
	orderedCandidates := make([]*channelRouteCandidate, len(candidates))
	copy(orderedCandidates, candidates)
	sort.SliceStable(orderedCandidates, func(i, j int) bool {
		return orderedCandidates[i].channel.ID < orderedCandidates[j].channel.ID
	})
	return orderedCandidates
}

func nextChannelRouteCandidate(candidates []*channelRouteCandidate, lastChannelID uint) *channelRouteCandidate {
	for _, candidate := range candidates {
		if candidate.channel.ID > lastChannelID {
			return candidate
		}
	}
	return candidates[0]
}
