package service

import (
	"context"

	"core/api/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CustomerService 是闭源核心的客户业务逻辑。
// 试点阶段用内存数据占位，后续按此结构把 openscrm 的
// models + services（DB/gorm/企微同步）迁移进来，替换 store。
type CustomerService struct {
	pb.UnimplementedCustomerServiceServer
	store *customerStore
}

func NewCustomerService() *CustomerService {
	return &CustomerService{store: newCustomerStore()}
}

func (s *CustomerService) Get(ctx context.Context, req *pb.GetCustomerRequest) (*pb.Customer, error) {
	c, ok := s.store.get(req.GetExtId(), req.GetCorpId())
	if !ok {
		return nil, status.Error(codes.NotFound, "customer not found")
	}
	return c, nil
}

func (s *CustomerService) Query(ctx context.Context, req *pb.QueryCustomerRequest) (*pb.QueryCustomerResponse, error) {
	items, total := s.store.query(req.GetCorpId(), req.GetPage(), req.GetPageSize(), req.GetKeyword())
	return &pb.QueryCustomerResponse{Items: items, Total: total}, nil
}
