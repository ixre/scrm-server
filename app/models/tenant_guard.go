package models

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"core/common/log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// 租户隔离守卫（fail closed）。
//
// 所有含 tenant_id 列的表，查询/更新/删除必须在顶层 AND 条件中带非空 tenant_id 过滤，
// 新增记录必须带非空 TenantID；否则语句在执行前被拒绝并返回 ErrTenantScopeMissing。
//
// 老 openscrm 表（legacyTenantTables：tenant_id 由数据库触发器按 ext_corp_id 映射写入）也接受
// ext_corp_id 过滤（企业与租户是数据库强制的一对一映射），且使用独立模式 legacyMode：
// 这些表仍有大量按 ext_id 等非企业键的查询，需随企微多企业改造逐步收敛，默认仅记录违规。
//
// 确需跨租户的路径（定时任务扫描、平台超管、渠道回调反查租户、种子数据）必须显式使用
// CrossTenant() / CrossTenantDB(db) / CrossTenantContext(ctx)，让绕过可检索、可审计。
//
// 已知边界：Raw/Exec 原生 SQL 与作为参数传入的子查询不经守卫，须人工保证带租户条件。

// ErrTenantScopeMissing 访问租户表但缺少租户条件。
var ErrTenantScopeMissing = errors.New("tenant scope missing: query on tenant table without tenant_id condition (use models.For(ctx) / TenantScope, or models.CrossTenant() for intentional cross-tenant access)")

// TenantGuard 模式。
const (
	TenantGuardEnforce = "enforce" // 默认：拒绝违规语句
	TenantGuardLog     = "log"     // 仅记录违规（灰度排查用），不拦截
)

type crossTenantKey struct{}

// CrossTenantContext 标记 ctx 允许跨租户访问。仅用于平台/系统级路径。
func CrossTenantContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, crossTenantKey{}, true)
}

func isCrossTenant(ctx context.Context) bool {
	v, _ := ctx.Value(crossTenantKey{}).(bool)
	return v
}

// CrossTenant 返回显式跨租户的 DB 会话（定时任务、平台超管、回调反查租户、种子数据）。
// 注意：后续再调用 WithContext(ctx) 会替换 ctx 并丢失该标记，此时改用 CrossTenantContext(ctx)。
func CrossTenant() *gorm.DB {
	return DB.WithContext(CrossTenantContext(context.Background()))
}

// CrossTenantDB 在已有会话（如事务 tx）上开启跨租户访问。
func CrossTenantDB(db *gorm.DB) *gorm.DB {
	return db.WithContext(CrossTenantContext(db.Statement.Context))
}

// legacyTenantTables 老 openscrm 实体表：tenant_id 由迁移 0015 的触发器按企业映射写入。
var legacyTenantTables = map[string]bool{
	"customer": true, "staff": true, "department": true, "group_chat": true,
	"mass_msg": true, "material": true, "contact_way": true,
}

// tenantGuard 是 GORM 插件。
type tenantGuard struct {
	mode       string
	legacyMode string
	tables     sync.Map // table name -> tenantTable
}

type tenantTable struct {
	legacy bool
}

// TenantGuardConfig 守卫模式。空值分别默认为 enforce（CRM 表）与 log（老企微表）。
type TenantGuardConfig struct {
	Mode       string
	LegacyMode string
}

// UseTenantGuard 在 db 上注册租户守卫，并预登记 models 中所有含 tenant_id 的表。
func UseTenantGuard(db *gorm.DB, cfg TenantGuardConfig) error {
	if cfg.Mode == "" {
		cfg.Mode = TenantGuardEnforce
	}
	if cfg.LegacyMode == "" {
		cfg.LegacyMode = TenantGuardLog
	}
	for _, m := range []string{cfg.Mode, cfg.LegacyMode} {
		if m != TenantGuardEnforce && m != TenantGuardLog {
			return fmt.Errorf("invalid tenant guard mode %q", m)
		}
	}
	g := &tenantGuard{mode: cfg.Mode, legacyMode: cfg.LegacyMode}
	for _, m := range migratedModels() {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(m); err != nil {
			return err
		}
		g.learn(stmt.Schema)
	}
	return db.Use(g)
}

