package filesystems

import "time"

// FS is an interface that defines the methods that a filesystem must implement
type FS interface {
	Put(fileName, folder string) error
	Get(destination string, items ...string) error
	List(prefix string) ([]Listing, error)
	Delete(items []string) bool
}

// Listing is a struct that represents a file or directory in a filesystem
type Listing struct {
	Etag         string
	LastModified time.Time
	Key          string
	Size         float64
	IsDir        bool
}
