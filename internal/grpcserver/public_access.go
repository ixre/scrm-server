package grpcserver

import (
	"context"
	"encoding/json"

	"core/api/pb"
	"core/common/publiclimit"
	cacheredis "core/common/redis"
	"google.golang.org/grpc/metadata"
)

func checkPublicTraffic(ctx context.Context, action string, params []byte) *pb.BusinessResponse {
	if len(params) > 16<<10 {
		return &pb.BusinessResponse{Code: 413, Msg: "提交内容过大"}
	}
	kind := "view"
	if action == "PublicSubmitLead" {
		kind = "lead"
	}
	if action == "PublicLike" {
		kind = "like"
	}
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get("x-scrm-client-ip")
	clientIP := ""
	if len(values) == 1 {
		clientIP = values[0]
	}
	limiter := publiclimit.Limiter{Client: cacheredis.RedisClient}
	// A nil concrete Redis pointer in an interface is non-nil. Check it before invoking the limiter.
	if cacheredis.RedisClient == nil {
		return &pb.BusinessResponse{Code: 503, Msg: "公开入口暂不可用，请稍后重试"}
	}
	wait, err := limiter.Allow(ctx, kind, clientIP, publiclimit.Budgets(kind)...)
	if err != nil {
		return &pb.BusinessResponse{Code: 503, Msg: "公开入口暂不可用，请稍后重试"}
	}
	if wait > 0 {
		data, _ := json.Marshal(map[string]any{"retry_after": publiclimit.RetrySeconds(wait)})
		return &pb.BusinessResponse{Code: 429, Msg: "操作频繁，请稍后重试", Data: data}
	}
	return nil
}
