package config

import "testing"

func TestFromEnvRequiresDatabase(t *testing.T) {
	t.Setenv("EPHYRA_DATABASE_URL", "")
	t.Setenv("EPHYRA_ENV", "dev")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected missing database url")
	}
}

func TestProductionRejectsInsecureDefaults(t *testing.T) {
	t.Setenv("EPHYRA_ENV", "production")
	t.Setenv("EPHYRA_DATABASE_URL", "postgres://ephyra@localhost/ephyra")
	t.Setenv("EPHYRA_SECURE_COOKIE", "")
	t.Setenv("EPHYRA_S3_ENDPOINT", "")
	t.Setenv("EPHYRA_REDIS_URL", "")
	t.Setenv("EPHYRA_GRPC_CERT", "")
	t.Setenv("EPHYRA_GRPC_KEY", "")
	t.Setenv("EPHYRA_PUBLIC_URL", "http://localhost:8080")
	_, err := FromEnv()
	if err == nil {
		t.Fatal("production must refuse to start")
	}
}

func TestProductionAcceptsCompleteConfig(t *testing.T) {
	t.Setenv("EPHYRA_ENV", "production")
	t.Setenv("EPHYRA_DATABASE_URL", "postgres://ephyra@localhost/ephyra")
	t.Setenv("EPHYRA_REDIS_URL", "redis://localhost:6379")
	t.Setenv("EPHYRA_S3_ENDPOINT", "s3.example")
	t.Setenv("EPHYRA_S3_ACCESS_KEY", "key")
	t.Setenv("EPHYRA_S3_SECRET_KEY", "secret")
	t.Setenv("EPHYRA_GRPC_CERT", "cert.pem")
	t.Setenv("EPHYRA_GRPC_KEY", "key.pem")
	t.Setenv("EPHYRA_SECURE_COOKIE", "true")
	t.Setenv("EPHYRA_PUBLIC_URL", "https://oj.example")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Production() || !cfg.SecureCookie {
		t.Fatalf("%+v", cfg)
	}
}

func TestDevAllowsMissingS3(t *testing.T) {
	t.Setenv("EPHYRA_ENV", "dev")
	t.Setenv("EPHYRA_DATABASE_URL", "postgres://ephyra@localhost/ephyra")
	t.Setenv("EPHYRA_S3_ENDPOINT", "")
	t.Setenv("EPHYRA_REDIS_URL", "")
	t.Setenv("EPHYRA_SECURE_COOKIE", "false")
	if _, err := FromEnv(); err != nil {
		t.Fatal(err)
	}
}
