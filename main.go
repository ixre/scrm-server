package main

import (
	"log"
	"net"
	"time"

	"core/api/pb"
	"core/app/callback"
	"core/app/constants"
	"core/app/consumers"
	"core/app/models"
	"core/app/services"
	"core/app/tasks"
	"core/common/delay_queue"
	"core/common/id_generator"
	log2 "core/common/log"
	"core/common/redis"
	"core/conf"
	"core/hotreload"
	"core/internal/grpcserver"
	"core/internal/migrate"

	"google.golang.org/grpc"
)

// 闭源核心入口：初始化与 openscrm 一致的地基（conf/db/redis），
// 然后对外提供 gRPC 服务。定时任务（tasks）已启动：公海自动回收 + openscrm 遗产 cron。
// 队列消费者与定时任务在实际服务进程中启动，避免热更新监督进程重复执行。
func init() {
	if err := conf.SetupSetting(); err != nil {
		log.Fatalf("[core] conf.SetupSetting: %v", err)
	}
	time.Local = constants.PRCLocation
	log2.SetupLogger(conf.Settings.App.Env)
	id_generator.SetupIDGenerator()
	redis.Setup(conf.Settings.Redis.Host, conf.Settings.Redis.Password, conf.Settings.Redis.DBNumber)
	models.SetupDB()
	if err := migrate.VerifyBackendPolicies(models.DB); err != nil {
		log.Fatalf("[core] %v", err)
	}
	services.SeedDefaultPlans()          // 新租户建立前先保证默认套餐存在
	services.SeedCRMDefaults()           // 为既有租户幂等补齐默认角色/销售阶段
	services.SeedDemoAccount()           // DEV/TEST：仅显式设置强密码时创建演示账号
	services.SeedPlatformAdmin()         // DEV/TEST：仅显式设置强密码时创建平台账号
	services.SeedDefaultPaymentConfigs() // 幂等创建默认支付配置占位（微信/支付宝）
}

func main() {
	hotreload.WatchAndRun(".", runServer)
}

// runServer 启动闭源核心的 gRPC 服务与企微回调 webhook。
func runServer() {
	if err := services.RestoreChannelClients(); err != nil {
		log.Fatalf("restore channel clients: %v", err)
	}
	if conf.Settings.App.AutoSyncWeWorkData {
		go services.Syncs()
	}
	delay_queue.SetupDelayQueue()
	consumers.Start()
	tasks.Start()
	internalSecret, err := grpcserver.InternalSecret(conf.Settings.App.InternalSecret)
	if err != nil {
		log.Fatalf("[core] %v", err)
	}
	lis, err := net.Listen("tcp", ":9001")
	if err != nil {
		log.Fatalf("[core] listen: %v", err)
	}

	// 所有 gRPC 调用须携带网关的内部签名，否则拒绝（身份字段不可被旁路伪造）。
	s := grpc.NewServer(
		grpc.UnaryInterceptor(grpcserver.InternalAuthUnaryInterceptor(internalSecret, time.Now)),
		grpc.StreamInterceptor(grpcserver.RejectStreamInterceptor),
	)
	pb.RegisterCustomerServiceServer(s, grpcserver.NewCustomerServer())
	pb.RegisterStaffServiceServer(s, grpcserver.NewStaffServer())
	pb.RegisterBusinessServiceServer(s, grpcserver.NewBusinessServer())

	// 企微事件回调 webhook（独立端口，按 corp_id 归属租户）。启动失败不影响 gRPC 主服务。
	callback.StartWeComCallbackServer()
	// 小红书事件回调 webhook（独立端口，按 app_id 归属租户）。
	callback.StartXhsCallbackServer()

	log.Printf("[core] closed-source gRPC server (real business) listening on :9001")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("[core] serve: %v", err)
	}
}
