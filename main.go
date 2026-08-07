package main

import (
	"log"
	"net"
	"time"

	"core/api/pb"
	"core/app/constants"
	"core/app/models"
	"core/common/id_generator"
	log2 "core/common/log"
	"core/common/redis"
	"core/conf"
	"core/internal/grpcserver"
	"google.golang.org/grpc"
)

// 闭源核心入口：初始化与 openscrm 一致的地基（conf/db/redis），
// 然后对外提供 gRPC 服务。试点阶段不启动 consumers/tasks/delay_queue（依赖企微回调），
// 生产需按 openscrm main.go 完整初始化。
func init() {
	if err := conf.SetupSetting(); err != nil {
		log.Fatalf("[core] conf.SetupSetting: %v", err)
	}
	time.Local = constants.PRCLocation
	log2.SetupLogger(conf.Settings.App.Env)
	id_generator.SetupIDGenerator()
	redis.Setup(conf.Settings.Redis.Host, conf.Settings.Redis.Password, conf.Settings.Redis.DBNumber)
	models.SetupDB()
}

func main() {
	lis, err := net.Listen("tcp", ":9002")
	if err != nil {
		log.Fatalf("[core] listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterCustomerServiceServer(s, grpcserver.NewCustomerServer())
	pb.RegisterStaffServiceServer(s, grpcserver.NewStaffServer())

	log.Printf("[core] closed-source gRPC server (real business) listening on :9002")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("[core] serve: %v", err)
	}
}
