package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr               string
	GRPCAddr               string
	DatabaseURL            string
	RedisURL               string
	S3Endpoint             string
	S3AccessKey            string
	S3SecretKey            string
	S3Bucket               string
	S3UseSSL               bool
	PublicURL              string
	SecureCookie           bool
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
	SMTPHost               string
	SMTPPort               int
	SMTPUser               string
	SMTPPass               string
	SMTPFrom               string
	GRPCCert               string
	GRPCKey                string
}

func FromEnv() (Config, error) {
	c := Config{
		HTTPAddr:               env("EPHYRA_HTTP_ADDR", ":8080"),
		GRPCAddr:               env("EPHYRA_GRPC_ADDR", ":9090"),
		DatabaseURL:            os.Getenv("EPHYRA_DATABASE_URL"),
		RedisURL:               os.Getenv("EPHYRA_REDIS_URL"),
		S3Endpoint:             os.Getenv("EPHYRA_S3_ENDPOINT"),
		S3AccessKey:            os.Getenv("EPHYRA_S3_ACCESS_KEY"),
		S3SecretKey:            os.Getenv("EPHYRA_S3_SECRET_KEY"),
		S3Bucket:               env("EPHYRA_S3_BUCKET", "ephyra"),
		S3UseSSL:               envBool("EPHYRA_S3_USE_SSL", false),
		PublicURL:              env("EPHYRA_PUBLIC_URL", "http://localhost:8080"),
		SecureCookie:           envBool("EPHYRA_SECURE_COOKIE", false),
		BootstrapAdminEmail:    os.Getenv("EPHYRA_BOOTSTRAP_ADMIN_EMAIL"),
		BootstrapAdminPassword: os.Getenv("EPHYRA_BOOTSTRAP_ADMIN_PASSWORD"),
		SMTPHost:               os.Getenv("EPHYRA_SMTP_HOST"),
		SMTPPort:               envInt("EPHYRA_SMTP_PORT", 587),
		SMTPUser:               os.Getenv("EPHYRA_SMTP_USER"),
		SMTPPass:               os.Getenv("EPHYRA_SMTP_PASS"),
		SMTPFrom:               os.Getenv("EPHYRA_SMTP_FROM"),
		GRPCCert:               os.Getenv("EPHYRA_GRPC_CERT"),
		GRPCKey:                os.Getenv("EPHYRA_GRPC_KEY"),
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("EPHYRA_DATABASE_URL is required")
	}
	return c, nil
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
