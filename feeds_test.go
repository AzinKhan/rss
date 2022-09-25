package rss

import (
	"reflect"
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
			assertEqual(t, tc.expected, result)
		})

	}
}

func TestFilterMultiple(t *testing.T) {
	type testcase struct {
		item     FeedItem
		expected bool
	}
	testcases := []struct {
		name   string
		filter Filter
		cases  []testcase
	}{
		{
			name:   "Max items",
			filter: MaxItems(2),
			cases: []testcase{
				{
					expected: true,
					item:     FeedItem{},
				},
				{
					expected: true,
					item:     FeedItem{},
				},
				{
					expected: false,
					item:     FeedItem{},
				},
				{
					expected: false,
					item:     FeedItem{},
				},
			},
		},
		{
			name:   "Max items per channel",
			filter: MaxItemsPerChannel(1),
			cases: []testcase{
				{
					expected: true,
					item: FeedItem{
						Channel: "1",
					},
				},
				{
					expected: true,
					item: FeedItem{
						Channel: "2",
					},
				},
				{
					expected: false,
					item: FeedItem{
						Channel: "1",
					},
				},
				{
					expected: false,
					item: FeedItem{
						Channel: "2",
					},
				},
			},
		},
		{
			name:   "Deduplicate",
			filter: Deduplicate(),
			cases: []testcase{
				{
					expected: true,
					item: FeedItem{
						Links: []string{"link1"},
					},
				},
				{
					expected: false,
					item: FeedItem{
						Links: []string{"link1"},
					},
				},
				{
					expected: false,
					item: FeedItem{
						Links: []string{"link1"},
					},
				},
				{
					expected: false,
					item: FeedItem{
						Links: []string{"link1"},
					},
				},
			},
		},
	}

	t.Parallel()
	for _, tc := range testcases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			for _, c := range tc.cases {
				result := tc.filter(c.item)
				assertEqual(t, c.expected, result)
			}
		})

	}
}

func assertEqual(t *testing.T, expected interface{}, result interface{}) {
	if reflect.DeepEqual(expected, result) {
		return
	}
	t.Fail()
	t.Logf("Expected %v, got %v", expected, result)
}
