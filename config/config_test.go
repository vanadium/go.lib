package config

import (
	"testing"
)

func checkPresent(t *testing.T, c Config, k, wantV string) {
	if v, err := c.Get(k); err != nil {
		t.Errorf("Expected value %q for key %q, got error %v instead", wantV, k, err)
	} else if v != wantV {
		t.Errorf("Expected value %q for key %q, got %q instead", wantV, k, v)
	}
}

func checkAbsent(t *testing.T, c Config, k string) {
	if v, err := c.Get(k); err != ErrKeyNotFound {
		t.Errorf("Expected (\"\", %v) for key %q, got (%q, %v) instead", ErrKeyNotFound, k, v, err)
	}
}

// TestConfig checks that Set and Get work as expected.
func TestConfig(t *testing.T) {
	c := New()
	c.Set("foo", "bar")
	checkPresent(t, c, "foo", "bar")
	checkAbsent(t, c, "food")
	c.Set("foo", "baz")
	checkPresent(t, c, "foo", "baz")
}

// TestSerialize checks that serializing the config and merging from a
// serialized config work as expected.
func TestSerialize(t *testing.T) {
	c := New()
	c.Set("k1", "v1")
	c.Set("k2", "v2")
	s, err := c.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}
	readC := New()
	if err := readC.MergeFrom(s); err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}
	checkPresent(t, readC, "k1", "v1")
	checkPresent(t, readC, "k2", "v2")

	readC.Set("k2", "newv2") // This should be overwritten by the next merge.
	checkPresent(t, readC, "k2", "newv2")
	readC.Set("k3", "v3") // This should survive the next merge.

	c.Set("k1", "newv1") // This should overwrite v1 in the next merge.
	c.Set("k4", "v4")    // This should be added following the next merge.
	s, err = c.Serialize()
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}
	if err := readC.MergeFrom(s); err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}
	checkPresent(t, readC, "k1", "newv1")
	checkPresent(t, readC, "k2", "v2")
	checkPresent(t, readC, "k3", "v3")
	checkPresent(t, readC, "k4", "v4")
}
