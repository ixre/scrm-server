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

// StaffServer 承载真实业务 services.StaffService。
type StaffServer struct {
	pb.UnimplementedStaffServiceServer
	srv *services.StaffService
}

func NewStaffServer() *StaffServer {
	return &StaffServer{srv: services.NewStaffService()}
}

func (s *StaffServer) Get(ctx context.Context, req *pb.GetStaffRequest) (*pb.Staff, error) {
	st, err := s.srv.Get(req.GetExtId(), req.GetCorpId())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return staffToPB(st), nil
}

func (s *StaffServer) Query(ctx context.Context, req *pb.QueryStaffRequest) (*pb.QueryStaffResponse, error) {
	q := requests.QueryStaffReq{
		Name:  req.GetName(),
		Pager: app.Pager{Page: int(req.GetPage()), PageSize: int(req.GetPageSize())},
	}
	if q.Pager.Page == 0 {
		q.Pager.Page = 1
	}
	if q.Pager.PageSize == 0 {
		q.Pager.PageSize = 20
	}
	items, total, err := s.srv.Query(q, req.GetCorpId(), &app.Sorter{}, &q.Pager)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*pb.Staff, 0, len(items))
	for i := range items {
		out = append(out, staffToPB(items[i]))
	}
	return &pb.QueryStaffResponse{Items: out, Total: total}, nil
}

func staffToPB(s *models.Staff) *pb.Staff {
	if s == nil {
		return &pb.Staff{}
	}
	return &pb.Staff{
		ExtId:     s.ExtID,
		Name:      s.Name,
		ExtCorpId: s.ExtCorpID,
		AvatarUrl: s.AvatarURL,
		Mobile:    s.Mobile,
		RoleType:  s.RoleType,
		Status:    int32(s.Status),
	}
}
