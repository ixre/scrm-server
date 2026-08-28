package grpcserver

import (
	"context"
	"core/app/constants"
	"core/app/services"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"

	"core/api/pb"
	"core/app/models"
	"core/common/identity"
	"core/common/we_work"
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
	if (service == "crm_auth" || service == "crm_platform") && action == "Login" {
		if rejection := checkLoginTraffic(ctx, service, req.GetParams()); rejection != nil {
			return rejection, nil
		}
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
	if service == "crm_auth" && (action == "Logout" || action == "ValidateSession") && req.GetUserId() == "" && !req.GetIsPlatform() && req.GetCorpId() != "" && req.GetStaffId() != "" {
		if action == "ValidateSession" {
			if _, err := services.ResolveWecomConfig(ctx, req.GetCorpId()); err != nil {
				return &pb.BusinessResponse{Code: deniedCode(err), Msg: err.Error()}, nil
			}
		}
		staff, err := (&models.Staff{}).Get(req.GetStaffId(), req.GetCorpId(), false)
		if err != nil || staff.Status == constants.UserStatusDeactivated || strconv.FormatInt(staff.AuthVersion, 10) != sessionVersion(ctx) {
			return &pb.BusinessResponse{Code: 401, Msg: "登录已失效"}, nil
		}
		if action == "ValidateSession" {
			return &pb.BusinessResponse{Data: []byte("null")}, nil
		}
		if err := services.LogoutWecom(ctx, req.GetCorpId(), req.GetStaffId()); err != nil {
			return &pb.BusinessResponse{Code: 1, Msg: err.Error()}, nil
		}
		return &pb.BusinessResponse{Data: []byte("null")}, nil
	}
	if !publicBusinessAction(service, action) && (strings.HasPrefix(service, "crm_") || req.GetTenantId() != "" || req.GetIsPlatform()) {
		allowExpired := expiredSessionAction(service, action)
		var code int32
		var err error
		if service == "crm_auth" && action == "Logout" {
			code, err = services.ValidateLogoutSession(ctx, sessionVersion(ctx))
		} else {
			code, err = services.ValidateBusinessSession(ctx, allowExpired, sessionVersion(ctx))
		}
		if err != nil {
			if errors.Is(err, models.ErrTenantExpired) {
				code = 402
			}
			return &pb.BusinessResponse{Code: code, Msg: err.Error()}, nil
		}
	}
	if service == "crm_smart_card" && publicBusinessAction(service, action) {
		if rejection := checkPublicTraffic(ctx, action, req.GetParams()); rejection != nil {
			return rejection, nil
		}
	}
	corpID, staffID := req.GetCorpId(), req.GetStaffId()
	if wecomBusinessAction(service) {
		cfg, err := services.ResolveWecomConfig(ctx, corpID)
		if err != nil {
			return &pb.BusinessResponse{Code: deniedCode(err), Msg: err.Error()}, nil
		}
		corpID = cfg.CorpID
		ctx = services.SyncTenantCtx(ctx, corpID)
		ctx = we_work.WithCorpID(ctx, corpID)
		if identity.UserID(ctx) == "" && staffID != "" {
			staff, err := (&models.Staff{}).Get(staffID, corpID, false)
			if err != nil || staff.Status == constants.UserStatusDeactivated || strconv.FormatInt(staff.AuthVersion, 10) != sessionVersion(ctx) {
				return &pb.BusinessResponse{Code: 401, Msg: "企微员工已离职或登录已失效"}, nil
			}
		}
		if staffID == "" && identity.UserID(ctx) != "" {
			var link models.ChannelLink
			if err := models.MustFor(ctx).Where("channel = 'wecom' AND entity_type = 'user' AND entity_id = ? AND ext_corp_id = ?", identity.UserID(ctx), corpID).First(&link).Error; err == nil {
				staffID = link.ExtID
			}
		}
	}
	if !publicBusinessAction(service, action) {
		if err := services.ValidateBusinessEntitlement(ctx, service, action); err != nil {
			return &pb.BusinessResponse{Code: deniedCode(err), Msg: err.Error()}, nil
		}
	}
	ctx = services.WithBusinessPermission(ctx, service, action)
	if !publicBusinessAction(service, action) {
		if err := services.ValidateBusinessPermission(ctx, service, action); err != nil {
			return &pb.BusinessResponse{Code: deniedCode(err), Msg: err.Error()}, nil
		}
	}
	if wecomBusinessAction(service) && service != "js_api" {
		if err := services.ValidateWecomBusinessPermission(ctx, service, action, corpID, staffID, req.GetParams()); err != nil {
			return &pb.BusinessResponse{Code: deniedCode(err), Msg: err.Error()}, nil
		}
	}
	res, err := a(ctx, corpID, staffID, req.GetParams())
	if err != nil {
		// 业务错误走响应码透传给网关，由网关映射为 HTTP 状态；不用 gRPC status。
		code := int32(1)
		if errors.Is(err, services.ErrPermissionDenied) {
			code = 403
		}
		if errors.Is(err, models.ErrTenantExpired) {
			code = 402
		}
		return &pb.BusinessResponse{Code: code, Msg: err.Error()}, nil
	}
	data, err := json.Marshal(res)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal result of %s/%s: %v", service, action, err)
	}
	return &pb.BusinessResponse{Data: data}, nil
}

func wecomBusinessAction(service string) bool {
	return !strings.HasPrefix(service, "crm_") && !strings.HasPrefix(service, "internal_") && service != "login" && service != "util" && service != "permission"
}

func publicBusinessAction(service, action string) bool {
	if (service == "crm_auth" || service == "crm_platform") && action == "Login" {
		return true
	}
	return service == "crm_smart_card" && (action == "PublicGet" || action == "PublicSubmitLead" || action == "PublicLike")
}

// 套餐到期映射为专用业务码 402（HTTP Payment Required 语义），
// 与权限不足的通用 403 区分，前端据此全局引导续费。
func deniedCode(err error) int32 {
	if errors.Is(err, models.ErrTenantExpired) {
		return 402
	}
	return 403
}

// Keep account and billing entry points available so an expired tenant can renew.
func expiredSessionAction(service, action string) bool {
	return service == "crm_plan" && action == "List" || service == "crm_tenant_order" || service == "crm_tenant" && action == "GetCurrentInfo" || service == "crm_user" && action == "GetCurrent" || service == "crm_auth" && (action == "ChangePassword" || action == "Logout")
}

func sessionVersion(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get("x-scrm-session-version")
	if len(values) != 1 {
		return ""
	}
	return values[0]
}
