package dispatch

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRegistryRejectsDuplicateActions(t *testing.T) {
	r := Registry{}
	r.Register("test", "Get", nil)
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate registration accepted")
		}
	}()
	r.Register("test", "Get", nil)
}
func TestDashboardContracts(t *testing.T) {
	root := "../../../scrm-app/dashboard/src"
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("dashboard repository is not checked out alongside core")
	}
	pattern := regexp.MustCompile("[\x27\x22\x60]/(?:api/v1/biz/)?(crm_[A-Za-z0-9_]+)/(\\w+)[\x27\x22\x60]")
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !(strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx")) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
			count++
			if _, ok := Reg.Lookup(match[1], match[2]); !ok {
				t.Errorf("%s: unregistered %s/%s", path, match[1], match[2])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no frontend contracts found")
	}
}
