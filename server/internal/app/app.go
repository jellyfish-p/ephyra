package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
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
	clk := clock.System{}
	db, err := persist.Open(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	ctx := context.Background()
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
		log.Print("S3 is not configured; using in-memory object store (dev only)")
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
		log.Print("Redis is not configured; outbox will not publish and scheduling will poll PostgreSQL")
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
	}
	httpSrv := &http.Server{Addr: cfg.HTTPAddr, Handler: api.Router()}

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
		log.Print("gRPC TLS certs are not set; nodes must only be used on a trusted network for development")
		grpcSrv = grpc.NewServer()
	}
	judgev1.RegisterJudgeControlServer(grpcSrv, &judgegrpc.Server{Judging: jg, Hub: jg.Hub, Clock: clk})

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pub := &outbox.Publisher{DB: db, Redis: rdb}
	go pub.Run(runCtx)
	cons := &stream.Consumer{
		DB: db, Redis: rdb, Group: "ephyra-server", Consumer: uuid.NewString(),
		OnReady: jg.Schedule,
	}
	go cons.Run(runCtx)
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-t.C:
				_ = jg.Schedule(runCtx)
				_ = jg.ReapLost(runCtx)
				_ = sc.TryPublish(runCtx)
			}
		}
	}()

	go func() {
		log.Printf("http %s", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("http: %v", err)
		}
	}()
	go func() {
		log.Printf("grpc %s", cfg.GRPCAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Printf("grpc: %v", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	cancel()
	shutdownCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	_ = httpSrv.Shutdown(shutdownCtx)
	grpcSrv.GracefulStop()
	return nil
}
