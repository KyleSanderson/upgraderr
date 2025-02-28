package main

import (
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Test String", "test string"},
		{"  Spaces  ", "spaces"},
		{"UPPERCASE", "uppercase"},
		{"mixed CASE", "mixed case"},
		{"", ""},
		{" \t\n ", ""},
		{"Invalid\xFFUTF8", "invalidutf8"},
	}

	for _, test := range tests {
		result := Normalize(test.input)
		if result != test.expected {
			t.Errorf("Normalize(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedNum    int
		expectedValid  bool
		expectedRemain string
	}{
		{"positive", "123", 123, true, ""},
		{"negative", "-123", -123, true, ""},
		{"plus_sign", "+123", 123, true, ""},
		{"with_suffix", "123abc", 123, true, "abc"},
		{"with_spaces", "  123", 123, true, ""},
		{"non_numeric", "abc", 0, false, "abc"},
		{"zero", "0", 0, true, ""},
		{"negative_zero", "-0", 0, true, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			num, valid, remain := Atoi(test.input)
			if num != test.expectedNum || valid != test.expectedValid || remain != test.expectedRemain {
				t.Errorf("Atoi(%q) = (%d, %t, %q), expected (%d, %t, %q)",
					test.input, num, valid, remain, test.expectedNum, test.expectedValid, test.expectedRemain)
			}
		})
	}
}

func TestCacheTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected struct {
			title      string
			year       int
			resolution string
			source     string
		}
	}{
		{
			"Movie.2023.1080p.WEB-DL.DDP5.1.H.264-GROUP",
			struct {
				title      string
				year       int
				resolution string
				source     string
			}{
				title:      "Movie",
				year:       2023,
				resolution: "1080p",
				source:     "WEB-DL",
			},
		},
		{
			"TV.Show.S01E01.1080p.AMZN.WEB-DL.DDP5.1.H.264-GROUP",
			struct {
				title      string
				year       int
				resolution string
				source     string
			}{
				title:      "TV Show",
				year:       0,
				resolution: "1080p",
				source:     "WEB-DL",
			},
		},
	}

	for _, test := range tests {
		result := CacheTitle(test.input)

		if result.Title != test.expected.title ||
			result.Year != test.expected.year ||
			result.Resolution != test.expected.resolution ||
			result.Source != test.expected.source {
			t.Errorf("CacheTitle(%q) produced unexpected result: got %+v, expected title=%q, year=%d, resolution=%q, source=%q",
				test.input, result, test.expected.title, test.expected.year, test.expected.resolution, test.expected.source)
		}
	}
}
