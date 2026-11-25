package storage

import "testing"

func TestStorageBackendConfiguration(t *testing.T) {
	for _, test := range []struct {
		name, backend, variablePrefix, endpoint, baseEndpoint, addressPrefix, region string
		invalid                                                                      bool
	}{
		{name: "default R2", variablePrefix: "R2_", endpoint: "https://storage.example.com", baseEndpoint: "https://storage.example.com", addressPrefix: "r2://", region: "auto"},
		{name: "R2 rejects HTTP", backend: "r2", variablePrefix: "R2_", endpoint: "http://storage.example.com", invalid: true},
		{name: "MinIO HTTP", backend: "minio", variablePrefix: "MINIO_", endpoint: "http://minio:9000", baseEndpoint: "http://minio:9000", addressPrefix: "s3://", region: "us-east-1"},
		{name: "MinIO host and port", backend: "minio", variablePrefix: "MINIO_", endpoint: "minio:9000", baseEndpoint: "http://minio:9000", addressPrefix: "s3://", region: "us-east-1"},
		{name: "MinIO rejects bucket path", backend: "minio", variablePrefix: "MINIO_", endpoint: "http://minio:9000/my-bucket", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("STORAGE_BACKEND", test.backend)
			t.Setenv(test.variablePrefix+"ENDPOINT", test.endpoint)
			t.Setenv(test.variablePrefix+"BUCKET", "test-bucket")
			t.Setenv(test.variablePrefix+"ACCESS_KEY_ID", "test-access-key")
			t.Setenv(test.variablePrefix+"SECRET_ACCESS_KEY", "test-secret-key")
			t.Setenv(test.variablePrefix+"REGION", "")
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
			store := Store()
			options := store.client.Options()
			if *options.BaseEndpoint != test.baseEndpoint || options.Region != test.region || !options.UsePathStyle {
				t.Errorf("unexpected S3 endpoint, region, or addressing mode")
			}
			if got := store.Address("objects/digest/id"); got != test.addressPrefix+"objects/digest/id" {
				t.Errorf("unexpected object address %q", got)
			}
		})
	}
}

func TestObjectKeyResolvesOnlyTheActiveBackend(t *testing.T) {
	store := &S3Store{addressPrefix: minioAddressPrefix}
	const object = "objects/digest/id"
	if key, remote, err := store.ObjectKey("s3://" + object); key != object || !remote || err != nil {
		t.Errorf("MinIO address = (%q, %v, %v)", key, remote, err)
	}
	if key, remote, err := store.ObjectKey("r2://" + object); key != "" || !remote || err == nil {
		t.Errorf("R2 address = (%q, %v, %v)", key, remote, err)
	}
	if key, remote, err := store.ObjectKey("/legacy/local.txt"); key != "" || remote || err != nil {
		t.Errorf("legacy path = (%q, %v, %v)", key, remote, err)
	}
}
