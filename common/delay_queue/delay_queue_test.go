package delay_queue

import (
	"core/app/constants"
	"core/conf"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestPush(t *testing.T) {
	addr := os.Getenv("SCRM_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set SCRM_TEST_REDIS_ADDR to an isolated Redis instance")
	}
	if err := conf.SetupTestSetting(); err != nil {
		t.Fatal(err)
	}
	conf.Settings.Redis.Host = addr
	conf.Settings.Redis.Password = ""
	conf.Settings.Redis.DBNumber = 0
	conf.Settings.Redis.ReadTimeout = 5 * time.Second
	suffix := fmt.Sprint(time.Now().UnixNano())
	conf.Settings.DelayQueue.BucketSize = 1
	conf.Settings.DelayQueue.BucketName = "test_" + suffix + "_bucket_%d"
	conf.Settings.DelayQueue.QueueName = "test_" + suffix + "_queue_%s"
	conf.Settings.DelayQueue.QueueBlockTimeout = 5
	SetupDelayQueue()
	topic := constants.Topic("test_" + suffix)
	job := Job{Topic: topic, ID: "test_" + suffix, ExecuteAt: time.Now().Unix(), TTR: 30, Body: "test"}
	t.Cleanup(func() {
		for _, timer := range timers {
			timer.Stop()
		}
		_ = Remove(job.ID)
		_ = Rdb.Close()
	})
	if err := Add(job); err != nil {
		t.Fatal(err)
	}
	received, err := Listen(topic)
	if err != nil {
		t.Fatal(err)
	}
	if received.ID != job.ID || received.Body != job.Body || received.Topic != job.Topic {
		t.Fatalf("wrong job: %+v", received)
	}
}
