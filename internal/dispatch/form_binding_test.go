package dispatch

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"core/app/models"
	"core/common/identity"
)

func TestFormTimeBinding(t *testing.T) {
	var req struct {
		models.CrmOrder
		Plans    []models.CrmReceivablesPlan `json:"plans"`
		Customer models.CrmCustomer          `json:"customer"`
	}
	raw := json.RawMessage(`{"signed_at":"2026-10-15","plans":[{"plan_date":"2026-10-16"}],"customer":{"birthday":"1990-02-01","next_followup_at":"2026-10-17 14:30:00"}}`)
	if err := Bind(raw, &req); err != nil {
		t.Fatal(err)
	}
	if req.SignedAt.Format(time.RFC3339) != "2026-10-15T00:00:00+08:00" || req.Plans[0].PlanDate.Day() != 16 || req.Customer.NextFollowupAt.Hour() != 14 || req.Customer.Birthday != "1990-02-01" {
		t.Fatalf("unexpected binding: %+v", req)
	}
	for _, raw := range []string{`{"signed_at":""}`, `{"signed_at":null}`} {
		var order models.CrmOrder
		if err := Bind(json.RawMessage(raw), &order); err != nil || order.SignedAt != nil {
			t.Fatalf("empty optional date: %v", err)
		}
	}
	for _, value := range []string{"2026-02-30", "not-a-date"} {
		var order models.CrmOrder
		if err := Bind(json.RawMessage(`{"signed_at":"`+value+`"}`), &order); err == nil {
			t.Fatal("invalid date accepted")
		}
	}
	var order models.CrmOrder
	if err := Bind(json.RawMessage(`{"signed_at":"2026-10-15T10:00:00Z"}`), &order); err != nil || order.SignedAt.Hour() != 10 {
		t.Fatalf("RFC3339: %v", err)
	}
}

// Exercise every repaired registered adapter before it can touch storage. A nonempty
// channel staff ID must never substitute for the business record ID.
func TestActionIDsComeFromPayload(t *testing.T) {
	names := map[string]bool{}
	paths, err := filepath.Glob("crm*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "bindActionID" {
					names[fn.Name.Name] = true
				}
				return true
			})
		}
	}
	ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: "1", UserID: "10"})
	checked := 0
	for service, actions := range Reg {
		for action, adapter := range actions {
			name := runtime.FuncForPC(reflect.ValueOf(adapter).Pointer()).Name()
			name = name[strings.LastIndex(name, ".")+1:]
			if !names[name] {
				continue
			}
			checked++
			t.Run(service+"/"+action, func(t *testing.T) {
				if _, err := adapter(ctx, "corp", "777", json.RawMessage(`{}`)); err == nil || err.Error() != "id 不能为空" {
					t.Fatalf("missing payload ID: %v", err)
				}
			})
		}
	}
	if checked < 70 {
		t.Fatalf("unexpected adapter coverage: %d", checked)
	}
	if id, err := bindActionID(json.RawMessage(`{"id":"123"}`)); err != nil || id != "123" {
		t.Fatalf("valid ID: %s %v", id, err)
	}
}
