package service

import (
	"math"
	"strconv"
	"strings"
)

// normalizeString trims and lowercases for case-insensitive comparison.
func normalizeString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// normalizeKode uppercases after trim.
func normalizeKode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// normalizeTime normalizes time strings: "08:00:00" and "08:00" both become "08:00".
func normalizeTime(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Split by colon and take only HH:MM
	parts := strings.Split(s, ":")
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return s
}

// isEmpty checks if a value is empty. "0" is empty only on non-SKS/mutu/IPK/no fields.
func isEmpty(s string, isIntField bool) bool {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return true
	}
	if !isIntField && s == "0" {
		return true
	}
	return false
}

// normalizeIntString normalizes an integer string for comparison.
func normalizeIntString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return "0"
	}
	return s
}

// ipkEqual checks if two IPK values are equal within tolerance 0.01.
func ipkEqual(a, b float64) bool {
	return math.Abs(a-b) <= 0.01
}

// parseFloat parses a float from string, treating empty/null as 0.
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// parseInt parses an int from string, treating empty/null as 0.
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}
