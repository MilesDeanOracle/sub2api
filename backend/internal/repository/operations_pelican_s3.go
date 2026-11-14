package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
)

type operationsS3 struct {
	client *s3.Client
	bucket string
}

func NewOperationsObjectStoreFactory() service.OperationsObjectStoreFactory {
	return func(ctx context.Context, c *service.BackupS3Config) (service.OperationsObjectStore, error) {
		client, e := newS3Client(ctx, s3ClientParams{Endpoint: c.Endpoint, Region: c.Region, AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, ForcePathStyle: c.ForcePathStyle})
		if e != nil {
			return nil, e
		}
		return &operationsS3{client: client, bucket: c.Bucket}, nil
	}
}
func operationsKey(key string) error {
	if !strings.HasPrefix(key, service.OperationsPelicanPrefix) || strings.Contains(key, "..") {
		return errors.New("invalid operations object key")
	}
	return nil
}
func (s *operationsS3) Put(ctx context.Context, key string, data []byte) (*service.OperationsStoredObject, error) {
	if e := operationsKey(key); e != nil {
		return nil, e
	}
	out, e := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: bytes.NewReader(data), ContentType: aws.String("text/html; charset=utf-8"), Metadata: map[string]string{"operations-artifact": "pelican"}})
	if e != nil {
		return nil, e
	}
	return &service.OperationsStoredObject{VersionID: aws.ToString(out.VersionId), Size: int64(len(data))}, nil
}
func (s *operationsS3) Read(ctx context.Context, key string) ([]byte, error) {
	if e := operationsKey(key); e != nil {
		return nil, e
	}
	out, e := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if e != nil {
		return nil, e
	}
	defer out.Body.Close()
	data, e := io.ReadAll(io.LimitReader(out.Body, service.OperationsPelicanMaxHTML+1))
	if e != nil {
		return nil, e
	}
	if len(data) > service.OperationsPelicanMaxHTML {
		return nil, errors.New("artifact exceeds size limit")
	}
	return data, nil
}
func (s *operationsS3) Inspect(ctx context.Context, key string) (*service.OperationsStoredObject, error) {
	if e := operationsKey(key); e != nil {
		return nil, e
	}
	out, e := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key})
	if e != nil {
		return nil, e
	}
	return &service.OperationsStoredObject{VersionID: aws.ToString(out.VersionId), Size: aws.ToInt64(out.ContentLength)}, nil
}
func (s *operationsS3) Delete(ctx context.Context, key, version string) error {
	if e := operationsKey(key); e != nil {
		return e
	}
	// List exact-key versions to cover uncertain PUTs and a final DB write failure.
	// No other key in the dedicated prefix is removed by this operation.
	var keyMarker, versionMarker *string
	listed := true
	for {
		out, e := s.client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{Bucket: &s.bucket, Prefix: &key, KeyMarker: keyMarker, VersionIdMarker: versionMarker})
		if e != nil {
			var api smithy.APIError
			if errors.As(e, &api) && (api.ErrorCode() == "NotImplemented" || api.ErrorCode() == "NotSupported" || api.ErrorCode() == "InvalidRequest") {
				listed = false
				break
			}
			return e
		}
		for _, v := range out.Versions {
			if aws.ToString(v.Key) == key {
				if _, e = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key, VersionId: v.VersionId}); e != nil {
					return e
				}
			}
		}
		for _, v := range out.DeleteMarkers {
			if aws.ToString(v.Key) == key {
				if _, e = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key, VersionId: v.VersionId}); e != nil {
					return e
				}
			}
		}
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		keyMarker = out.NextKeyMarker
		versionMarker = out.NextVersionIdMarker
	}
	input := &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key}
	if version != "" {
		input.VersionId = &version
	}
	// After clearing versions, an unversioned delete on an enabled bucket would
	// create a new marker. Query versioning before issuing the final fallback.
	state, e := s.client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: &s.bucket})
	if e == nil && state.Status != "" {
		if !listed {
			return errors.New("cannot enumerate versions in a versioned bucket")
		}
		return nil
	}
	if e != nil {
		var api smithy.APIError
		if !errors.As(e, &api) || (api.ErrorCode() != "NotImplemented" && api.ErrorCode() != "NotSupported") {
			return e
		}
	}
	_, e = s.client.DeleteObject(ctx, input)
	return e
}
func (s *operationsS3) Test(ctx context.Context) error {
	key := service.OperationsPelicanPrefix + ".connection-test/" + uuid.NewString() + ".html"
	body := []byte("<!doctype html><html><body>storage test</body></html>")
	obj, e := s.Put(ctx, key, body)
	if e != nil {
		return fmt.Errorf("storage write test failed: %w", e)
	}
	data, readErr := s.Read(ctx, key)
	deleteErr := s.Delete(ctx, key, obj.VersionID)
	if deleteErr != nil {
		return fmt.Errorf("storage delete test failed (object %s): %w", key, deleteErr)
	}
	if readErr != nil {
		return fmt.Errorf("storage read test failed: %w", readErr)
	}
	if !bytes.Equal(data, body) {
		return errors.New("storage read verification failed")
	}
	return nil
}
