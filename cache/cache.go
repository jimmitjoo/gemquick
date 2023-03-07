package cache

import (
	"bytes"
	"encoding/gob"

	"github.com/gomodule/redigo/redis"
)

type Cache interface {
	Has(string) (bool, error)
	Get(string) (interface{}, error)
	Set(string, interface{}, ...int) error
	Forget(string) error
	EmptyByMatch(string) error
	Flush() error
}

type RedisCache struct {
	Conn   *redis.Pool
	Prefix string
}

type Entry map[string]interface{}

func encode(item Entry) ([]byte, error) {
	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode(item)
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func decode(str string) (Entry, error) {
	b := bytes.Buffer{}
	b.WriteString(str)
	d := gob.NewDecoder(&b)
	var item Entry
	err := d.Decode(&item)
	if err != nil {
		return nil, err
	}

	return item, nil
}

func (c *RedisCache) Has(str string) (bool, error) {
	key := c.Prefix + str
	conn := c.Conn.Get()
	defer conn.Close()

	return redis.Bool(conn.Do("EXISTS", key))
}

func (c *RedisCache) Get(str string) (interface{}, error) {
	key := c.Prefix + str
	conn := c.Conn.Get()
	defer conn.Close()

	cacheEntry, err := redis.Bytes(conn.Do("GET", key))
	if err != nil {
		return nil, err
	}

	decoded, err := decode(string(cacheEntry))
	if err != nil {
		return nil, err
	}

	return decoded[key], nil
}

func (c *RedisCache) Set(str string, value interface{}, ttl ...int) error {
	key := c.Prefix + str
	conn := c.Conn.Get()
	defer conn.Close()

	entry := Entry{}
	entry[key] = value

	encoded, err := encode(entry)
	if err != nil {
		return err
	}

	if len(ttl) > 0 {
		_, err = conn.Do("SETEX", key, ttl[0], string(encoded))
		if err != nil {
			return err
		}
	} else {
		_, err = conn.Do("SET", key, string(encoded))
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *RedisCache) Forget(str string) error {
	key := c.Prefix + str
	conn := c.Conn.Get()
	defer conn.Close()

	_, err := conn.Do("DEL", key)
	return err
}

func (c *RedisCache) EmptyByMatch(str string) error {
	conn := c.Conn.Get()
	defer conn.Close()

	keys, err := redis.Strings(conn.Do("KEYS", c.Prefix+str))
	if err != nil {
		return err
	}

	for _, key := range keys {
		_, err = conn.Do("DEL", key)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *RedisCache) Flush() error {
	return c.EmptyByMatch("*")
}
