package types

import (
	"context"

	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewClient creates a grpc client for a driver plugin
func NewClient(driverName string, addr string) (Driver, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	c := NewDriverClient(conn)
	return &grpcClient{
		client:     c,
		driverName: driverName,
	}, nil
}

// grpcClient defines the grpc client struct
type grpcClient struct {
	client     DriverClient
	driverName string
}

// Create call grpc create
func (rpc *grpcClient) Create(ctx context.Context, opts *DriverOptions) (*ClusterInfo, error) {
	o, err := rpc.client.Create(ctx, opts)
	return o, handlErr(err)
}

// Update call grpc update
func (rpc *grpcClient) Update(ctx context.Context, clusterInfo *ClusterInfo, opts *DriverOptions) (*ClusterInfo, error) {
	o, err := rpc.client.Update(ctx, &UpdateRequest{
		ClusterInfo:   clusterInfo,
		DriverOptions: opts,
	})
	return o, handlErr(err)
}

func (rpc *grpcClient) PostCheck(ctx context.Context, clusterInfo *ClusterInfo) (*ClusterInfo, error) {
	o, err := rpc.client.PostCheck(ctx, clusterInfo)
	return o, handlErr(err)
}

// Remove call grpc remove
func (rpc *grpcClient) Remove(ctx context.Context, clusterInfo *ClusterInfo) error {
	_, err := rpc.client.Remove(ctx, clusterInfo)
	return handlErr(err)
}

// GetDriverCreateOptions call grpc getDriverCreateOptions
func (rpc *grpcClient) GetDriverCreateOptions(ctx context.Context) (*DriverFlags, error) {
	o, err := rpc.client.GetDriverCreateOptions(ctx, &Empty{})
	return o, handlErr(err)
}

// GetDriverUpdateOptions call grpc getDriverUpdateOptions
func (rpc *grpcClient) GetDriverUpdateOptions(ctx context.Context) (*DriverFlags, error) {
	o, err := rpc.client.GetDriverUpdateOptions(ctx, &Empty{})
	return o, handlErr(err)
}

func handlErr(err error) error {
	if st, ok := status.FromError(err); ok {
		if st.Code() == codes.Unknown && st.Message() != "" {
			return errors.New(st.Message())
		}
	}
	return err
}
