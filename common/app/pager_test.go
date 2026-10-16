package app

import "testing"

func TestPagerBoundsLargeRequests(t *testing.T) {
	for _, size := range []int{1001, 1000000, int(^uint(0) >> 1)} {
		p := (&Pager{Page: 2, PageSize: size}).SetDefault()
		if p.GetLimit() != MaxPageSize || p.GetOffset() != MaxPageSize {
			t.Fatalf("unbounded page: %+v", p)
		}
	}
	p := (&Pager{}).SetDefault()
	if p.GetLimit() != 10 || p.GetOffset() != 0 {
		t.Fatalf("default changed: %+v", p)
	}
	p = (&Pager{Page: 2, PageSize: 1000}).SetDefault()
	if p.GetLimit() != 1000 {
		t.Fatal("existing large dropdown pages must remain supported")
	}
}
