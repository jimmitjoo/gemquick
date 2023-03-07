package cache

import "testing"

func TestRedisCache_Has(t *testing.T) {
	err := testRedisCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	inCache, err := testRedisCache.Has("foo")
	if err != nil {
		t.Error(err)
	}

	if inCache {
		t.Error("foo should not be in cache")
	}

	err = testRedisCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	inCache, err = testRedisCache.Has("foo")
	if err != nil {
		t.Error(err)
	}

	if !inCache {
		t.Error("foo should be in cache")
	}
}

func TestRedisCache_Get(t *testing.T) {
	err := testRedisCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	_, err = testRedisCache.Get("foo")
	if err == nil {
		t.Error("foo should not be in cache")
	}

	err = testRedisCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	val, err := testRedisCache.Get("foo")
	if err != nil {
		t.Error(err)
	}

	if val != "bar" {
		t.Error("foo should be bar")
	}
}

func TestRedisCache_Set(t *testing.T) {
	err := testRedisCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	err = testRedisCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	val, err := testRedisCache.Get("foo")
	if err != nil {
		t.Error(err)
	}

	if val != "bar" {
		t.Error("foo should be bar")
	}

	err = testRedisCache.Set("time", "set", 60)
	if err != nil {
		t.Error(err)
	}

	val, err = testRedisCache.Get("time")
	if err != nil {
		t.Error(err)
	}

	if val != "set" {
		t.Error("time should be set")
	}

}

func TestRedisCache_Forget(t *testing.T) {
	err := testRedisCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	err = testRedisCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	err = testRedisCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	_, err = testRedisCache.Get("foo")
	if err == nil {
		t.Error("foo should not be in cache")
	}
}

func TestRedisCache_EmptyByMatch(t *testing.T) {
	err := testRedisCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	err = testRedisCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	err = testRedisCache.Set("bar", "baz")
	if err != nil {
		t.Error(err)
	}

	err = testRedisCache.Set("foo:bar", "baz")
	if err != nil {
		t.Error(err)
	}

	err = testRedisCache.EmptyByMatch("foo*")
	if err != nil {
		t.Error(err)
	}

	_, err = testRedisCache.Get("foo")
	if err == nil {
		t.Error("foo should not be in cache")
	}

	_, err = testRedisCache.Get("foo:bar")
	if err == nil {
		t.Error("foo:bar should not be in cache")
	}

	_, err = testRedisCache.Get("bar")
	if err != nil {
		t.Error("bar should be in cache")
	}
}

func TestRedisCache_Flush(t *testing.T) {
	err := testRedisCache.Forget("foo")
	if err != nil {
		t.Error(err)
	}

	err = testRedisCache.Set("foo", "bar")
	if err != nil {
		t.Error(err)
	}

	err = testRedisCache.Set("bar", "baz")
	if err != nil {
		t.Error(err)
	}

	err = testRedisCache.Set("foo:bar", "baz")
	if err != nil {
		t.Error(err)
	}

	err = testRedisCache.Flush()
	if err != nil {
		t.Error(err)
	}

	_, err = testRedisCache.Get("foo")
	if err == nil {
		t.Error("foo should not be in cache")
	}

	_, err = testRedisCache.Get("foo:bar")
	if err == nil {
		t.Error("foo:bar should not be in cache")
	}

	_, err = testRedisCache.Get("bar")
	if err == nil {
		t.Error("bar should not be in cache")
	}
}

func TestEncodeDecode(t *testing.T) {

	entry := Entry{}
	entry["foo"] = "bar"

	bytes, err := encode(entry)
	if err != nil {
		t.Error(err)
	}

	_, err = decode(string(bytes))
	if err != nil {
		t.Error(err)
	}
}
