package rss

import (
	"testing"
	"time"
)

func TestFiltersApplyOldestItem(t *testing.T) {
	now := time.Now()
	testcases := []struct {
		name     string
		item     FeedItem
		filters  Filters
		expected bool
	}{
		{

			name: "Filter out older item",
			item: FeedItem{
				Title:       "Title",
				PublishTime: now.Add(-36 * time.Hour),
				Feed:        "Feed",
				Channel:     "Channel",
			},
			filters:  Filters{OldestItem(24 * time.Hour)},
			expected: false,
		},
		{
			name: "Keep newer item",
			item: FeedItem{
				Title:       "Title",
				PublishTime: now.Add(-36 * time.Hour),
				Feed:        "Feed",
				Channel:     "Channel",
			},
			filters:  Filters{OldestItem(40 * time.Hour)},
			expected: true,
		},
	}

	t.Parallel()
	for _, tc := range testcases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			result := tc.filters.Apply(tc.item)
			if result != tc.expected {
				t.Fail()
			}
		})

	}
}
