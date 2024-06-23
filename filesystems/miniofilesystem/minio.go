package miniofilesystem

import (
	"context"
	"github.com/jimmitjoo/gemquick/filesystems"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"log"
	"path"
	"strings"
)

type MinioClientInterface interface {
	FPutObject(ctx context.Context, bucketName, objectName, filePath string, opts minio.PutObjectOptions) (info minio.UploadInfo, err error)
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
	FGetObject(ctx context.Context, bucketName, objectName, filePath string, opts minio.GetObjectOptions) error
}

type Minio struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Region    string
	Bucket    string
	Client    MinioClientInterface
}

/*func (m *Minio) getCredentials() *minio.Client {
	client, err := minio.New(m.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(m.AccessKey, m.SecretKey, ""),
		Secure: m.UseSSL,
	})
	if err != nil {
		log.Println(err)
	} else {
		log.Println("Connected to Minio at", m.Endpoint)
	}
	return client
}*/

func (m *Minio) getCredentials() MinioClientInterface {
	if m.Client != nil {
		return m.Client
	}

	client, err := minio.New(m.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(m.AccessKey, m.SecretKey, ""),
		Secure: m.UseSSL,
	})
	if err != nil {
		log.Println(err)
	} else {
		log.Println("Connected to Minio at", m.Endpoint)
	}

	return client
}

// Put uploads a file to the Minio bucket
func (m *Minio) Put(fileName, folder string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	objectName := path.Base(fileName)
	client := m.getCredentials()
	uploadInfo, err := client.FPutObject(ctx, m.Bucket, path.Join(folder, objectName), fileName, minio.PutObjectOptions{})
	if err != nil {
		log.Println("Failed to upload", fileName, "to", m.Bucket, ":", err)
		log.Println("Upload info:", uploadInfo)
		return err
	}

	return nil
}

func (m *Minio) List(prefix string) ([]filesystems.Listing, error) {
	var listing []filesystems.Listing

	if prefix == "/" {
		prefix = ""
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := m.getCredentials()
	objects := client.ListObjects(ctx, m.Bucket, minio.ListObjectsOptions{
		Prefix: prefix,
	})

	for object := range objects {
		if object.Err != nil {
			return listing, object.Err
		}

		if !strings.HasPrefix(object.Key, ".") {
			b := float64(object.Size)
			kb := b / 1024
			mb := kb / 1024
			item := filesystems.Listing{
				Etag:         object.ETag,
				LastModified: object.LastModified,
				Key:          object.Key,
				Size:         mb,
			}

			listing = append(listing, item)
		}
	}

	return listing, nil
}

func (m *Minio) Delete(items []string) bool {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := m.getCredentials()

	for _, item := range items {
		err := client.RemoveObject(ctx, m.Bucket, item, minio.RemoveObjectOptions{
			GovernanceBypass: true,
		})
		if err != nil {
			log.Println("Failed to remove", item, "from", m.Bucket, ":", err)
			return false
		}
	}

	return true
}

func (m *Minio) Get(destination string, items ...string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := m.getCredentials()
	for _, item := range items {
		objectName := path.Base(item)
		err := client.FGetObject(ctx, m.Bucket, item, path.Join(destination, objectName), minio.GetObjectOptions{})
		if err != nil {
			log.Println("Failed to download", item, "from", m.Bucket, ":", err)
			return err
		}
	}

	return nil
}
