// pkg/plugin/grpc.go
package plugin

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-plugin"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"

	"github.com/model-ci/apack/pkg/plugin/proto"
)

type PluginGRPC struct {
	plugin.Plugin
	Impl Plugin
}

func (p *PluginGRPC) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterPluginServiceServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *PluginGRPC) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: proto.NewPluginServiceClient(c)}, nil
}

type GRPCClient struct {
	client proto.PluginServiceClient
}

func (m *GRPCClient) Process(ctx *fasthttp.RequestCtx) (bool, error) {
	req := &proto.ProcessRequest{
		Method:     string(ctx.Method()),
		Path:       string(ctx.Path()),
		Headers:    make(map[string]string),
		Body:       ctx.PostBody(),
		RemoteAddr: ctx.RemoteAddr().String(),
		UserAgent:  string(ctx.UserAgent()),
	}

	ctx.Request.Header.VisitAll(func(key, value []byte) {
		req.Headers[string(key)] = string(value)
	})

	resp, err := m.client.Process(context.Background(), req)
	if err != nil {
		return false, err
	}

	if resp.Error != "" {
		return resp.Blocked, fmt.Errorf(resp.Error)
	}

	if resp.StatusCode > 0 {
		ctx.SetStatusCode(int(resp.StatusCode))

		for key, value := range resp.ResponseHeaders {
			ctx.Response.Header.Set(key, value)
		}

		if len(resp.ResponseBody) > 0 {
			ctx.SetBody(resp.ResponseBody)
		}
	}

	return resp.Blocked, nil
}

func (m *GRPCClient) GetInfo() (*PluginInfo, error) {
	resp, err := m.client.GetInfo(context.Background(), &proto.GetInfoRequest{})
	if err != nil {
		return nil, err
	}

	return &PluginInfo{
		Name:         resp.Name,
		Version:      resp.Version,
		Description:  resp.Description,
		Capabilities: resp.Capabilities,
	}, nil
}

func (m *GRPCClient) Configure(config map[string]string) error {
	resp, err := m.client.Configure(context.Background(), &proto.ConfigureRequest{
		Config: config,
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("configuration failed: %s", resp.Error)
	}

	return nil
}

type GRPCServer struct {
	proto.UnimplementedPluginServiceServer
	Impl Plugin
}

func (m *GRPCServer) Process(ctx context.Context, req *proto.ProcessRequest) (*proto.ProcessResponse, error) {
	return &proto.ProcessResponse{
		Blocked:    false,
		Reason:     "",
		StatusCode: 200,
	}, nil
}

func (m *GRPCServer) GetInfo(ctx context.Context, req *proto.GetInfoRequest) (*proto.GetInfoResponse, error) {
	if infoProvider, ok := m.Impl.(InfoProvider); ok {
		info := infoProvider.GetInfo()
		return &proto.GetInfoResponse{
			Name:         info.Name,
			Version:      info.Version,
			Description:  info.Description,
			Capabilities: info.Capabilities,
		}, nil
	}

	return &proto.GetInfoResponse{
		Name:        "unknown",
		Version:     "1.0.0",
		Description: "Plugin",
	}, nil
}

func (m *GRPCServer) Configure(ctx context.Context, req *proto.ConfigureRequest) (*proto.ConfigureResponse, error) {
	if configurable, ok := m.Impl.(Configurable); ok {
		err := configurable.Configure(req.Config)
		if err != nil {
			return &proto.ConfigureResponse{
				Success: false,
				Error:   err.Error(),
			}, nil
		}
	}

	return &proto.ConfigureResponse{
		Success: true,
	}, nil
}
