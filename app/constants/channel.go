package constants

// 渠道维度常量：客户来源 source、营销活动投放渠道 channel、ChannelLink.channel 共用同一套取值。
// 参考同类产品（群脉/数云/有赞）：渠道是一等维度，客户主档/活动均按渠道组织。

const (
	// ChannelWecom 企业微信
	ChannelWecom = "wecom"
	// ChannelXiaohongshu 小红书
	ChannelXiaohongshu = "xiaohongshu"
	// ChannelGzh 微信公众号
	ChannelGzh = "gzh"
	// ChannelDouyin 抖音
	ChannelDouyin = "douyin"
	// ChannelGeneral 通用/其他渠道（预留扩展位）
	ChannelGeneral = "general"
)

// CRM 原生录入/导入来源（非渠道，source 仍为同一列）。
const (
	SourceManual  = "manual"
	SourceImport  = "import"
	SourceWebForm = "web_form"
)

// DictItem 字典项（后台下发，前端展示用）。
type DictItem struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

// AddWayByChannel 渠道内添加/承接方式字典（crm_customer.add_way），由 crm_dict/AddWay 下发。
// wecom 对齐企业微信官方[来源定义]（含 201 内部成员共享/202 管理员分配）；
// xiaohongshu 为自定义承接方式。新增渠道在此追加，前端不再硬编码。
var AddWayByChannel = map[string][]DictItem{
	ChannelWecom: {
		{Value: 0, Label: "未知来源"},
		{Value: 1, Label: "扫描二维码"},
		{Value: 2, Label: "搜索手机号"},
		{Value: 3, Label: "名片分享"},
		{Value: 4, Label: "群聊"},
		{Value: 5, Label: "手机通讯录好友"},
		{Value: 6, Label: "微信联系人"},
		{Value: 7, Label: "微信添加好友申请"},
		{Value: 8, Label: "第三方应用自动添加"},
		{Value: 9, Label: "搜索邮箱"},
		{Value: 10, Label: "日程邀请加入"},
		{Value: 11, Label: "社交名片加入"},
		{Value: 12, Label: "视频号添加"},
		{Value: 13, Label: "直播添加"},
		{Value: 201, Label: "内部成员共享"},
		{Value: 202, Label: "管理员/负责人分配"},
	},
	ChannelXiaohongshu: {
		{Value: 0, Label: "其他"},
		{Value: 1, Label: "留资表单"},
		{Value: 2, Label: "私信承接"},
		{Value: 3, Label: "评论互动"},
		{Value: 4, Label: "人工录入"},
	},
	ChannelGzh: {
		{Value: 0, Label: "其他"},
		{Value: 1, Label: "扫码关注"},
		{Value: 2, Label: "菜单点击"},
		{Value: 3, Label: "文章/图文引导"},
		{Value: 4, Label: "关键词回复"},
		{Value: 5, Label: "模板消息/客服消息"},
		{Value: 6, Label: "人工录入"},
	},
	ChannelDouyin: {
		{Value: 0, Label: "其他"},
		{Value: 1, Label: "主页留资"},
		{Value: 2, Label: "私信承接"},
		{Value: 3, Label: "评论互动"},
		{Value: 4, Label: "直播间新增"},
		{Value: 5, Label: "团购/货架"},
		{Value: 6, Label: "人工录入"},
	},
}

// ChannelLabel 渠道中文名（报表/前端展示用）。
func ChannelLabel(ch string) string {
	switch ch {
	case ChannelWecom:
		return "企业微信"
	case ChannelXiaohongshu:
		return "小红书"
	case ChannelGzh:
		return "公众号"
	case ChannelDouyin:
		return "抖音"
	case ChannelGeneral:
		return "通用"
	default:
		return ch
	}
}
