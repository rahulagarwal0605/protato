package utils

import (
	"testing"
)

func TestStringSliceToMap(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  map[string]bool
	}{
		{
			name:  "empty slice",
			items: []string{},
			want:  map[string]bool{},
		},
		{
			name:  "single item",
			items: []string{"item1"},
			want:  map[string]bool{"item1": true},
		},
		{
			name:  "multiple items",
			items: []string{"item1", "item2", "item3"},
			want:  map[string]bool{"item1": true, "item2": true, "item3": true},
		},
		{
			name:  "duplicate items",
			items: []string{"item1", "item2", "item1"},
			want:  map[string]bool{"item1": true, "item2": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringSliceToMap(tt.items)
			if len(got) != len(tt.want) {
				t.Errorf("StringSliceToMap() length = %v, want %v", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("StringSliceToMap() [%s] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestDeduplicate(t *testing.T) {
	type item struct {
		id   string
		name string
	}

	tests := []struct {
		name     string
		items    []item
		keyFunc  func(item) string
		wantLen  int
	}{
		{
			name:    "no duplicates",
			items:   []item{{id: "1", name: "a"}, {id: "2", name: "b"}},
			keyFunc: func(i item) string { return i.id },
			wantLen: 2,
		},
		{
			name:    "with duplicates",
			items:   []item{{id: "1", name: "a"}, {id: "1", name: "b"}, {id: "2", name: "c"}},
			keyFunc: func(i item) string { return i.id },
			wantLen: 2,
		},
		{
			name:    "all duplicates",
			items:   []item{{id: "1", name: "a"}, {id: "1", name: "b"}, {id: "1", name: "c"}},
			keyFunc: func(i item) string { return i.id },
			wantLen: 1,
		},
		{
			name:    "empty slice",
			items:   []item{},
			keyFunc: func(i item) string { return i.id },
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Deduplicate(tt.items, tt.keyFunc)
			if len(got) != tt.wantLen {
				t.Errorf("Deduplicate() length = %v, want %v", len(got), tt.wantLen)
			}
		})
	}
}

func TestMergeStringSlice(t *testing.T) {
	tests := []struct {
		name      string
		existing  []string
		newItems  []string
		wantCount int
	}{
		{
			name:      "merge with no duplicates",
			existing:  []string{"a", "b"},
			newItems:  []string{"c", "d"},
			wantCount: 4,
		},
		{
			name:      "merge with duplicates",
			existing:  []string{"a", "b"},
			newItems:  []string{"b", "c"},
			wantCount: 3,
		},
		{
			name:      "empty existing",
			existing:  []string{},
			newItems:  []string{"a", "b"},
			wantCount: 2,
		},
		{
			name:      "empty new items",
			existing:  []string{"a", "b"},
			newItems:  []string{},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeStringSlice(tt.existing, tt.newItems)
			if len(got) != tt.wantCount {
				t.Errorf("MergeStringSlice() length = %v, want %v", len(got), tt.wantCount)
			}
			gotMap := StringSliceToMap(got)
			for _, item := range tt.existing {
				if !gotMap[item] {
					t.Errorf("MergeStringSlice() missing existing item: %s", item)
				}
			}
			for _, item := range tt.newItems {
				if !gotMap[item] {
					t.Errorf("MergeStringSlice() missing new item: %s", item)
				}
			}
		})
	}
}

func TestConvertSlice(t *testing.T) {
	tests := []struct {
		name     string
		items    []string
		converter func(string) int
		want     []int
	}{
		{
			name:     "convert strings to ints",
			items:    []string{"1", "2", "3"},
			converter: func(s string) int { return len(s) },
			want:     []int{1, 1, 1},
		},
		{
			name:     "empty slice",
			items:    []string{},
			converter: func(s string) int { return len(s) },
			want:     []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertSlice(tt.items, tt.converter)
			if len(got) != len(tt.want) {
				t.Errorf("ConvertSlice() length = %v, want %v", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ConvertSlice()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBuildFileSet(t *testing.T) {
	type file struct {
		path string
		size int64
	}

	tests := []struct {
		name     string
		files    []file
		getPath  func(file) string
		wantSize int
	}{
		{
			name:     "build file set",
			files:    []file{{path: "a.proto", size: 100}, {path: "b.proto", size: 200}},
			getPath:  func(f file) string { return f.path },
			wantSize: 2,
		},
		{
			name:     "empty files",
			files:    []file{},
			getPath:  func(f file) string { return f.path },
			wantSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildFileSet(tt.files, tt.getPath)
			if len(got) != tt.wantSize {
				t.Errorf("BuildFileSet() size = %v, want %v", len(got), tt.wantSize)
			}
		})
	}
}

func TestSliceToMapWithValue(t *testing.T) {
	type file struct {
		path string
		hash string
	}

	tests := []struct {
		name     string
		items    []file
		keyFunc  func(file) string
		valueFunc func(file) string
		wantSize int
	}{
		{
			name:     "build map with values",
			items:    []file{{path: "a.proto", hash: "hash1"}, {path: "b.proto", hash: "hash2"}},
			keyFunc:  func(f file) string { return f.path },
			valueFunc: func(f file) string { return f.hash },
			wantSize: 2,
		},
		{
			name:     "empty items",
			items:    []file{},
			keyFunc:  func(f file) string { return f.path },
			valueFunc: func(f file) string { return f.hash },
			wantSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SliceToMapWithValue(tt.items, tt.keyFunc, tt.valueFunc)
			if len(got) != tt.wantSize {
				t.Errorf("SliceToMapWithValue() size = %v, want %v", len(got), tt.wantSize)
			}
			for _, item := range tt.items {
				if val, ok := got[tt.keyFunc(item)]; ok {
					if val != tt.valueFunc(item) {
						t.Errorf("SliceToMapWithValue() value mismatch for %s", tt.keyFunc(item))
					}
				}
			}
		})
	}
}
