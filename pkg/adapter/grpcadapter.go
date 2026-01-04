package adapter

import (
	"fmt"
	"net/url"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GrpcAdapter struct {
	Cc   *grpc.ClientConn
	Cctx *ConnectionCtxt
}

type GrpcAdapterOption func(rc *GrpcAdapter)

func WithGrpcConnContext(cctx *ConnectionCtxt) GrpcAdapterOption {
	return func(rc *GrpcAdapter) {
		rc.Cctx = cctx
	}
}

func NewGrpcAdapter(opts ...GrpcAdapterOption) (*GrpcAdapter, error) {
	adpr := &GrpcAdapter{}
	for _, opt := range opts {
		opt(adpr)
	}
	u, err := url.Parse(adpr.Cctx.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse context URL: %s, %w", adpr.Cctx.URL, err)
	}
	grpcHost := "grpc." + u.Host
	if u.Scheme == "https" {
		grpcHost += ":443"
	} else {
		grpcHost += ":80"
	}

	adpr.Cc, err = grpc.NewClient(grpcHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("could not connect to gRPC server at %s: %w", u.Hostname(), err)
	}
	return adpr, nil
}
