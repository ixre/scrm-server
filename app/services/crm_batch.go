package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"
)

// BatchExecReq 批量操作执行入参。
type BatchExecReq struct {
	OpType     string   `json:"op_type"`      // change/seas/lock/unlock/blacklist/remove_blacklist/edit_field
	EntityType string   `json:"entity_type"`  // customer/lead
	EntityIDs  []string `json:"entity_ids"`   // 实体ID列表
	ToUserID   string   `json:"to_user_id"`   // 转移目标用户ID（op_type=change）
	FieldKey   string   `json:"field_key"`    // 字段名（op_type=edit_field）
	FieldValue string   `json:"field_value"`  // 字段值（op_type=edit_field）
}

type CrmBatchService struct{}

func NewCrmBatch() *CrmBatchService { return &CrmBatchService{} }

// Execute 执行批量操作并记录。
func (s CrmBatchService) Execute(ctx context.Context, userID string, req BatchExecReq) (*models.CrmBatchOp, error) {
	tenantID := identity.TenantID(ctx)


	if len(req.EntityIDs) == 0 {
		return nil, errors.New("entity_ids 不能为空")
	}
	if req.EntityType == "" {
		return nil, errors.New("entity_type 不能为空")
	}
	op := strings.TrimSpace(req.OpType)
	if op == "" {
		return nil, errors.New("op_type 不能为空")
	}

	success, fail, remark, err := s.dispatch(ctx, userID, req)

	batch := &models.CrmBatchOp{
		Model:        models.Model{ID: id_generator.StringID()},
		TenantID:     tenantID,
		OpType:       op,
		EntityType:   req.EntityType,
		EntityIDs:    req.EntityIDs,
		Status:       "done",
		SuccessCount: success,
		FailCount:    fail,
		OperatorID:   userID,
		Remark:       remark,
	}
	if err != nil {
		batch.Status = "failed"
	}
	if cerr := models.DB.Create(batch).Error; cerr != nil {
		log.Sugar.Errorw("crm batch op record create failed", "err", cerr)
		return batch, cerr
	}
	return batch, err
}

// dispatch 根据 op_type 调用对应 service 方法，返回 (成功数, 失败数, 备注, error)。
func (s CrmBatchService) dispatch(ctx context.Context, userID string, req BatchExecReq) (int, int, string, error) {


	total := len(req.EntityIDs)
	remark := ""
	switch req.OpType {
	case "change": // 转移
		if req.ToUserID == "" {
			return 0, total, "", errors.New("to_user_id 不能为空")
		}
		n, err := NewCrmTransfer().Transfer(ctx, userID, req.EntityIDs, req.ToUserID, "批量转移")
		remark = "批量转移至 " + req.ToUserID
		return int(n), total - int(n), remark, err
	case "seas": // 入海（公海）
		n, err := NewCrmPool().Recycle(ctx, req.EntityIDs)
		remark = "批量入公海"
		return int(n), total - int(n), remark, err
	case "lock": // 锁客
		err := NewCrmCustomerExt().Lock(ctx, req.EntityIDs, "批量锁客")
		remark = "批量锁客"
		if err != nil {
			return 0, total, remark, err
		}
		return total, 0, remark, nil
	case "unlock": // 解锁
		err := NewCrmCustomerExt().Unlock(ctx, req.EntityIDs)
		remark = "批量解锁"
		if err != nil {
			return 0, total, remark, err
		}
		return total, 0, remark, nil
	case "blacklist": // 拉黑
		err := NewCrmCustomerExt().Blacklist(ctx, req.EntityIDs, "批量拉黑")
		remark = "批量拉黑"
		if err != nil {
			return 0, total, remark, err
		}
		return total, 0, remark, nil
	case "remove_blacklist": // 移出黑名单
		err := NewCrmCustomerExt().RemoveBlacklist(ctx, req.EntityIDs)
		remark = "批量移出黑名单"
		if err != nil {
			return 0, total, remark, err
		}
		return total, 0, remark, nil
	case "edit_field": // 改字段
		if req.FieldKey == "" {
			return 0, total, "", errors.New("field_key 不能为空")
		}
		n, err := batchUpdateField(ctx, req.EntityType, req.EntityIDs, req.FieldKey, req.FieldValue)
		remark = "批量修改字段 " + req.FieldKey
		return int(n), total - int(n), remark, err
	default:
		return 0, total, "", errors.New("不支持的 op_type: " + req.OpType)
	}
}

// batchUpdateField 批量更新实体某字段（仅支持 customer/lead）。
func batchUpdateField(ctx context.Context, entityType string, ids []string, fieldKey, fieldValue string) (int64, error) {
	switch entityType {
	case "customer":
		res := models.MustFor(ctx).Model(&models.CrmCustomer{}).
			Where("id IN ?", ids).
			Update(fieldKey, fieldValue)
		return res.RowsAffected, res.Error
	case "lead":
		res := models.MustFor(ctx).Model(&models.CrmLead{}).
			Where("id IN ?", ids).
			Update(fieldKey, fieldValue)
		return res.RowsAffected, res.Error
	default:
		return 0, errors.New("不支持的 entity_type: " + entityType)
	}
}
