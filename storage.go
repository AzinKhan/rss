package rss

import (
	"encoding/xml"
	"os"
	"path"
)

func Store(feed *Feed, folder string) error {
	filename := path.Join(folder, feed.Channel.Title)

	// Try to load the file if it exists

	_, err := os.Stat(filename)
	if !os.IsNotExist(err) {
		// File exists so load it and merge incoming
		existing, err := Load(filename)
		if err != nil {
			return err
		}

		feed = merge(existing, feed)

	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	b, err := xml.Marshal(feed.RSS)
	if err != nil {
		return err
	}

	return os.WriteFile(filename, b, 0666)
}

func LoadAll(folder string) ([]*Feed, error) {
	files, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}

	result := make([]*Feed, len(files))
	for i, f := range files {
		feed, err := Load(path.Join(folder, f.Name()))
		if err != nil {
			return nil, err
		}
		result[i] = feed
	}
	return result, nil
}

func Load(filename string) (*Feed, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var feed Feed
	err = xml.Unmarshal(b, &feed)
	if err != nil {
		return nil, err
	}
	return &feed, nil
}

func merge(a, b *Feed) *Feed {
	itemsA := hashItems(a.Channel.Items)
	itemsB := hashItems(b.Channel.Items)

	toAppend := make([]Item, 0, len(itemsB))
	for key, item := range itemsB {
		_, ok := itemsA[key]
		if ok {
			continue
		}
		toAppend = append(toAppend, item)
	}

	a.Channel.Items = append(a.Channel.Items, toAppend...)
	return a
}

func hashItems(items []Item) map[string]Item {
	result := make(map[string]Item)
	for _, item := range items {
		key := item.Title + item.PubDate
		result[key] = item
	}
	return result
}
