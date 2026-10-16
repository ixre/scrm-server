package models

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"core/conf"
	"core/migrations"
)

// Opt in on an isolated database. This measures representative database paths,
// not end-to-end HTTP capacity or all CRM filters.
func TestCRMScaleQueries(t *testing.T) {
	if os.Getenv("SCRM_SCALE_ROWS") == "" {
		t.Skip("set SCRM_SCALE_ROWS to enable large data verification")
	}
	rows, err := strconv.Atoi(os.Getenv("SCRM_SCALE_ROWS"))
	if err != nil || rows < 100000 || rows > 10000000 {
		t.Fatal("SCRM_SCALE_ROWS must be between 100000 and 10000000")
	}
	db := aggregateTestDB(t)
	if err := db.AutoMigrate(&CrmCustomer{}, &CrmLead{}, &CrmOpportunity{}, &CrmFollowup{}, &CrmOrder{}, &CrmReceivables{}, &CrmQuote{}); err != nil {
		t.Fatal(err)
	}
	indexes, err := migrations.ModelIndexStatements()
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range indexes {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	connection, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	configureConnectionPool(connection, conf.DBConfig{})
	// One large tenant owns 20 percent of all rows. The remainder spans 1000 tenants.
	heavy := rows / 5
	t.Logf("seeding %d customers and %d followups across 1001 tenants", rows, 2*rows)
	if err := db.Exec(`INSERT INTO crm_customer (id,tenant_id,owner_user_id,name,score,pool_status,shared_staff_ids,created_at,updated_at)
		SELECT n, CASE WHEN n<=? THEN 1 ELSE 2+(n%1000) END, 10+(n%20), 'customer-'||n, n%100, 0,
		CASE WHEN n%97=0 THEN '["10"]'::jsonb ELSE '[]'::jsonb END, now(),now()
		FROM generate_series(1,?) n`, heavy, rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO crm_followup (id,tenant_id,customer_id,user_id,content,created_at,updated_at)
		SELECT n, CASE WHEN ((n-1)%?)+1<=? THEN 1 ELSE 2+((((n-1)%?)+1)%1000) END,
		((n-1)%?)+1,10,'followup-'||n,now()-(n%10000)*interval '1 second',now()
		FROM generate_series(1,?) n`, rows, heavy, rows, rows, 2*rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"crm_customer", "crm_followup"} {
		if err := db.Exec("ANALYZE " + table).Error; err != nil {
			t.Fatal(err)
		}
	}
	queries := map[string]string{
		"tenant_list":       "SELECT * FROM crm_customer WHERE tenant_id=1 AND deleted_at IS NULL ORDER BY score DESC,id DESC LIMIT 20",
		"owner_list":        "SELECT * FROM crm_customer WHERE tenant_id=1 AND owner_user_id=10 AND deleted_at IS NULL ORDER BY score DESC,id DESC LIMIT 20",
		"visible_count":     `SELECT count(*) FROM crm_customer WHERE tenant_id=1 AND deleted_at IS NULL AND (owner_user_id=10 OR shared_staff_ids @> '["10"]'::jsonb)`,
		"customer_timeline": "SELECT * FROM crm_followup WHERE tenant_id=1 AND customer_id=100 AND deleted_at IS NULL ORDER BY created_at DESC,id DESC LIMIT 20",
	}
	plans := make(map[string]json.RawMessage)
	for _, name := range []string{"tenant_list", "owner_list", "visible_count", "customer_timeline"} {
		sql := queries[name]
		var plan string
		if err := db.Raw("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) " + sql).Scan(&plan).Error; err != nil {
			t.Fatal(err)
		}
		plans[name] = json.RawMessage(plan)
		if strings.Contains(plan, `"Node Type": "Seq Scan"`) {
			t.Errorf("%s scans the whole table: %s", name, plan)
		}
	}
	var warmCount string
	if err := db.Raw("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) " + queries["visible_count"]).Scan(&warmCount).Error; err != nil {
		t.Fatal(err)
	}
	plans["visible_count_repeat"] = json.RawMessage(warmCount)
	const workers, requests = 32, 20
	readTimes, writeTimes := make(chan float64, workers*requests), make(chan float64, workers*requests)
	failures := make(chan error, workers*requests*2)
	start := time.Now()
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for n := 0; n < requests; n++ {
				tenant := 2 + (worker*requests+n)%1000
				if worker%4 == 0 {
					tenant = 1
				}
				var customers []CrmCustomer
				begin := time.Now()
				err := db.Where("tenant_id = ?", tenant).Order("score DESC,id DESC").Limit(20).Find(&customers).Error
				readTimes <- float64(time.Since(begin).Microseconds()) / 1000
				if err != nil {
					failures <- err
					continue
				}
				for _, customer := range customers {
					if customer.TenantID != strconv.Itoa(tenant) {
						failures <- fmt.Errorf("tenant scope leaked")
						break
					}
				}
				if n%4 == 0 && len(customers) > 0 {
					id := customers[worker%len(customers)].ID
					begin = time.Now()
					result := db.Model(&CrmCustomer{}).Where("tenant_id = ? AND id = ?", tenant, id).Update("remark", "scale-verification")
					writeTimes <- float64(time.Since(begin).Microseconds()) / 1000
					if result.Error != nil {
						failures <- result.Error
					} else if result.RowsAffected != 1 {
						failures <- fmt.Errorf("scoped write missed row")
					}
				}
			}
		}(worker)
	}
	wg.Wait()
	elapsed := time.Since(start)
	close(readTimes)
	close(writeTimes)
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	stats := func(times chan float64) map[string]any {
		var samples []float64
		for v := range times {
			samples = append(samples, v)
		}
		sort.Float64s(samples)
		if len(samples) == 0 {
			return map[string]any{"count": 0}
		}
		return map[string]any{"count": len(samples), "p50_ms": samples[len(samples)/2], "p95_ms": samples[(len(samples)-1)*95/100], "max_ms": samples[len(samples)-1]}
	}
	report := map[string]any{"customer_rows": rows, "followup_rows": 2 * rows, "tenants": 1001, "largest_tenant_customers": heavy, "workers": workers, "pool_max_connections": connection.Stats().MaxOpenConnections, "elapsed_seconds": elapsed.Seconds(), "reads": stats(readTimes), "writes": stats(writeTimes), "plans": plans}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("SCRM_SCALE_REPORT"); path != "" {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("database scale results: %s", data)
}
