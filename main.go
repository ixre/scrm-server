package main

import (
	"log"
	"net"
	"time"

	"core/api/pb"
	"core/app/callback"
	"core/app/constants"
	"core/app/models"
	"core/app/services"
	"core/app/tasks"
	"core/common/id_generator"
	log2 "core/common/log"
	"core/common/redis"
	"core/conf"
	"core/hotreload"
	"core/internal/grpcserver"

	"google.golang.org/grpc"
)

// 闭源核心入口：初始化与 openscrm 一致的地基（conf/db/redis），
// 然后对外提供 gRPC 服务。定时任务（tasks）已启动：公海自动回收 + openscrm 遗产 cron。
// consumers/delay_queue 依赖企微回调，按需启动。
func init() {
	if err := conf.SetupSetting(); err != nil {
		log.Fatalf("[core] conf.SetupSetting: %v", err)
	}
	time.Local = constants.PRCLocation
	log2.SetupLogger(conf.Settings.App.Env)
	id_generator.SetupIDGenerator()
	redis.Setup(conf.Settings.Redis.Host, conf.Settings.Redis.Password, conf.Settings.Redis.DBNumber)
	models.SetupDB()
	services.SeedCRMDefaults()   // 为既有租户幂等补齐默认角色/销售阶段
	services.SeedDemoAccount()   // 非生产：幂等创建演示租户与登录账号（demo/admin/123456）
	services.SeedPlatformAdmin() // 非生产：幂等创建平台超管账号（platform/admin123）
	tasks.Start()                // 启动定时任务（公海回收 + openscrm 遗产 cron）
}

func main() {
	hotreload.WatchAndRun(".", runServer)
}

// runServer 启动闭源核心的 gRPC 服务与企微回调 webhook。
func runServer() {
	lis, err := net.Listen("tcp", ":9002")
	if err != nil {
		log.Fatalf("[core] listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterCustomerServiceServer(s, grpcserver.NewCustomerServer())
	pb.RegisterStaffServiceServer(s, grpcserver.NewStaffServer())
	pb.RegisterBusinessServiceServer(s, grpcserver.NewBusinessServer())

	// 企微事件回调 webhook（独立端口，按 corp_id 归属租户）。启动失败不影响 gRPC 主服务。
	callback.StartWeComCallbackServer()
	// 小红书事件回调 webhook（独立端口，按 app_id 归属租户）。
	callback.StartXhsCallbackServer()

	log.Printf("[core] closed-source gRPC server (real business) listening on :9002")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("[core] serve: %v", err)
	}
}
