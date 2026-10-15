package grpcserver

import (
	"context"
	"core/app/services"
	"encoding/json"
	"strings"

	"core/api/pb"
	"core/common/identity"
	"core/internal/dispatch"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BusinessServer 是通用 JSON 透传 gRPC 服务，把 (service, action) 分发给闭源 core 的真实业务。
// 网关只调这里，不接触 core 内 app/models、app/services 实现。
type BusinessServer struct {
	pb.UnimplementedBusinessServiceServer
}

func NewBusinessServer() *BusinessServer { return &BusinessServer{} }

func (s *BusinessServer) Invoke(ctx context.Context, req *pb.BusinessRequest) (*pb.BusinessResponse, error) {
	service, action := req.GetService(), req.GetAction()
	a, ok := dispatch.Reg.Lookup(service, action)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "unknown business action %s/%s", service, action)
	}
	// CRM 渠道无关身份（tenant_id/user_id）经 context 注入，供 CRM Adapter 读取；
	// 既有企微 Adapter 继续用 corpID/staffID 入参，不受影响。
	// is_platform/platform_id 标记平台超管身份，供 crm_tenant 等平台管理 API 鉴权。
	ctx = identity.WithIdentity(ctx, identity.Identity{
		TenantID:   req.GetTenantId(),
		UserID:     req.GetUserId(),
		IsPlatform: req.GetIsPlatform(),
		PlatformID: req.GetPlatformId(),
	})
	if !publicBusinessAction(service, action) && (strings.HasPrefix(service, "crm_") || req.GetTenantId() != "" || req.GetIsPlatform()) {
		allowExpired := expiredSessionAction(service, action)
		if code, err := services.ValidateBusinessSession(ctx, allowExpired); err != nil {
			return &pb.BusinessResponse{Code: code, Msg: err.Error()}, nil
		}
	}
	res, err := a(ctx, req.GetCorpId(), req.GetStaffId(), req.GetParams())
	if err != nil {
		// 业务错误走响应码透传给网关，由网关映射为 HTTP 状态；不用 gRPC status。
		return &pb.BusinessResponse{Code: 1, Msg: err.Error()}, nil
	}
	data, err := json.Marshal(res)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal result of %s/%s: %v", service, action, err)
	}
	return &pb.BusinessResponse{Data: data}, nil
}

func publicBusinessAction(service, action string) bool {
	if (service == "crm_auth" || service == "crm_platform") && action == "Login" {
		return true
	}
	return service == "crm_smart_card" && (action == "PublicGet" || action == "PublicSubmitLead" || action == "PublicLike")
}

// Keep account and billing entry points available so an expired tenant can renew.
func expiredSessionAction(service, action string) bool {
	return service == "crm_plan" && action == "List" || service == "crm_tenant_order" || service == "crm_tenant" && action == "GetCurrentInfo" || service == "crm_user" && action == "GetCurrent" || service == "crm_auth" && action == "ChangePassword"
}