func (g *tenantGuard) Name() string { return "scrm:tenant_guard" }

func (g *tenantGuard) Initialize(db *gorm.DB) error {
	cb := db.Callback()
	if err := cb.Query().Before("gorm:query").Register("scrm:tenant_guard", g.checkRead); err != nil {
		return err
	}
	if err := cb.Row().Before("gorm:row").Register("scrm:tenant_guard", g.checkRead); err != nil {
		return err
	}
	if err := cb.Update().After("gorm:setup_reflect_value").Before("gorm:update").Register("scrm:tenant_guard", g.checkWrite); err != nil {
		return err
	}
	if err := cb.Delete().Before("gorm:delete").Register("scrm:tenant_guard", g.checkWrite); err != nil {
		return err
	}
	return cb.Create().Before("gorm:create").Register("scrm:tenant_guard", g.checkCreate)
}

func (g *tenantGuard) learn(s *schema.Schema) {
	if s == nil || s.Table == "" {
		return
	}
	if f := s.LookUpField("tenant_id"); f != nil && f.DBName == "tenant_id" {
		g.tables.LoadOrStore(s.Table, tenantTable{legacy: legacyTenantTables[s.Table]})
	}
}

var leadingIdent = regexp.MustCompile(`^\s*["` + "`" + `]?(\w+)["` + "`" + `]?`)

// target 返回语句实际访问的表及其租户属性；非租户表返回 ok=false。
func (g *tenantGuard) target(stmt *gorm.Statement) (string, tenantTable, bool) {
	if stmt.Schema != nil {
		g.learn(stmt.Schema)
	}
	table := stmt.Table
	if stmt.TableExpr != nil {
		// Table("crm_customer c") 时 gorm 会把别名写进 stmt.Table，这里取真实表名。
		if m := leadingIdent.FindStringSubmatch(stmt.TableExpr.SQL); m != nil {
			table = m[1]
		} else {
			return "", tenantTable{}, false // 子查询作为 FROM：内部条件无法检查
		}
	}
	if table == "" && stmt.Schema != nil {
		table = stmt.Schema.Table
	}
	v, ok := g.tables.Load(table)
	if !ok {
		return "", tenantTable{}, false
	}
	return table, v.(tenantTable), true
}

func (g *tenantGuard) violate(db *gorm.DB, table string, tt tenantTable, reason string) {
	err := fmt.Errorf("%w: table=%s %s at %s", ErrTenantScopeMissing, table, reason, callerOutsideGorm())
	mode := g.mode
	if tt.legacy {
		mode = g.legacyMode
	}
	// 两种模式都记录：业务层常把查询错误转成“记录不存在”，日志是定位拦截的唯一线索。
	if log.Sugar != nil {
		log.Sugar.Warnw("tenant guard violation", "mode", mode, "err", err.Error())
	}
	if mode == TenantGuardEnforce {
		_ = db.AddError(err)
	}
}

// callerOutsideGorm 定位发起语句的业务代码位置，便于排查违规。
func callerOutsideGorm() string {
	pcs := make([]uintptr, 32)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(3, pcs)])
	for {
		f, more := frames.Next()
		if !strings.Contains(f.File, "gorm.io/") && !strings.HasSuffix(f.File, "app/models/tenant_guard.go") {
			return fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		if !more {
			return "unknown"
		}
	}
}

func (g *tenantGuard) checkRead(db *gorm.DB) {
	if db.Error != nil || isCrossTenant(db.Statement.Context) {
		return
	}
	table, tt, ok := g.target(db.Statement)
	if !ok || hasTenantCondition(db.Statement, tt.legacy) {
		return
	}
	g.violate(db, table, tt, "read without tenant condition")
}

