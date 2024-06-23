package miniofilesystem

import (
	"context"
	"errors"
	"github.com/jimmitjoo/gemquick/filesystems"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

// MockMinioClient is a mock implementation of the MinioClientInterface
type MockMinioClient struct{}

var mockMinio = &Minio{
	Endpoint:  "localhost:9000",
	AccessKey: "minioadmin",
	SecretKey: "minioadmin",
	UseSSL:    false,
	Region:    "us-east-1",
	Bucket:    "testbucket",
	Client:    &MockMinioClient{},
}

func (m *MockMinioClient) FPutObject(ctx context.Context, bucketName, objectName, filePath string, opts minio.PutObjectOptions) (info minio.UploadInfo, err error) {
	// Mock implementation here
	return minio.UploadInfo{
		Bucket: bucketName,
		Key:    objectName,
		ETag:   "mock-etag",
		Size:   1234,
	}, nil
}

func (m *MockMinioClient) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	objectInfoChan := make(chan minio.ObjectInfo)

	go func() {
		defer close(objectInfoChan)

		// Here you can add the expected objects to the channel
		// This is just a placeholder, replace it with your actual objects
		objectInfoChan <- minio.ObjectInfo{
			Key:          "expectedFileName",
			LastModified: time.Now(),
			ETag:         "mock-etag",
			Size:         1234,
		}
	}()

	return objectInfoChan
}

func (m *MockMinioClient) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	// Mock implementation here
	return nil
}

func (m *MockMinioClient) FGetObject(ctx context.Context, bucketName, objectName, filePath string, opts minio.GetObjectOptions) error {
	// Return an error if the objectName is empty or non-existent
	if objectName == "" || objectName == "nonExistentItem" {
		return errors.New("object does not exist")
	}

	// Return nil for other cases to simulate a successful operation
	return nil
}

func TestMinio_Put(t *testing.T) {
	m := mockMinio

	err := m.Put("testdata/filetoupload.txt", "testfolder")
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
}

func TestMinio_List(t *testing.T) {
	m := &Minio{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		UseSSL:    false,
		Region:    "us-east-1",
		Bucket:    "testbucket",
		Client:    &MockMinioClient{},
	}

	listings, err := m.List("testfolder")
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	expectedFileListing := filesystems.Listing{
		Etag: "mock-etag",
		Key:  "expectedFileName",
	}
	for _, listing := range listings {
		if listing.Key != expectedFileListing.Key {
			t.Errorf("Expected %v, got %v", expectedFileListing.Key, listing.Key)
		}
		if listing.Etag != expectedFileListing.Etag {
			t.Errorf("Expected %v, got %v", expectedFileListing.Etag, listing.Etag)
		}
	}
}

func TestMinio_Delete(t *testing.T) {
	m := mockMinio

	items := []string{"testfolder/testfile"}
	result := m.Delete(items)
	if !result {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestMinio_Get_EmptyItems(t *testing.T) {
	m := mockMinio

	err := m.Get("destinationFolder")
	if err != nil {
		t.Errorf("Expected nil, got error")
	}
}

func TestMinio_Get_NonExistentItem(t *testing.T) {
	m := mockMinio

	items := []string{"nonExistentItem"}
	err := m.Get("destinationFolder", items...)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestMinio_Get_MultipleItems(t *testing.T) {
	m := mockMinio

	items := []string{"item1", "item2", "item3"}
	err := m.Get("destinationFolder", items...)
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
}
