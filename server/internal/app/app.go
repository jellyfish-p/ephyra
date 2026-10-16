package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	judgev1 "github.com/jellyfish-p/ephyra/protocol/judge/v1"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/config"
	"github.com/jellyfish-p/ephyra/server/internal/contest"
	"github.com/jellyfish-p/ephyra/server/internal/httpapi"
	"github.com/jellyfish-p/ephyra/server/internal/identity"
	"github.com/jellyfish-p/ephyra/server/internal/judgegrpc"
	"github.com/jellyfish-p/ephyra/server/internal/judging"
	"github.com/jellyfish-p/ephyra/server/internal/mailer"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/outbox"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
	"github.com/jellyfish-p/ephyra/server/internal/rate"
	"github.com/jellyfish-p/ephyra/server/internal/scoring"
	"github.com/jellyfish-p/ephyra/server/internal/stream"
	"github.com/jellyfish-p/ephyra/server/internal/submission"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func Run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	configureLog(cfg)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return run(ctx, cfg)
}

func configureLog(cfg config.Config) {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	var h slog.Handler
	if cfg.Production() {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

func run(ctx context.Context, cfg config.Config) error {
	clk := clock.System{}
	db, err := persist.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	if err := persist.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	var blobs objectstore.Store
	if cfg.S3Endpoint != "" {
		s3, err := objectstore.NewS3(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3UseSSL)
		if err != nil {
			return err
		}
		if err := s3.EnsureBucket(ctx); err != nil {
			return err
		}
		blobs = s3
	} else {
		slog.Warn("S3 is not configured; using in-memory object store (dev only)")
		blobs = objectstore.NewMemory()
	}

	var mail mailer.Mailer = mailer.Log{}
	if cfg.SMTPHost != "" {
		mail = mailer.SMTP{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom}
	}

	var rdb *redis.Client
	if cfg.RedisURL != "" {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return err
		}
		rdb = redis.NewClient(opt)
	} else {
		slog.Warn("Redis is not configured; outbox will not publish and scheduling will poll PostgreSQL")
	}

	id := &identity.Module{DB: db, Clock: clk, Mail: mail, Limit: rate.New(), PublicURL: cfg.PublicURL}
	if err := id.BootstrapAdmin(ctx, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword); err != nil {
		return err
	}
	ct := &contest.Module{DB: db, Clock: clk}
	pr := &problem.Module{DB: db, Clock: clk, Blobs: blobs}
	sub := &submission.Module{DB: db, Clock: clk, Blobs: blobs}
	jg := judging.New(db, clk, blobs, cfg.PublicURL)
	if err := jg.EnsureSystemPool(ctx); err != nil {
		return err
	}
	sc := &scoring.Module{DB: db, Clock: clk}

	api := &httpapi.API{
		Identity: id, Contest: ct, Problem: pr, Submission: sub, Judging: jg, Scoring: sc,
		Blobs: blobs, Secure: cfg.SecureCookie,
		Ready: func(ctx context.Context) error {
			if err := persist.Ping(ctx, db); err != nil {
				return err
			}
			if rdb != nil {
				return rdb.Ping(ctx).Err()
			}
			return nil
		},
	}
	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
		// WriteTimeout stays zero: SSE connections write for the lifetime of the contest view.
	}

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return err
	}
	var grpcSrv *grpc.Server
	if cfg.GRPCCert != "" && cfg.GRPCKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.GRPCCert, cfg.GRPCKey)
		if err != nil {
			return err
		}
		grpcSrv = grpc.NewServer(grpc.Creds(credentials.NewServerTLSFromCert(&cert)))
	} else {
		slog.Warn("gRPC TLS certs are not set; nodes must only be used on a trusted network for development")
		grpcSrv = grpc.NewServer()
	}
	judgev1.RegisterJudgeControlServer(grpcSrv, &judgegrpc.Server{Judging: jg, Hub: jg.Hub, Clock: clk})

	pub := &outbox.Publisher{DB: db, Redis: rdb}
	cons := &stream.Consumer{
		DB: db, Redis: rdb, Group: "ephyra-server", Consumer: uuid.NewString(),
		OnReady: jg.Schedule,
	}

	slog.Info("listening", "http", cfg.HTTPAddr, "grpc", cfg.GRPCAddr)
	return serve(ctx, httpSrv, grpcSrv, lis,
		pub.Run,
		cons.Run,
		func(ctx context.Context) error {
			return background(ctx, jg, sc)
		},
	)
}

func background(ctx context.Context, jg *judging.Module, sc *scoring.Module) error {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := jg.Schedule(ctx); err != nil && ctx.Err() == nil {
				slog.Error("schedule", "err", err)
			}
			if err := jg.ReapLost(ctx); err != nil && ctx.Err() == nil {
				slog.Error("reap", "err", err)
			}
			if err := sc.TryPublish(ctx); err != nil && ctx.Err() == nil {
				slog.Error("publish", "err", err)
			}
		}
	}
}
