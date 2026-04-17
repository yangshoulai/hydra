package metrics

import (
	"sort"
	"sync"
	"time"
)

const (
	recentWindow      = 60 * time.Second
	qpsTrendWindowMin = 60
	statsWindowMin    = 24 * 60
)

// RequestEvent 单次代理请求的统计事件（按最终结果记录）
type RequestEvent struct {
	Timestamp        time.Time
	Success          bool
	Model            string
	ChannelID        uint
	ChannelName      string
	PromptTokens     int64
	CompletionTokens int64
}

// QPSPoint QPS 趋势点（按分钟）
type QPSPoint struct {
	Timestamp string
	QPS       float64
}

// ChannelSnapshot 渠道维度统计快照
type ChannelSnapshot struct {
	ChannelID        uint
	ChannelName      string
	TotalRequests    int
	SuccessRequests  int
	FailedRequests   int
	PromptTokens     int64
	CompletionTokens int64
}

// ModelSnapshot 模型维度统计快照
type ModelSnapshot struct {
	ModelName       string
	TotalRequests   int
	SuccessRequests int
	FailedRequests  int
}

// Snapshot 运行时统计快照
type Snapshot struct {
	CurrentQPS            float64
	QPSTrend              []QPSPoint
	TotalRequests         int
	SuccessRequests       int
	FailedRequests        int
	SuccessRate           float64
	TotalPromptTokens     int64
	TotalCompletionTokens int64
	ChannelStats          map[uint]ChannelSnapshot
	ModelStats            map[string]ModelSnapshot
}

type channelCounter struct {
	name             string
	total            int
	success          int
	promptTokens     int64
	completionTokens int64
}

type modelCounter struct {
	total   int
	success int
}

type minuteBucket struct {
	total            int
	success          int
	promptTokens     int64
	completionTokens int64
	channels         map[uint]*channelCounter
	models           map[string]*modelCounter
}

// RuntimeMetrics 运行时统计收集器（进程内内存）
type RuntimeMetrics struct {
	mu                 sync.Mutex
	minuteBuckets      map[int64]*minuteBucket
	recentRequestTimes []time.Time
}

func NewRuntimeMetrics() *RuntimeMetrics {
	return &RuntimeMetrics{
		minuteBuckets:      make(map[int64]*minuteBucket),
		recentRequestTimes: make([]time.Time, 0, 2048),
	}
}

// Record 记录一次请求结果
func (m *RuntimeMetrics) Record(event RequestEvent) {
	now := event.Timestamp
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupOldBucketsLocked(now)
	m.cleanupRecentLocked(now)

	minuteKey := now.Unix() / 60
	bucket, exists := m.minuteBuckets[minuteKey]
	if !exists {
		bucket = &minuteBucket{
			channels: make(map[uint]*channelCounter),
			models:   make(map[string]*modelCounter),
		}
		m.minuteBuckets[minuteKey] = bucket
	}

	bucket.total++
	if event.Success {
		bucket.success++
	}
	bucket.promptTokens += event.PromptTokens
	bucket.completionTokens += event.CompletionTokens

	if event.ChannelID != 0 {
		ch, ok := bucket.channels[event.ChannelID]
		if !ok {
			ch = &channelCounter{}
			bucket.channels[event.ChannelID] = ch
		}
		if event.ChannelName != "" {
			ch.name = event.ChannelName
		}
		ch.total++
		if event.Success {
			ch.success++
		}
		ch.promptTokens += event.PromptTokens
		ch.completionTokens += event.CompletionTokens
	}

	if event.Model != "" {
		md, ok := bucket.models[event.Model]
		if !ok {
			md = &modelCounter{}
			bucket.models[event.Model] = md
		}
		md.total++
		if event.Success {
			md.success++
		}
	}

	m.recentRequestTimes = append(m.recentRequestTimes, now)
	m.cleanupRecentLocked(now)
}

// Snapshot 生成当前统计快照
func (m *RuntimeMetrics) Snapshot(now time.Time) Snapshot {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupOldBucketsLocked(now)
	m.cleanupRecentLocked(now)

	snapshot := Snapshot{
		CurrentQPS:            float64(len(m.recentRequestTimes)) / recentWindow.Seconds(),
		QPSTrend:              make([]QPSPoint, 0, qpsTrendWindowMin),
		ChannelStats:          make(map[uint]ChannelSnapshot),
		ModelStats:            make(map[string]ModelSnapshot),
		TotalPromptTokens:     0,
		TotalCompletionTokens: 0,
	}

	nowMinute := now.Unix() / 60
	fromMinute := nowMinute - (qpsTrendWindowMin - 1)
	for minute := fromMinute; minute <= nowMinute; minute++ {
		count := 0
		if bucket, ok := m.minuteBuckets[minute]; ok {
			count = bucket.total
		}
		snapshot.QPSTrend = append(snapshot.QPSTrend, QPSPoint{
			Timestamp: time.Unix(minute*60, 0).Local().Format("15:04"),
			QPS:       float64(count) / 60,
		})
	}

	statsFromMinute := nowMinute - (statsWindowMin - 1)
	for minute, bucket := range m.minuteBuckets {
		if minute < statsFromMinute {
			continue
		}

		snapshot.TotalRequests += bucket.total
		snapshot.SuccessRequests += bucket.success
		snapshot.FailedRequests += bucket.total - bucket.success
		snapshot.TotalPromptTokens += bucket.promptTokens
		snapshot.TotalCompletionTokens += bucket.completionTokens

		for channelID, ch := range bucket.channels {
			existing := snapshot.ChannelStats[channelID]
			if ch.name != "" {
				existing.ChannelName = ch.name
			}
			existing.ChannelID = channelID
			existing.TotalRequests += ch.total
			existing.SuccessRequests += ch.success
			existing.FailedRequests += ch.total - ch.success
			existing.PromptTokens += ch.promptTokens
			existing.CompletionTokens += ch.completionTokens
			snapshot.ChannelStats[channelID] = existing
		}

		for modelName, model := range bucket.models {
			existing := snapshot.ModelStats[modelName]
			existing.ModelName = modelName
			existing.TotalRequests += model.total
			existing.SuccessRequests += model.success
			existing.FailedRequests += model.total - model.success
			snapshot.ModelStats[modelName] = existing
		}
	}

	if snapshot.TotalRequests > 0 {
		snapshot.SuccessRate = float64(snapshot.SuccessRequests) / float64(snapshot.TotalRequests) * 100
	}

	return snapshot
}

func (m *RuntimeMetrics) cleanupOldBucketsLocked(now time.Time) {
	minMinute := now.UTC().Unix()/60 - (statsWindowMin + 5)
	for minute := range m.minuteBuckets {
		if minute < minMinute {
			delete(m.minuteBuckets, minute)
		}
	}
}

func (m *RuntimeMetrics) cleanupRecentLocked(now time.Time) {
	if len(m.recentRequestTimes) == 0 {
		return
	}

	threshold := now.Add(-recentWindow)
	idx := sort.Search(len(m.recentRequestTimes), func(i int) bool {
		return !m.recentRequestTimes[i].Before(threshold)
	})
	if idx <= 0 {
		return
	}

	m.recentRequestTimes = append(m.recentRequestTimes[:0], m.recentRequestTimes[idx:]...)
}
