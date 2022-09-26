package rss

import (
	"fmt"
	"strings"
)

type colourizer interface {
	colourize(string, Colour) string
}

type colourizeFunc func(string, Colour) string

func (cf colourizeFunc) colourize(s string, c Colour) string {
	return cf(s, c)
}

type stringWriter interface {
	WriteString(string) (int, error)
}

type formatOption func(*formatSettings)

func includeLinks(include bool) formatOption {
	return func(fs *formatSettings) {
		fs.includeLinks = include
	}
}

func setColourizer(c colourizer) formatOption {
	return func(fs *formatSettings) {
		fs.colourizer = c
	}
}

type formatSettings struct {
	title        Colour
	date         Colour
	link         Colour
	colourizer   colourizer
	includeLinks bool
}

func formatFeed(fi FeedItem, opts ...formatOption) string {
	// Set some defaults
	settings := &formatSettings{
		title:        green,
		date:         yellow,
		link:         blue,
		colourizer:   colourizeFunc(colourize),
		includeLinks: false,
	}
	for _, opt := range opts {
		opt(settings)
	}

	c := settings.colourizer

	builder := &strings.Builder{}
	if !fi.PublishTime.IsZero() {
		date := fi.PublishTime.Format(outputTimeLayout)
		builder.WriteString(fmt.Sprintf("%s:", c.colourize(date, settings.date)))
	}

	if len(fi.Links) == 0 && len(fi.Title) != 0 {
		// This is one of the items acting as a title card for the feed so
		// colour its title green.
		fi.Title = c.colourize(fi.Title, settings.title)
	}
	builder.WriteString(fmt.Sprintf("\t%s", fi.Title))
	if settings.includeLinks {
		for _, link := range fi.Links {
			builder.WriteString(fmt.Sprintf("\t%s", colourize(link, blue)))
		}
	}
	builder.WriteString("\n")
	return builder.String()
}

func formatFeedInteractive(fi FeedItem) string {
	return formatFeed(fi, setColourizer(colourizeFunc(colourizeInteractive)))
}

func colourize(text string, c Colour) string {
	return fmt.Sprintf("%s%s%s", c, text, reset)
}

func colourizeInteractive(text string, c Colour) string {
	var b string
	switch c {
	case red:
		b = "red"
	case green:
		b = "green"
	case yellow:
		b = "yellow"
	case blue:
		b = "blue"
	case purple:
		b = "purple"
	case cyan:
		b = "cyan"
	case gray:
		b = "gray"
	case white:
		b = "white"
	}
	return fmt.Sprintf("[%s]%s[white]", b, text)
}