// checkWrite 校验 UPDATE/DELETE。若语句未显式带租户条件，但目标是已加载的单条记录
// （Model(&obj) / Delete(&obj)），则用该记录的 TenantID 追加条件，防止按主键越权改写。
func (g *tenantGuard) checkWrite(db *gorm.DB) {
	if db.Error != nil || isCrossTenant(db.Statement.Context) {
		return
	}
	stmt := db.Statement
	table, tt, ok := g.target(stmt)
	if !ok || hasTenantCondition(stmt, tt.legacy) {
		return
	}
	if tenantID, ok := reflectedTenantID(stmt); ok {
		stmt.AddClause(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Table: clause.CurrentTable, Name: "tenant_id"}, Value: tenantID},
		}})
		return
	}
	g.violate(db, table, tt, "write without tenant condition")
}

func (g *tenantGuard) checkCreate(db *gorm.DB) {
	if db.Error != nil || isCrossTenant(db.Statement.Context) {
		return
	}
	stmt := db.Statement
	table, tt, ok := g.target(stmt)
	if !ok || tt.legacy {
		return // 老表 tenant_id 由数据库触发器按企业映射写入，缺映射时数据库拒绝
	}
	if !allRowsHaveTenant(stmt) {
		g.violate(db, table, tt, "create without tenant_id")
	}
}

// reflectedTenantID 读取单条目标记录的 TenantID；记录须带主键，或语句已有其他 WHERE 条件，
// 避免为无条件更新补上租户条件后扩大为“整租户更新”。
func reflectedTenantID(stmt *gorm.Statement) (string, bool) {
	if stmt.Schema == nil || stmt.ReflectValue.Kind() != reflect.Struct || stmt.ReflectValue.Type() != stmt.Schema.ModelType {
		return "", false
	}
	f := stmt.Schema.LookUpField("tenant_id")
	if f == nil {
		return "", false
	}
	v, zero := f.ValueOf(stmt.Context, stmt.ReflectValue)
	tenantID, _ := v.(string)
	if zero || tenantID == "" {
		return "", false
	}
	if _, hasWhere := stmt.Clauses["WHERE"]; hasWhere {
		return tenantID, true
	}
	for _, pk := range stmt.Schema.PrimaryFields {
		if _, pkZero := pk.ValueOf(stmt.Context, stmt.ReflectValue); !pkZero {
			return tenantID, true
		}
	}
	return "", false
}

func allRowsHaveTenant(stmt *gorm.Statement) bool {
	rv := stmt.ReflectValue
	switch rv.Kind() {
	case reflect.Map:
		return mapHasTenant(rv)
	case reflect.Slice, reflect.Array:
		if rv.Len() == 0 {
			return true
		}
		for i := 0; i < rv.Len(); i++ {
			row := reflect.Indirect(rv.Index(i))
			if row.Kind() == reflect.Map {
				if !mapHasTenant(row) {
					return false
				}
				continue
			}
			if !structHasTenant(stmt, row) {
				return false
			}
		}
		return true
	case reflect.Struct:
		return structHasTenant(stmt, rv)
	}
	return false
}

func structHasTenant(stmt *gorm.Statement, rv reflect.Value) bool {
	if stmt.Schema == nil {
		return false
	}
	f := stmt.Schema.LookUpField("tenant_id")
	if f == nil {
		return false
	}
	v, zero := f.ValueOf(stmt.Context, rv)
	s, _ := v.(string)
	return !zero && s != ""
}

func mapHasTenant(rv reflect.Value) bool {
	for _, key := range []string{"tenant_id", "TenantID"} {
		v := rv.MapIndex(reflect.ValueOf(key))
		if v.IsValid() {
			if s, ok := v.Interface().(string); ok {
				return s != ""
			}
			return !v.IsZero()
		}
	}
	return false
}

