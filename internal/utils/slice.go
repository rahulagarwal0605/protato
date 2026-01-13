package utils

// StringSliceToMap converts a string slice to a map for fast lookups.
func StringSliceToMap(items []string) map[string]bool {
	m := make(map[string]bool)
	for _, item := range items {
		m[item] = true
	}
	return m
}

// SliceToMap is a generic helper to convert slices to maps using a key function.
func SliceToMap[T any](items []T, keyFunc func(T) string) map[string]bool {
	m := make(map[string]bool)
	for _, item := range items {
		m[keyFunc(item)] = true
	}
	return m
}

// Deduplicate removes duplicate items from a slice using a key function.
func Deduplicate[T any](items []T, keyFunc func(T) string) []T {
	seen := make(map[string]bool)
	var deduplicated []T
	for _, item := range items {
		key := keyFunc(item)
		if !seen[key] {
			deduplicated = append(deduplicated, item)
			seen[key] = true
		}
	}
	return deduplicated
}

// MergeStringSlice merges new items into existing slice, avoiding duplicates.
func MergeStringSlice(existing, newItems []string) []string {
	seen := StringSliceToMap(existing)

	result := existing
	for _, item := range newItems {
		if !seen[item] {
			result = append(result, item)
			seen[item] = true
		}
	}
	return result
}

// BuildFileSet creates a set of file paths from a slice of files using a path extractor function.
func BuildFileSet[T any](files []T, getPath func(T) string) map[string]bool {
	m := make(map[string]bool)
	for _, f := range files {
		m[getPath(f)] = true
	}
	return m
}

// SliceToMapWithValue converts a slice to a map using key and value extractor functions.
// Example: SliceToMapWithValue(files, func(f File) string { return f.Path }, func(f File) Hash { return f.Hash })
func SliceToMapWithValue[K comparable, V any, T any](items []T, keyFunc func(T) K, valueFunc func(T) V) map[K]V {
	m := make(map[K]V)
	for _, item := range items {
		m[keyFunc(item)] = valueFunc(item)
	}
	return m
}

// ConvertSlice converts a slice of one type to a slice of another type using a converter function.
// Example: ConvertSlice(strings, func(s string) ProjectPath { return ProjectPath(s) })
func ConvertSlice[T any, R any](items []T, converter func(T) R) []R {
	result := make([]R, len(items))
	for i, item := range items {
		result[i] = converter(item)
	}
	return result
}
