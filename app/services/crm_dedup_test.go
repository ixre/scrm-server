package services

import "testing"
import "core/app/models"

// TestPickPrimary 锁定主记录选择逻辑。
func TestPickPrimary(t *testing.T) {
	cases := []struct {
		name string
		cs   []models.CrmCustomer
		want string
	}{
		{"空列表→空", nil, ""},
		{"有归属非公海→选之", []models.CrmCustomer{
			{Model: models.Model{ID: "a"}, OwnerUserID: "", PoolStatus: 1},
			{Model: models.Model{ID: "b"}, OwnerUserID: "u1", PoolStatus: 0},
		}, "b"},
		{"全无归属→选最新(最后一个)", []models.CrmCustomer{
			{Model: models.Model{ID: "a"}, OwnerUserID: "", PoolStatus: 1},
			{Model: models.Model{ID: "b"}, OwnerUserID: "", PoolStatus: 1},
		}, "b"},
		{"公海但有归属不优先", []models.CrmCustomer{
			{Model: models.Model{ID: "a"}, OwnerUserID: "u1", PoolStatus: 1},
			{Model: models.Model{ID: "b"}, OwnerUserID: "", PoolStatus: 0},
		}, "b"}, // a 有归属但是公海，不优先；b 非公海但无归属，也不优先 → 回退最新=b
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pickPrimary(c.cs); got != c.want {
				t.Fatalf("pickPrimary = %q, want %q", got, c.want)
			}
		})
	}
}
