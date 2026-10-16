package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

const shutdownTimeout = 10 * time.Second

// serve runs HTTP, gRPC, and background workers under one context.
// Cancelling ctx starts a bounded shutdown: HTTP Shutdown plus gRPC
// GracefulStop, falling back to Stop when the deadline expires.
func serve(ctx context.Context, httpSrv *http.Server, grpcSrv *grpc.Server, lis net.Listener, workers ...func(context.Context) error) error {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		err := httpSrv.ListenAndServe()
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("http: %w", err)
	})
	g.Go(func() error {
		err := grpcSrv.Serve(lis)
		if err == nil || errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("grpc: %w", err)
	})
	for _, w := range workers {
		worker := w
		g.Go(func() error { return worker(ctx) })
	}
	g.Go(func() error {
		<-ctx.Done()
		return shutdownServers(httpSrv, grpcSrv)
	})
	return g.Wait()
}

func shutdownServers(httpSrv *http.Server, grpcSrv *grpc.Server) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	httpDone := make(chan error, 1)
	go func() { httpDone <- httpSrv.Shutdown(ctx) }()

	grpcDone := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(grpcDone)
	}()

	select {
	case <-grpcDone:
	case <-ctx.Done():
		grpcSrv.Stop()
		<-grpcDone
	}
	return <-httpDone
}
