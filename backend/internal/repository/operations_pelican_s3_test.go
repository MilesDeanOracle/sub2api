package repository

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOperationsS3DeletesExactObjectVersions(t *testing.T) {
	key := service.OperationsPelicanPrefix + "test.html"
	var mu sync.Mutex
	deleted := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Has("versions"):
			fmt.Fprintf(w, `<ListVersionsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><IsTruncated>false</IsTruncated><Version><Key>%s</Key><VersionId>v1</VersionId></Version><Version><Key>%s.other</Key><VersionId>untouched</VersionId></Version><DeleteMarker><Key>%s</Key><VersionId>marker1</VersionId></DeleteMarker></ListVersionsResult>`, key, key, key)
		case r.Method == http.MethodGet && r.URL.Query().Has("versioning"):
			fmt.Fprint(w, `<VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Status>Enabled</Status></VersioningConfiguration>`)
		case r.Method == http.MethodDelete:
			mu.Lock()
			deleted = append(deleted, strings.TrimPrefix(r.URL.Path, "/artifacts/")+":"+r.URL.Query().Get("versionId"))
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected S3 request: %s %s", r.Method, r.URL)
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	store, e := NewOperationsObjectStoreFactory()(context.Background(), &service.BackupS3Config{Endpoint: server.URL, Bucket: "artifacts", Region: "auto", AccessKeyID: "test", SecretAccessKey: "test", ForcePathStyle: true})
	require.NoError(t, e)
	require.NoError(t, store.Delete(context.Background(), key, ""))
	mu.Lock()
	defer mu.Unlock()
	require.ElementsMatch(t, []string{key + ":v1", key + ":marker1"}, deleted)
}

func TestOperationsS3DeletionDenialIsNotSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `<Error><Code>AccessDenied</Code><Message>denied</Message></Error>`)
	}))
	defer server.Close()
	store, e := NewOperationsObjectStoreFactory()(context.Background(), &service.BackupS3Config{Endpoint: server.URL, Bucket: "artifacts", AccessKeyID: "test", SecretAccessKey: "test", ForcePathStyle: true})
	require.NoError(t, e)
	require.Error(t, store.Delete(context.Background(), service.OperationsPelicanPrefix+"test.html", "v1"))
	require.Error(t, store.Delete(context.Background(), "backup/other.html", ""))
}
