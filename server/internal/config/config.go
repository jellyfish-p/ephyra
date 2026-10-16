package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Env string

const (
	EnvDev        Env = "dev"
	EnvProduction Env = "production"
)

type Config struct {
	Env                    Env
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

func (c Config) Production() bool { return c.Env == EnvProduction }

func FromEnv() (Config, error) {
	env, err := parseEnv(os.Getenv("EPHYRA_ENV"))
	if err != nil {
		return Config{}, err
	}
	c := Config{
		Env:                    env,
		HTTPAddr:               envDefault("EPHYRA_HTTP_ADDR", ":8080"),
		GRPCAddr:               envDefault("EPHYRA_GRPC_ADDR", ":9090"),
		DatabaseURL:            os.Getenv("EPHYRA_DATABASE_URL"),
		RedisURL:               os.Getenv("EPHYRA_REDIS_URL"),
		S3Endpoint:             os.Getenv("EPHYRA_S3_ENDPOINT"),
		S3AccessKey:            os.Getenv("EPHYRA_S3_ACCESS_KEY"),
		S3SecretKey:            os.Getenv("EPHYRA_S3_SECRET_KEY"),
		S3Bucket:               envDefault("EPHYRA_S3_BUCKET", "ephyra"),
		S3UseSSL:               envBool("EPHYRA_S3_USE_SSL", false),
		PublicURL:              envDefault("EPHYRA_PUBLIC_URL", "http://localhost:8080"),
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
	if c.Production() {
		if err := c.validateProduction(); err != nil {
			return Config{}, err
		}
	}
	return c, nil
}

func (c Config) validateProduction() error {
	var missing []string
	if c.S3Endpoint == "" {
		missing = append(missing, "EPHYRA_S3_ENDPOINT")
	}
	if c.S3AccessKey == "" {
		missing = append(missing, "EPHYRA_S3_ACCESS_KEY")
	}
	if c.S3SecretKey == "" {
		missing = append(missing, "EPHYRA_S3_SECRET_KEY")
	}
	if c.RedisURL == "" {
		missing = append(missing, "EPHYRA_REDIS_URL")
	}
	if c.GRPCCert == "" || c.GRPCKey == "" {
		missing = append(missing, "EPHYRA_GRPC_CERT and EPHYRA_GRPC_KEY")
	}
	if !c.SecureCookie {
		missing = append(missing, "EPHYRA_SECURE_COOKIE=true")
	}
	if !strings.HasPrefix(strings.ToLower(c.PublicURL), "https://") {
		missing = append(missing, "EPHYRA_PUBLIC_URL (https)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("production refuses to start without: %s", strings.Join(missing, ", "))
	}
	return nil
}

func parseEnv(s string) (Env, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "dev", "development":
		return EnvDev, nil
	case "prod", "production":
		return EnvProduction, nil
	default:
		return "", fmt.Errorf("EPHYRA_ENV must be dev or production")
	}
}

func envDefault(key, def string) string {
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
