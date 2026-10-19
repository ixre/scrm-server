package dispatch

import (
	"context"
	"encoding/json"
	"strings"

	"core/app/constants"
	"core/app/dto"
	"core/app/models"
	"core/app/services"
	"core/common/storage"
	"core/conf"
)

// 数据导出（data_export）域适配器 —— 通用 JSON 透传分发。
// 导出动作返回签名下载 URL（前端直接 window.open 下载），经 JSON 透传。
func init() {
	s := "data_export"
	Reg.Register(s, "ExportGroupChatList", dataExportGroupChatList)
	Reg.Register(s, "ExportDeleteStaffWarningList", dataExportDeleteStaffWarningList)
	Reg.Register(s, "GenDataExportTask", dataExportGenTask)
	Reg.Register(s, "Get", dataExportGet)
}

// fullPathToSignedURL converts a local absolute file path to a signed download URL.
func fullPathToSignedURL(fullPath string) (string, error) {
	objectKey := strings.TrimPrefix(fullPath, conf.Settings.Storage.LocalRootPath)
	objectKey = strings.TrimPrefix(objectKey, "/")
	return storage.FileStorage.SignURL(objectKey, constants.HTTPGet, 3600)
}

func dataExportGroupChatList(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req dto.QueryGroupChatReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	fullPath, err := services.NewDataExport().ExportGroupChatList(req, corpID)
	if err != nil {
		return nil, err
	}
	return fullPathToSignedURL(fullPath)
}

func dataExportDeleteStaffWarningList(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req dto.DataExportReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	fullPath, err := services.NewDataExport().ExportDeleteStaffWarningList(req, corpID)
	if err != nil {
		return nil, err
	}
	return fullPathToSignedURL(fullPath)
}

func dataExportGenTask(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req dto.DataExportReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDataExport().GenDataExportTask(req, models.Staff{ExtCorpID: corpID, ExtID: staffID})
}

func dataExportGet(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDataExport().Get(req.ID, models.Staff{ExtCorpID: corpID, ExtID: staffID})
}
