// 手工数据库迁移入口。
//
// 按配置连接数据库并运行 AutoMigrate，再应用 migrations/*.sql。
//
// 幂等：只执行未记录进 schema_migrations 的 SQL 文件。
//
// 用法（在 core 目录下）：
//
//	go run ./cmd/migrate
//	go run ./cmd/migrate -schema-only // 不额外导入旧企微数据到 CRM
//
// 说明：本命令只建库连接 + 迁移，不初始化种子数据，也不依赖 redis。
package main

import (
	"flag"
	"log"
	"time"

	"core/app/constants"
	"core/app/models"
	log2 "core/common/log"
	"core/conf"
	"core/internal/migrate"
)

func init() {
	if err := conf.SetupSetting(); err != nil {
		log.Fatalf("[migrate] conf.SetupSetting: %v", err)
	}
	time.Local = constants.PRCLocation
	log2.SetupLogger(conf.Settings.App.Env)
}

func main() {
	schemaOnly := flag.Bool("schema-only", false, "apply schema migrations without importing legacy CRM data")
	flag.Parse()

	// 连接 + AutoMigrate（新增字段/表）。种子数据初始化在正常启动时做，这里不做。
	models.SetupSchema()

	if err := migrate.Apply(models.DB); err != nil {
		log.Fatalf("[migrate] apply: %v", err)
	}
	log.Println("[migrate] schema migrations done")
	if *schemaOnly {
		return
	}

	// 回填存量企微数据到 CRM 原生表（幂等，无企微数据时为空操作）。
	if err := migrate.BackfillCRM(models.DB); err != nil {
		log.Fatalf("[migrate] backfill crm: %v", err)
	}
	log.Println("[migrate] crm backfill done")
}
