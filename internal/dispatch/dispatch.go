// Package dispatch 提供通用 JSON 透传的动作分发器。
//
// 背景：openscrm 有数十个业务域、上百个异构方法签名（io.ReadCloser、bytes.Buffer、
// 多字符串参数、creator 参数等），为每个域建模类型化 gRPC proto 不现实。
// 因此这里用 (service, action) 定位闭源 core 内的真实 service 方法，参数/结果走 JSON，
// 依然保证实现全在闭源 core 内，网关只透传。类型化契约（customer/staff）保持不变，逐域叠加。
package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"core/app/services"
	app "core/common/app"
)

// Adapter 处理一个 (service, action) 动作。params 为入参 JSON，返回可 JSON 序列化的结果。
type Adapter func(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error)

// Registry 是 service → action → Adapter 的映射。
type Registry map[string]map[string]Adapter

// Register 注册一个动作。
func (r Registry) Register(service, action string, a Adapter) {
	if r[service] == nil {
		r[service] = map[string]Adapter{}
	}
	if _, exists := r[service][action]; exists {
		panic(fmt.Sprintf("duplicate business action %s/%s", service, action))
	}
	r[service][action] = a
}

// Lookup 查找一个动作。
func (r Registry) Lookup(service, action string) (Adapter, bool) {
	m, ok := r[service]
	if !ok {
		return nil, false
	}
	a, ok := m[action]
	return a, ok
}

// Services 返回已注册的服务名列表（用于网关透传校验/调试）。
func (r Registry) Services() []string {
	out := make([]string, 0, len(r))
	for k := range r {
		out = append(out, k)
	}
	return out
}

// Reg 全局注册表，各业务域适配器在 init() 中注册。
var Reg = Registry{}

// Bind 把入参 JSON 绑定到目标 struct（等价 gin 的 ShouldBindJSON）。
func Bind(params json.RawMessage, v any) error {
	normalized, err := normalizeFormTimes(params, reflect.TypeOf(v))
	if err != nil {
		return err
	}
	return json.Unmarshal(normalized, v)
}

// Result 包装"列表 + 总数"，对应 openscrm 常见的 ([]T, total int64) 返回形状。
func Result(items any, total int64) map[string]any {
	return map[string]any{"items": items, "total": total}
}

// QueryParams 是带分页/排序的列表查询入参包裹。Query 字段随具体请求类型而定。
type QueryParams[T any] struct {
	Query  T           `json:"query"`
	Sorter *app.Sorter `json:"sorter"`
	Pager  *app.Pager  `json:"pager"`
}

// MergePS 把 QueryParams 顶层的 Pager/Sorter 合并到 query 中同名嵌入字段。
// 很多 requests.XXXReq 内嵌了 app.Pager / app.Sorter，而 PQuery 网关层把
// pager/sorter 放在 JSON 顶层而非 query 内部，需要手动回填。
func MergePS[T any](p *QueryParams[T]) {
	if p.Pager == nil && p.Sorter == nil {
		return
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return
	}
	// 重新序列化为扁平结构：把 pager/sorter 字段注入到 query 内部
	var flat map[string]json.RawMessage
	if err := json.Unmarshal(raw, &flat); err != nil {
		return
	}
	queryBytes, ok := flat["query"]
	if !ok {
		return
	}
	var queryMap map[string]json.RawMessage
	if err := json.Unmarshal(queryBytes, &queryMap); err != nil {
		return
	}
	if pagerBytes, ok := flat["pager"]; ok {
		queryMap["pager"] = pagerBytes
	}
	if sorterBytes, ok := flat["sorter"]; ok {
		queryMap["sorter"] = sorterBytes
	}
	merged, _ := json.Marshal(queryMap)
	_ = json.Unmarshal(merged, &p.Query)
}

// corpCtx 从 corpID 反查 channel_link 得到对应 tenant_id，注入 ctx。
// 用于老 openscrm 域（customer/staff/contact_way/group_chat/mass_msg 等）的 dispatch 适配器。
// 找不到时返回原 ctx（保持兼容，未配 channel_link 的 corp 也能跑，但 tenant 隔离失效）。
//
// 实际委托给 services.SyncTenantCtx（同一份逻辑），避免重复实现。
func corpCtx(ctx context.Context, corpID string) context.Context {
	return services.SyncTenantCtx(ctx, corpID)
}