var (
	tenantExprPattern = regexp.MustCompile(`(?i)(?:^|[^\w"])(?:"?\w+"?\.)?"?tenant_id"?\s*(=|\bin\b)`)
	corpExprPattern   = regexp.MustCompile(`(?i)(?:^|[^\w"])(?:"?\w+"?\.)?"?ext_corp_id"?\s*(=|\bin\b)`)
	orPattern         = regexp.MustCompile(`(?i)\bor\b`)
)

// hasTenantCondition 仅认可 WHERE 顶层 AND 条件中的非空 tenant_id（老表另认 ext_corp_id），
// 出现在 OR 分支中的条件不构成隔离保证。
func hasTenantCondition(stmt *gorm.Statement, legacy bool) bool {
	c, ok := stmt.Clauses["WHERE"]
	if !ok {
		return false
	}
	where, ok := c.Expression.(clause.Where)
	if !ok {
		return false
	}
	for _, e := range where.Exprs {
		// 顶层出现 Or(...) 时整个 WHERE 变为 "(... AND ...) OR ..."，任何 AND 条件都不再是保证。
		if _, isOr := e.(clause.OrConditions); isOr {
			return false
		}
	}
	columns := []string{"tenant_id"}
	patterns := []*regexp.Regexp{tenantExprPattern}
	if legacy {
		columns = append(columns, "ext_corp_id")
		patterns = append(patterns, corpExprPattern)
	}
	return exprsHaveTenant(where.Exprs, columns, patterns)
}

func exprsHaveTenant(exprs []clause.Expression, columns []string, patterns []*regexp.Regexp) bool {
	for _, e := range exprs {
		if exprHasTenant(e, columns, patterns) {
			return true
		}
	}
	return false
}

func exprHasTenant(e clause.Expression, columns []string, patterns []*regexp.Regexp) bool {
	switch v := e.(type) {
	case clause.Eq:
		return columnIs(v.Column, columns) && nonEmpty(v.Value)
	case clause.IN:
		return columnIs(v.Column, columns)
	case clause.AndConditions:
		return exprsHaveTenant(v.Exprs, columns, patterns)
	case clause.Expr:
		return sqlHasTenant(v.SQL, v.Vars, patterns)
	case clause.NamedExpr:
		return sqlHasTenant(v.SQL, v.Vars, patterns)
	}
	return false
}

func columnIs(col any, names []string) bool {
	var name string
	switch c := col.(type) {
	case string:
		name = c
	case clause.Column:
		name = c.Name
	default:
		return false
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Trim(name, `"`+"`")
	for _, n := range names {
		if name == n {
			return true
		}
	}
	return false
}

// sqlHasTenant 只看括号外（顶层）的文本：顶层出现 OR 则不构成保证；
// 括号/子查询中的 OR 或 tenant_id 不影响判断。
func sqlHasTenant(sql string, vars []any, patterns []*regexp.Regexp) bool {
	top := topLevelSQL(sql)
	if orPattern.MatchString(top) {
		return false
	}
	for _, p := range patterns {
		loc := p.FindStringIndex(top)
		if loc == nil {
			continue
		}
		// 紧随其后的占位符值为空时视为缺失（如 Where("tenant_id = ?", "")）。
		idx := strings.Count(sql[:loc[1]], "?")
		return idx >= len(vars) || nonEmpty(vars[idx])
	}
	return false
}

// topLevelSQL 把括号内与引号字符串内的内容替换为空格（长度不变，便于按位置统计占位符）。
func topLevelSQL(sql string) string {
	out := []byte(sql)
	depth := 0
	var quote byte
	for i := 0; i < len(out); i++ {
		c := out[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				out[i] = ' '
			}
			continue
		case c == '\'':
			quote = c
			continue
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
			out[i] = ' '
			continue
		}
		if depth > 0 {
			out[i] = ' '
		}
	}
	return string(out)
}

func nonEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return x != ""
	case *string:
		return x != nil && *x != ""
	}
	return true
}
