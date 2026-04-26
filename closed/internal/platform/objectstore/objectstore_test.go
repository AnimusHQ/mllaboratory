package objectstore

import "testing"

func TestConfigValidate(t *testing.T) {
	valid := Config{
		Endpoint:        "localhost:9000",
		AccessKey:       "a",
		SecretKey:       "b",
		Region:          "us-east-1",
		UseSSL:          false,
		BucketDatasets:  "datasets",
		BucketArtifacts: "artifacts",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() err=%v", err)
	}

	invalid := valid
	invalid.Endpoint = "http://localhost:9000"
	if err := invalid.Validate(); err == nil {
		t.Fatalf("Validate() expected error for scheme in endpoint")
	}
}
