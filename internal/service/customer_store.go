package service

import (
	"strings"

	"core/api/pb"
)

// customerStore 内存数据占位，模拟 DB。后续迁移真实 models 时整体替换。
type customerStore struct {
	byCorp map[string][]*pb.Customer // corp_id -> customers
}

func newCustomerStore() *customerStore {
	return &customerStore{byCorp: map[string][]*pb.Customer{
		"corp-demo": {
			{ExtId: "wm1001", Name: "张三", CorpId: "corp-demo", Avatar: "https://example.com/zhangsan.png", CreatedAt: 1700000000},
			{ExtId: "wm1002", Name: "李四", CorpId: "corp-demo", Avatar: "https://example.com/lisi.png", CreatedAt: 1700000100},
			{ExtId: "wm1003", Name: "王五", CorpId: "corp-demo", Avatar: "https://example.com/wangwu.png", CreatedAt: 1700000200},
		},
	}}
}

func (s *customerStore) get(extID, corpID string) (*pb.Customer, bool) {
	for _, c := range s.byCorp[corpID] {
		if c.ExtId == extID {
			return c, true
		}
	}
	return nil, false
}

func (s *customerStore) query(corpID string, page, pageSize int64, keyword string) ([]*pb.Customer, int64) {
	all := s.byCorp[corpID]
	var filtered []*pb.Customer
	for _, c := range all {
		if keyword == "" || strings.Contains(c.Name, keyword) {
			filtered = append(filtered, c)
		}
	}
	total := int64(len(filtered))

	if pageSize <= 0 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []*pb.Customer{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return filtered[start:end], total
}
