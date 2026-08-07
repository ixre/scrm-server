package dispatch

import (
	"context"
	"encoding/json"

	"core/app/models"
	"core/app/requests"
	"core/app/services"
)

// 数据导出（data_export）域适配器 —— 通用 JSON 透传分发。
// 导出动作返回文件路径/URL 字符串（前端据此下载），经 JSON 透传；文件流下载不在本泛化范围内。
func init() {
	s := "data_export"
	Reg.Register(s, "ExportGroupChatList", dataExportGroupChatList)
	Reg.Register(s, "ExportDeleteStaffWarningList", dataExportDeleteStaffWarningList)
	Reg.Register(s, "GenDataExportTask", dataExportGenTask)
	Reg.Register(s, "Get", dataExportGet)
}

func dataExportGroupChatList(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.QueryGroupChatReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDataExport().ExportGroupChatList(req, corpID)
}

func dataExportDeleteStaffWarningList(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.DataExportReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDataExport().ExportDeleteStaffWarningList(req, corpID)
}

func dataExportGenTask(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.DataExportReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDataExport().GenDataExportTask(req, models.Staff{ExtCorpID: corpID, ExtID: staffID})
}

func dataExportGet(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDataExport().Get(req.ID, models.Staff{ExtCorpID: corpID, ExtID: staffID})
}
