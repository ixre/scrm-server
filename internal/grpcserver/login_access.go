package grpcserver

import (
	"context"
	"core/api/pb"
	"core/common/publiclimit"
	cacheredis "core/common/redis"
	"encoding/json"
	"google.golang.org/grpc/metadata"
	"strings"
	"time"
)

func checkLoginTraffic(ctx context.Context, service string, params []byte) *pb.BusinessResponse {
	if len(params) > 4096 {
		return &pb.BusinessResponse{Code: 413, Msg: "登录内容过大"}
	}
	if cacheredis.RedisClient == nil {
		return &pb.BusinessResponse{Code: 503, Msg: "登录服务暂不可用"}
	}
	var input struct {
		TenantCode string `json:"tenant_code"`
		Username   string `json:"username"`
	}
	if json.Unmarshal(params, &input) != nil || len(input.Username) > 64 || len(input.TenantCode) > 64 {
		return &pb.BusinessResponse{Code: 400, Msg: "登录参数无效"}
	}
	md, _ := metadata.FromIncomingContext(ctx)
	ip := ""
	if ips := md.Get(mdClientIP); len(ips) == 1 {
		ip = ips[0]
	}
	limiter := publiclimit.Limiter{Client: cacheredis.RedisClient, Prefix: "scrm:login-rate"}
	wait, err := limiter.Allow(ctx, service, ip, publiclimit.Budget{Limit: 30, Window: time.Minute})
	if err != nil {
		return &pb.BusinessResponse{Code: 503, Msg: "登录服务暂不可用"}
	}
	if wait == 0 {
		wait, err = limiter.AllowKey(ctx, service+":account", input.TenantCode+":"+strings.ToLower(input.Username), publiclimit.Budget{Limit: 10, Window: 5 * time.Minute})
	}
	if err != nil {
		return &pb.BusinessResponse{Code: 503, Msg: "登录服务暂不可用"}
	}
	if wait > 0 {
		return &pb.BusinessResponse{Code: 429, Msg: "尝试次数过多，请稍后再试"}
	}
	return nil
}
