package grpcserver

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"core/api/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	testSecret = "0123456789abcdef0123456789abcdef"
	testMethod = "/business.v1.BusinessService/Invoke"
)

// goldenSignature 与 gateway/internal/client/internal_auth_test.go 中的值必须一致，保证两端签名格式兼容。
const goldenSignature = "f15970e6b1d39d567066a810c147d4ab7b1398f4cfde52c20a911c83cd5c98d1"

func goldenRequest() *pb.BusinessRequest {
	return &pb.BusinessRequest{
		Service: "crm_customer", Action: "Query", TenantId: "t1", UserId: "u1",
		CorpId: "c1", StaffId: "s1", Params: []byte(`{"query":{}}`),
	}
}

func TestSignInternalCallGoldenVector(t *testing.T) {
	sig, err := signInternalCall([]byte(testSecret), testMethod, 1700000000, "10.0.0.1", goldenRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(sig); got != goldenSignature {
		t.Fatalf("signature = %s, want %s", got, goldenSignature)
	}
}

func TestInternalSignatureProtectsSessionVersion(t *testing.T) {
	now := time.Unix(1700000000, 0)
	sig, err := signInternalCall([]byte(testSecret), testMethod, now.Unix(), "", goldenRequest(), "7")
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.Pairs(mdAuthTimestamp, "1700000000", mdAuthSignature, base64.RawURLEncoding.EncodeToString(sig), "x-scrm-session-version", "7")
	if err := verifyInternalCall(metadata.NewIncomingContext(context.Background(), md), []byte(testSecret), testMethod, goldenRequest(), now); err != nil {
		t.Fatal(err)
	}
	md.Set("x-scrm-session-version", "8")
	if verifyInternalCall(metadata.NewIncomingContext(context.Background(), md), []byte(testSecret), testMethod, goldenRequest(), now) == nil {
		t.Fatal("tampered session version accepted")
	}
	md.Delete("x-scrm-session-version")
	if verifyInternalCall(metadata.NewIncomingContext(context.Background(), md), []byte(testSecret), testMethod, goldenRequest(), now) == nil {
		t.Fatal("removed session version accepted")
	}
}

func signedCtx(t *testing.T, ts time.Time, clientIP string, req proto.Message) context.Context {
	t.Helper()
	sig, err := signInternalCall([]byte(testSecret), testMethod, ts.Unix(), clientIP, req)
	if err != nil {
		t.Fatal(err)
	}
	md := metadata.Pairs(mdAuthTimestamp, strconv.FormatInt(ts.Unix(), 10), mdAuthSignature, base64.RawURLEncoding.EncodeToString(sig))
	if clientIP != "" {
		md.Append(mdClientIP, clientIP)
	}
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestInternalAuthUnaryInterceptor(t *testing.T) {
	now := time.Unix(1700000000, 0)
	interceptor := InternalAuthUnaryInterceptor([]byte(testSecret), func() time.Time { return now })
	info := &grpc.UnaryServerInfo{FullMethod: testMethod}

	forged := goldenRequest()
	forged.IsPlatform, forged.PlatformId = true, "p1"

	cases := []struct {
		name string
		ctx  context.Context
		req  proto.Message
		info *grpc.UnaryServerInfo
		ok   bool
	}{
		{"valid", signedCtx(t, now, "10.0.0.1", goldenRequest()), goldenRequest(), info, true},
		{"valid without client ip", signedCtx(t, now, "", goldenRequest()), goldenRequest(), info, true},
		{"no metadata", context.Background(), goldenRequest(), info, false},
		{"forged identity fields", signedCtx(t, now, "10.0.0.1", goldenRequest()), forged, info, false},
		{"tampered client ip", metadata.NewIncomingContext(context.Background(), metadata.Join(
			func() metadata.MD {
				md, _ := metadata.FromIncomingContext(signedCtx(t, now, "", goldenRequest()))
				return md
			}(),
			metadata.Pairs(mdClientIP, "1.2.3.4"))), goldenRequest(), info, false},
		{"different method", signedCtx(t, now, "", goldenRequest()), goldenRequest(), &grpc.UnaryServerInfo{FullMethod: "/core.StaffService/Get"}, false},
		{"expired", signedCtx(t, now.Add(-2*maxClockSkew), "", goldenRequest()), goldenRequest(), info, false},
		{"future", signedCtx(t, now.Add(2*maxClockSkew), "", goldenRequest()), goldenRequest(), info, false},
		{"wrong secret", func() context.Context {
			sig, _ := signInternalCall([]byte("another-secret-another-secret-xx"), testMethod, now.Unix(), "", goldenRequest())
			return metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				mdAuthTimestamp, strconv.FormatInt(now.Unix(), 10), mdAuthSignature, base64.RawURLEncoding.EncodeToString(sig)))
		}(), goldenRequest(), info, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			_, err := interceptor(tc.ctx, tc.req, tc.info, func(context.Context, any) (any, error) {
				called = true
				return nil, nil
			})
			if tc.ok != (err == nil) || tc.ok != called {
				t.Fatalf("ok=%v, err=%v, handler called=%v", tc.ok, err, called)
			}
			if !tc.ok && status.Code(err) != codes.Unauthenticated {
				t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
			}
		})
	}
}
