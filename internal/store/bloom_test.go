package store

import (
	"testing"
)

func TestBloomFilter_Basic(t *testing.T) {
	bf := NewBloomFilter(1000, 3)

	item := "https://google.com"
	if bf.Contains(item) {
		t.Error("expected empty filter to not contain item")
	}

	bf.Add(item)
	if !bf.Contains(item) {
		t.Error("expected filter to contain added item")
	}

	if bf.Contains("https://yahoo.com") {
		t.Log("Note: False positive hit")
	}
}
