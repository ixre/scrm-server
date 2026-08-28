package delay_queue

import (
	"context"
	"core/conf"
	"github.com/redis/go-redis/v9"
)

var Rdb *redis.Client

func NewRedisClient() {
	Rdb = redis.NewClient(&redis.Options{
		Addr:        conf.Settings.Redis.Host,
		Password:    conf.Settings.Redis.Password, // no password set
		DB:          conf.Settings.Redis.DBNumber,
		ReadTimeout: conf.Settings.Redis.ReadTimeout,
	})
}

// 执行redis命令, 执行完成后连接自动放回连接池
func execRedisCommand(command string, args ...interface{}) (interface{}, error) {
	err := Rdb.Do(context.TODO(), command, args).Err()
	return nil, err
}
