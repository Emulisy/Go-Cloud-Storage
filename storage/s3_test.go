package storage

import "testing"

func TestMinIOConfiguration(t *testing.T) {
	for _, test := range []struct {
		name, endpoint, baseEndpoint, configuredRegion, region string
		invalid                                                bool
	}{
		{name: "HTTP service", endpoint: "http://minio:9000", baseEndpoint: "http://minio:9000", region: "us-east-1"},
		{name: "HTTPS with custom region", endpoint: "https://minio.example.com", baseEndpoint: "https://minio.example.com", configuredRegion: "ap-southeast-1", region: "ap-southeast-1"},
		{name: "host and port", endpoint: "minio:9000", baseEndpoint: "http://minio:9000", region: "us-east-1"},
		{name: "missing endpoint", invalid: true},
		{name: "bucket path", endpoint: "http://minio:9000/my-bucket", invalid: true},
		{name: "unsupported scheme", endpoint: "ftp://minio:9000", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("MINIO_ENDPOINT", test.endpoint)
			t.Setenv("MINIO_BUCKET", "test-bucket")
			t.Setenv("MINIO_ACCESS_KEY_ID", "test-access-key")
			t.Setenv("MINIO_SECRET_ACCESS_KEY", "test-secret-key")
			t.Setenv("MINIO_REGION", test.configuredRegion)
			err := Init()
			if test.invalid {
				if err == nil {
					t.Fatal("accepted an invalid endpoint")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			options := Store().client.Options()
			if *options.BaseEndpoint != test.baseEndpoint || options.Region != test.region || !options.UsePathStyle {
				t.Error("unexpected MinIO endpoint, region, or addressing mode")
			}
		})
	}
}

func TestObjectKey(t *testing.T) {
	store := &S3Store{}
	const object = "objects/digest/id"
	address := store.Address(object)
	if address != "s3://"+object {
		t.Errorf("MinIO address = %q", address)
	}
	if key, remote, err := store.ObjectKey(address); key != object || !remote || err != nil {
		t.Errorf("MinIO object = (%q, %v, %v)", key, remote, err)
	}
	if key, remote, err := store.ObjectKey("legacy://" + object); key != "" || !remote || err == nil {
		t.Errorf("old object address = (%q, %v, %v)", key, remote, err)
	}
	if key, remote, err := store.ObjectKey("/legacy/local.txt"); key != "" || remote || err != nil {
		t.Errorf("legacy path = (%q, %v, %v)", key, remote, err)
	}
}
