package cache

import "testing"

func TestBadgerCache_Has(t *testing.T) {
	err := testBadgerCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	inCache, err := testBadgerCache.Has("foo")
	if err != nil {
		t.Error(err)
	}

	if inCache {
		t.Error("foo should not be in cache")
	}

	err = testBadgerCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	inCache, err = testBadgerCache.Has("foo")
	if err != nil {
		t.Error(err)
	}

	if !inCache {
		t.Error("foo should be in cache")
	}
}

func TestBadgerCache_Get(t *testing.T) {
	err := testBadgerCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	_, err = testBadgerCache.Get("foo")
	if err == nil {
		t.Error("foo should not be in cache")
	}

	err = testBadgerCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	val, err := testBadgerCache.Get("foo")
	if err != nil {
		t.Error(err)
	}

	if val != "bar" {
		t.Error("foo should be bar")
	}
}

func TestBadgerCache_Set(t *testing.T) {
	err := testBadgerCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	err = testBadgerCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	val, err := testBadgerCache.Get("foo")
	if err != nil {
		t.Error(err)
	}

	if val != "bar" {
		t.Error("foo should be bar")
	}

	// Test setting with expiration
	err = testBadgerCache.Set("time", "exp", 1)
	if err != nil {
		t.Error(err)
	}

	val, err = testBadgerCache.Get("time")
	if err != nil {
		t.Error(err)
	}

	if val != "exp" {
		t.Error("time should be exp")
	}
}

func TestBadgerCache_Forget(t *testing.T) {
	err := testBadgerCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	err = testBadgerCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	_, err = testBadgerCache.Get("foo")
	if err == nil {
		t.Error("foo should not be in cache")
	}
}

func TestBadgerCache_Flush(t *testing.T) {
	err := testBadgerCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	err = testBadgerCache.Flush()
	if err != nil {
		t.Error(err)
	}

	_, err = testBadgerCache.Get("foo")
	if err == nil {
		t.Error("foo should not be in cache")
	}
}

func TestBadgerCache_EmptyByMatch(t *testing.T) {
	err := testBadgerCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	err = testBadgerCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	err = testBadgerCache.Set("bar", "baz")
	if err != nil {
		t.Error(err)
	}

	err = testBadgerCache.Set("foo:bar", "baz")
	if err != nil {
		t.Error(err)
	}

	err = testBadgerCache.EmptyByMatch("foo*")
	if err != nil {
		t.Error(err)
	}

	_, err = testBadgerCache.Get("foo")
	if err == nil {
		t.Error("foo should not be in cache")
	}

	_, err = testBadgerCache.Get("foo:bar")
	if err == nil {
		t.Error("foo:bar should not be in cache")
	}

	_, err = testBadgerCache.Get("bar")
	if err != nil {
		t.Error("bar should be in cache")
	}
}
