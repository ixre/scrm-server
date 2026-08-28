package grpcserver

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// 网关 → core 的内部调用认证。
//
// core 不再无条件信任 BusinessRequest 中的 tenant_id/user_id/is_platform 等身份字段：
// 网关用共享密钥 SCRM_INTERNAL_SECRET 对「方法 + 时间戳 + 客户端 IP + 请求体」做 HMAC-SHA256，
// core 在拦截器中验签，未签名、签名错误、过期或被篡改的调用一律拒绝（fail closed）。
// 签名格式须与 gateway/internal/client/internal_auth.go 保持一致。
const (
	InternalSecretEnv = "SCRM_INTERNAL_SECRET"

	mdAuthTimestamp = "x-scrm-auth-ts"
	mdAuthSignature = "x-scrm-auth-sig"
	mdClientIP      = "x-scrm-client-ip"

	internalAuthVersion = "scrm-internal-v1"
	maxClockSkew        = 60 * time.Second
)

var errInternalAuth = status.Error(codes.Unauthenticated, "internal caller authentication failed")

// InternalSecret 读取并校验内部调用共享密钥（至少 32 字节）。
// 优先取环境变量 SCRM_INTERNAL_SECRET；未设置时回退到配置文件传入的 fallback。
func InternalSecret(fallbacks ...string) ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv(InternalSecretEnv))
	if secret == "" {
		for _, f := range fallbacks {
			if s := strings.TrimSpace(f); s != "" {
				secret = s
				break
			}
		}
	}
	if len(secret) < 32 {
		return nil, fmt.Errorf("%s env or App.InternalSecret config must contain at least 32 bytes", InternalSecretEnv)
	}
	return []byte(secret), nil
}

// InternalAuthUnaryInterceptor 校验每个 unary 调用均来自持有共享密钥的网关。
func InternalAuthUnaryInterceptor(secret []byte, now func() time.Time) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := verifyInternalCall(ctx, secret, info.FullMethod, req, now()); err != nil {
			return nil, errInternalAuth
		}
		return handler(ctx, req)
	}
}

// RejectStreamInterceptor core 未提供流式接口，任何流式调用直接拒绝，避免绕过 unary 验签。
func RejectStreamInterceptor(any, grpc.ServerStream, *grpc.StreamServerInfo, grpc.StreamHandler) error {
	return status.Error(codes.Unimplemented, "streaming is not supported")
}

func verifyInternalCall(ctx context.Context, secret []byte, method string, req any, now time.Time) error {
	md, _ := metadata.FromIncomingContext(ctx)
	tsValues, sigValues, ipValues := md.Get(mdAuthTimestamp), md.Get(mdAuthSignature), md.Get(mdClientIP)
	if len(tsValues) != 1 || len(sigValues) != 1 || len(ipValues) > 1 {
		return errors.New("missing or duplicated auth metadata")
	}
	ts, err := strconv.ParseInt(tsValues[0], 10, 64)
	if err != nil {
		return err
	}
	if skew := now.Sub(time.Unix(ts, 0)); skew > maxClockSkew || skew < -maxClockSkew {
		return errors.New("auth timestamp out of range")
	}
	got, err := base64.RawURLEncoding.DecodeString(sigValues[0])
	if err != nil {
		return err
	}
	msg, ok := req.(proto.Message)
	if !ok {
		return errors.New("request is not a proto message")
	}
	versions := md.Get("x-scrm-session-version")
	if len(versions) > 1 {
		return errors.New("duplicated session version")
	}
	version := ""
	if len(versions) == 1 {
		version = versions[0]
	}
	clientIP := ""
	if len(ipValues) == 1 {
		clientIP = ipValues[0]
	}
	want, err := signInternalCall(secret, method, ts, clientIP, msg, version)
	if err != nil {
		return err
	}
	if !hmac.Equal(got, want) {
		return errors.New("signature mismatch")
	}
	return nil
}

// signInternalCall 计算内部调用签名。请求体使用确定性 proto 序列化后取 SHA-256，
// 覆盖身份字段与全部入参，任何字段被篡改都会导致验签失败。
func signInternalCall(secret []byte, method string, ts int64, clientIP string, req proto.Message, sessionVersions ...string) ([]byte, error) {
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, err
	}
	bodyHash := sha256.Sum256(body)
	mac := hmac.New(sha256.New, secret)
	fmt.Fprintf(mac, "%s\n%s\n%d\n%s\n%s", internalAuthVersion, method, ts, clientIP, hex.EncodeToString(bodyHash[:]))
	if len(sessionVersions) > 0 && sessionVersions[0] != "" {
		fmt.Fprintf(mac, "\nsession:%s", sessionVersions[0])
	}
	return mac.Sum(nil), nil
}
