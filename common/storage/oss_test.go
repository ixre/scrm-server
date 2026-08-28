package storage

import (
	"core/app/constants"
	"core/conf"
	"net/url"
	"testing"
)

func TestOSSStorage_SignURL(t *testing.T) {
	store, err := NewOSS(conf.StorageConfig{EndPoint: "https://oss-cn-hangzhou.aliyuncs.com", AccessKeyId: "test-key-id", AccessKeySecret: "test-secret", Bucket: "test-bucket"})
	if err != nil {
		t.Fatal(err)
	}
	signed, err := store.SignURL("test/image.jpg", constants.HTTPPut, 60)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/test/image.jpg" || parsed.Query().Get("Signature") == "" {
		t.Fatal("missing object path or signature")
	}
}
