package proxy

import (
	"testing"

	"github.com/yangshoulai/hydra/internal/models"
)

func TestWeightedIndexForDraw(t *testing.T) {
	t.Parallel()

	weights := []int64{10, 20, 70}
	testCases := []struct {
		draw int64
		want int
	}{
		{draw: 0, want: 0},
		{draw: 9, want: 0},
		{draw: 10, want: 1},
		{draw: 29, want: 1},
		{draw: 30, want: 2},
		{draw: 99, want: 2},
	}

	for _, tc := range testCases {
		if got := weightedIndexForDraw(weights, tc.draw); got != tc.want {
			t.Fatalf("draw=%d: got %d, want %d", tc.draw, got, tc.want)
		}
	}
}

func TestFilterConfigsWithAvailableKeys(t *testing.T) {
	t.Parallel()

	configs := []models.ChannelModelConfig{
		{ID: 1, KeyGroups: models.KeyGroups{"alpha"}},
		{ID: 2, KeyGroups: models.KeyGroups{"beta", "gamma"}},
		{ID: 3, KeyGroups: models.KeyGroups{"delta"}},
	}
	keys := []models.ChannelKey{
		{ID: 11, ChannelKeyGroup: "beta"},
		{ID: 12, ChannelKeyGroup: "omega"},
	}

	filtered := filterConfigsWithAvailableKeys(configs, keys)
	if len(filtered) != 1 {
		t.Fatalf("unexpected filtered config count: got %d, want 1", len(filtered))
	}
	if filtered[0].ID != 2 {
		t.Fatalf("unexpected config selected: got %d, want 2", filtered[0].ID)
	}
}
