package grpcserver

import (
	"context"

	"core/api/pb"
	"core/app/models"
	"core/app/requests"
	"core/app/services"
	app "core/common/app"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CustomerServer 是闭源核心暴露的 gRPC 服务，承载真实业务 services.CustomerService。
// 网关只调这里，永远不接触 core 内的 models/services 实现。
type CustomerServer struct {
	pb.UnimplementedCustomerServiceServer
	srv *services.CustomerService
}

func NewCustomerServer() *CustomerServer {
	return &CustomerServer{srv: services.NewCustomer()}
}

func (s *CustomerServer) Get(ctx context.Context, req *pb.GetCustomerRequest) (*pb.Customer, error) {
	c, err := s.srv.Get(services.SyncTenantCtx(context.Background(), req.GetCorpId()), req.GetExtId(), req.GetCorpId(), true)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return toPB(&c), nil
}

func (s *CustomerServer) Query(ctx context.Context, req *pb.QueryCustomerRequest) (*pb.QueryCustomerResponse, error) {
	q := requests.QueryCustomerReq{
		Name:  req.GetKeyword(),
		Pager: app.Pager{Page: int(req.GetPage()), PageSize: int(req.GetPageSize())},
	}
	if q.Pager.Page == 0 {
		q.Pager.Page = 1
	}
	if q.Pager.PageSize == 0 {
		q.Pager.PageSize = 20
	}
	customers, total, err := s.srv.Query(services.SyncTenantCtx(context.Background(), req.GetCorpId()), q, req.GetCorpId(), &q.Pager)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	items := make([]*pb.Customer, 0, len(customers))
	for i := range customers {
		items = append(items, toPB(customers[i]))
	}
	return &pb.QueryCustomerResponse{Items: items, Total: total}, nil
}

// toPB 把真实 models.Customer 映射为契约 pb.Customer。试点只暴露核心字段，可按需扩展。
func toPB(c *models.Customer) *pb.Customer {
	return &pb.Customer{
		ExtId:     c.ExtID,
		Name:      c.Name,
		CorpId:    c.ExtCorpID,
		Avatar:    c.Avatar,
		CreatedAt: c.CreatedAt.Unix(),
	}
}
