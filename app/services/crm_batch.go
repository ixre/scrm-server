package services

import (
	"core/common/identity"

	"context"

	"errors"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"core/app/models"
	"core/common/id_generator"
)

// BatchExecReq 批量操作执行入参。
type BatchExecReq struct {
	OpType     string   `json:"op_type"`     // change/seas/lock/unlock/blacklist/remove_blacklist/edit_field
	EntityType string   `json:"entity_type"` // customer/lead
	EntityIDs  []string `json:"entity_ids"`  // 实体ID列表
	ToUserID   string   `json:"to_user_id"`  // 转移目标用户ID（op_type=change）
	FieldKey   string   `json:"field_key"`   // 字段名（op_type=edit_field）
	FieldValue string   `json:"field_value"` // 字段值（op_type=edit_field）
	Reason     string   `json:"reason"`
}

type CrmBatchService struct{}

func NewCrmBatch() *CrmBatchService { return &CrmBatchService{} }

// Execute 执行批量操作并记录。
func (s CrmBatchService) Execute(ctx context.Context, userID string, req BatchExecReq) (*models.CrmBatchOp, error) {
	if userID == "" || userID != identity.UserID(ctx) {
		return nil, ErrCustomerAccess
	}
	if _, _, err := customerActor(ctx, models.DB); err != nil {
		return nil, err
	}
	ids, err := workflowIDs(req.EntityIDs)
	if err != nil {
		return nil, err
	}
	req.EntityIDs = ids
	if len(ids) > 500 {
		return nil, errors.New("单次批量操作最多 500 条")
	}
	if req.EntityType != "customer" && req.EntityType != "lead" {
		return nil, errors.New("仅支持客户或线索批量操作")
	}
	req.OpType = strings.TrimSpace(req.OpType)
	action := req.OpType
	if action == "change" {
		action = "transfer"
	}
	if action == "seas" {
		action = "recycle"
	}
	if action == "edit_field" {
		action = "update"
	}
	if err := requireEntityOperation(ctx, models.DB, req.EntityType, action, "write"); err != nil {
		return nil, err
	}
	batch := &models.CrmBatchOp{Model: models.Model{ID: id_generator.StringID()}, TenantID: identity.TenantID(ctx), OpType: req.OpType, EntityType: req.EntityType, EntityIDs: ids, Status: "pending", OperatorID: userID}
	// Record intent before mutation, so a logging failure never performs an untracked operation.
	if err = createCRM(models.DB.WithContext(ctx), batch); err != nil {
		return nil, err
	}
	success, fail, remark, operationErr := s.dispatch(ctx, userID, req)
	batch.SuccessCount, batch.FailCount, batch.Remark, batch.Status = success, fail, remark, "done"
	if operationErr != nil {
		batch.Status = "failed"
		batch.Remark = operationErr.Error()
	}
	if err = models.DB.WithContext(ctx).Model(batch).Updates(map[string]any{"status": batch.Status, "success_count": success, "fail_count": fail, "remark": batch.Remark}).Error; err != nil {
		return batch, errors.New("操作执行记录更新失败，请先核对业务结果，勿重复提交")
	}
	return batch, operationErr
}

// dispatch 根据 op_type 调用对应 service 方法，返回 (成功数, 失败数, 备注, error)。
func (s CrmBatchService) dispatch(ctx context.Context, userID string, req BatchExecReq) (int, int, string, error) {

	total := len(req.EntityIDs)
	remark := ""
	if req.EntityType == "lead" {
		var n int64
		var err error
		switch req.OpType {
		case "change":
			n, err = NewCrmLeadExt().BatchTransfer(ctx, req.EntityIDs, req.ToUserID)
		case "seas":
			err = NewCrmLeadExt().MoveToPool(ctx, req.EntityIDs)
			if err == nil {
				n = int64(total)
			}
		case "edit_field":
			n, err = batchUpdateField(ctx, "lead", req.EntityIDs, req.FieldKey, req.FieldValue)
		default:
			err = errors.New("线索仅支持转移、入池和修改字段")
		}
		return int(n), total - int(n), "批量线索操作", err
	}
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
		if strings.TrimSpace(req.Reason) == "" {
			return 0, total, "", errors.New("请填写拉黑原因")
		}
		err := NewCrmCustomerExt().Blacklist(ctx, req.EntityIDs, req.Reason)
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
// 字段白名单：排除状态/关联/转化等关键字段，防止批量操作破坏状态一致性。
func batchUpdateField(ctx context.Context, entityType string, ids []string, fieldKey, fieldValue string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	// Only editable descriptive fields; individual identity and business-state fields use their dedicated forms.
	allowed := map[string]bool{"company": true, "source": true, "remark": true, "position": true, "address": true}
	if entityType == "customer" {
		allowed["industry"] = true
		allowed["level"] = true
	}
	if !allowed[fieldKey] {
		return 0, errors.New("不支持批量修改字段: " + fieldKey)
	}
	var value any = strings.TrimSpace(fieldValue)
	if fieldKey == "level" {
		n, err := strconv.Atoi(fieldValue)
		if err != nil || n < 0 || n > 5 {
			return 0, errors.New("客户等级须为 0 到 5 的整数")
		}
		value = n
	}
	if entityType == "lead" {
		return mutateLeads(ctx, ids, func(tx *gorm.DB, lead *models.CrmLead) error {
			if err := mutableLead(*lead); err != nil {
				return err
			}
			return tx.Model(lead).Update(fieldKey, value).Error
		})
	}
	if entityType != "customer" {
		return 0, errors.New("实体类型无效")
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		db, err := customerAccessDB(ctx, tx, true)
		if err != nil {
			return err
		}
		var rows []models.CrmCustomer
		if err = db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id").Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return ErrCustomerAccess
		}
		for _, row := range rows {
			if row.InPool() || row.IsReassigning() {
				return errors.New("请先领取或接收客户再编辑")
			}
		}
		return tx.Model(&models.CrmCustomer{}).Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Update(fieldKey, value).Error
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}
