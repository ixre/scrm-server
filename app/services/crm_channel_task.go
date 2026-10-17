package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CrmChannelTaskService struct{}

func NewCrmChannelTask() *CrmChannelTaskService { return &CrmChannelTaskService{} }

func activeChannelTask(status string) bool { return status == "pending" || status == "timeout" }
func channelTaskCustomerAccess(c models.CrmCustomer, uid string) bool {
	if c.DeletedAt.Valid || c.PoolStatus != 0 || c.IsBlacklist || c.ReassignStatus != 0 {
		return false
	}
	if c.OwnerUserID == uid {
		return true
	}
	for _, id := range c.SharedStaffIDs {
		if id == uid {
			return true
		}
	}
	return false
}
func channelTaskLog(tx *gorm.DB, task models.CrmChannelTask, actor, action, oldStatus, oldUser, note string) error {
	log := models.CrmChannelTaskLog{Model: models.Model{ID: id_generator.StringID()}, TenantID: task.TenantID, TaskID: task.ID, ActorID: actor, Action: action, FromStatus: oldStatus, ToStatus: task.Status, FromUserID: oldUser, ToUserID: task.AssignUserID, Note: note}
	return createCRM(tx, &log)
}

// Repeat source delivery does not reopen completed work. Reopen is explicit and audited.
func (s CrmChannelTaskService) CreateLeadToWecomTask(ctx context.Context, customerID string) error {
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return createLeadToWecomTask(ctx, tx, customerID) })
}
func createLeadToWecomTask(ctx context.Context, tx *gorm.DB, customerID string) error {
	tid := identity.TenantID(ctx)
	if tid == "" || customerID == "" {
		return errors.New("租户和客户不能为空")
	}
	var c models.CrmCustomer
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tid, customerID).First(&c).Error; err != nil {
		return err
	}
	if c.IsBlacklist {
		return nil
	}
	// Includes historical/soft-deleted tasks, so retries cannot silently recreate work.
	var existing models.CrmChannelTask
	err := tx.Unscoped().Where("tenant_id = ? AND customer_id = ? AND action = 'add_wecom' AND from_channel = ? AND to_channel = ?", tid, c.ID, constants.ChannelXiaohongshu, constants.ChannelWecom).Order("created_at desc, id desc").First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	due := time.Now().Add(48 * time.Hour)
	task := models.CrmChannelTask{Model: models.Model{ID: id_generator.StringID()}, TenantID: tid, CustomerID: c.ID, FromChannel: constants.ChannelXiaohongshu, ToChannel: constants.ChannelWecom, Action: "add_wecom", Status: "pending", DueAt: &due, Version: 1, DedupKey: "xhs-wecom:" + c.ID}
	if c.OwnerUserID != "" && channelTaskCustomerAccess(c, c.OwnerUserID) {
		var count int64
		if err = tx.Model(&models.User{}).Where("tenant_id = ? AND id = ? AND status = 1", tid, c.OwnerUserID).Count(&count).Error; err != nil {
			return err
		}
		if count == 1 {
			task.AssignUserID = c.OwnerUserID
		}
	}
	if err = createCRM(tx, &task); err != nil {
		return err
	}
	return channelTaskLog(tx, task, "", "created", "", "", "留资生成引流任务")
}

type ChannelTaskItem struct {
	models.CrmChannelTask
	CustomerName     string `json:"customer_name"`
	CustomerMobile   string `json:"customer_mobile"`
	AssigneeName     string `json:"assignee_name"`
	CanAssign        bool   `json:"can_assign"`
	CanClaim         bool   `json:"can_claim"`
	CanResolve       bool   `json:"can_resolve"`
	CanReopen        bool   `json:"can_reopen"`
	CustomerOwner    string `json:"-"`
	CustomerPool     int    `json:"-"`
	CustomerBlocked  bool   `json:"-"`
	CustomerReassign int    `json:"-"`
	CustomerExists   string `json:"-"`
}

func channelTaskReadDB(ctx context.Context, tx *gorm.DB) (*gorm.DB, models.User, bool, error) {
	actor, manager, err := customerActor(ctx, tx)
	if err != nil {
		return nil, actor, false, err
	}
	q := tx.WithContext(ctx).Table("crm_channel_task AS t").Joins("LEFT JOIN crm_customer c ON c.id = t.customer_id AND c.tenant_id = t.tenant_id AND c.deleted_at IS NULL").Where("t.tenant_id = ? AND t.deleted_at IS NULL", actor.TenantID)
	if !manager {
		shared, _ := json.Marshal([]string{actor.ID})
		q = q.Where("c.pool_status = 0 AND c.is_blacklist = false AND c.reassign_status = 0 AND (c.owner_user_id = ? OR (t.assign_user_id = ? AND c.shared_staff_ids @> ?::jsonb))", actor.ID, actor.ID, string(shared))
	}
	return q, actor, manager, nil
}
func (s CrmChannelTaskService) List(ctx context.Context, status string, pager *app.Pager) ([]ChannelTaskItem, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db, actor, manager, err := channelTaskReadDB(ctx, models.DB)
	if err != nil {
		return nil, 0, err
	}
	switch status {
	case "":
	case "pending", "timeout", "done", "skipped":
		db = db.Where("t.status = ?", status)
	default:
		return nil, 0, errors.New("任务状态无效")
	}
	var total int64
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ChannelTaskItem
	err = db.Joins(`LEFT JOIN "user" u ON u.id = t.assign_user_id AND u.tenant_id = t.tenant_id AND u.deleted_at IS NULL`).
		Select("t.*, c.name AS customer_name, c.mobile AS customer_mobile, c.id AS customer_exists, c.owner_user_id AS customer_owner, c.pool_status AS customer_pool, c.is_blacklist AS customer_blocked, c.reassign_status AS customer_reassign, u.name AS assignee_name").
		Order("t.created_at desc, t.id desc").Offset(pager.GetOffset()).Limit(pager.GetLimit()).Scan(&rows).Error
	for i := range rows {
		r := &rows[i]
		active := activeChannelTask(r.Status)
		valid := r.CustomerExists != "" && r.CustomerPool == 0 && !r.CustomerBlocked && r.CustomerReassign == 0
		r.CanAssign = manager && active && valid
		r.CanClaim = active && valid && r.CustomerOwner == actor.ID && r.AssignUserID != actor.ID
		r.CanResolve = active && (manager || r.AssignUserID == actor.ID) && valid
		r.CanReopen = manager && !active && valid && (r.Status == "done" || r.Status == "skipped")
	}
	return rows, total, err
}

// Customer before task matches customer merge/transfer lock order.
func mutateChannelTask(ctx context.Context, id string, fn func(*gorm.DB, models.User, bool, models.CrmCustomer, *models.CrmChannelTask) error) error {
	if id == "" {
		return errors.New("任务ID不能为空")
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, manager, err := customerActor(ctx, tx)
		if err != nil {
			return err
		}
		var task models.CrmChannelTask
		if err = tx.Where("tenant_id = ? AND id = ?", actor.TenantID, id).First(&task).Error; err != nil {
			return errors.New("任务不存在或无权操作")
		}
		var c models.CrmCustomer
		if err = tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("tenant_id = ? AND id = ?", actor.TenantID, task.CustomerID).First(&c).Error; err != nil {
			return errors.New("关联客户已删除或不存在")
		}
		customerID := task.CustomerID
		if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", actor.TenantID, id).First(&task).Error; err != nil {
			return err
		}
		if task.CustomerID != customerID {
			return errors.New("客户已合并，请刷新重试")
		}
		if !manager && !channelTaskCustomerAccess(c, actor.ID) {
			return ErrCustomerAccess
		}
		return fn(tx, actor, manager, c, &task)
	})
}
func taskVersion(task models.CrmChannelTask, version int64) error {
	if version < 1 || task.Version != version {
		return errors.New("任务已变化，请刷新后重试")
	}
	return nil
}
func taskNote(note string) (string, error) {
	note = strings.TrimSpace(note)
	if note == "" || utf8.RuneCountInString(note) > 512 {
		return "", errors.New("请填写1至512字的处理结果或原因")
	}
	return note, nil
}
func (s CrmChannelTaskService) Resolve(ctx context.Context, id, status, note string, version int64) error {
	if status != "done" && status != "skipped" {
		return errors.New("处理状态无效")
	}
	note, err := taskNote(note)
	if err != nil {
		return err
	}
	return mutateChannelTask(ctx, id, func(tx *gorm.DB, actor models.User, manager bool, c models.CrmCustomer, t *models.CrmChannelTask) error {
		if !manager && t.AssignUserID != actor.ID {
			return errors.New("请先领取任务")
		}
		if c.PoolStatus != 0 || c.IsBlacklist || c.ReassignStatus != 0 {
			return errors.New("请先完成客户领取或交接，并确认客户不在黑名单")
		}
		if t.Status == status && t.HandledBy == actor.ID && t.Resolution == note {
			return nil
		}
		if err := taskVersion(*t, version); err != nil {
			return err
		}
		if !activeChannelTask(t.Status) {
			return errors.New("任务已结束，请由经理重新打开后处理")
		}
		old := t.Status
		now := time.Now()
		updates := map[string]any{"status": status, "resolution": note, "handled_by": actor.ID, "handled_at": now, "version": t.Version + 1}
		if err := tx.Model(t).Updates(updates).Error; err != nil {
			return err
		}
		return channelTaskLog(tx, *t, actor.ID, status, old, t.AssignUserID, note)
	})
}
func (s CrmChannelTaskService) Assign(ctx context.Context, id, userID string, version int64) error {
	return mutateChannelTask(ctx, id, func(tx *gorm.DB, actor models.User, manager bool, c models.CrmCustomer, t *models.CrmChannelTask) error {
		if !manager {
			return errors.New("仅管理员或经理可以分配任务")
		}
		return assignChannelTask(tx, actor, c, t, userID, version, "assigned")
	})
}
func (s CrmChannelTaskService) Claim(ctx context.Context, id string, version int64) error {
	return mutateChannelTask(ctx, id, func(tx *gorm.DB, actor models.User, _ bool, c models.CrmCustomer, t *models.CrmChannelTask) error {
		if c.OwnerUserID != actor.ID {
			return errors.New("仅客户负责人可以领取，请联系经理分配")
		}
		return assignChannelTask(tx, actor, c, t, actor.ID, version, "claimed")
	})
}
func assignChannelTask(tx *gorm.DB, actor models.User, c models.CrmCustomer, t *models.CrmChannelTask, userID string, version int64, action string) error {
	if err := taskVersion(*t, version); err != nil {
		return err
	}
	if !activeChannelTask(t.Status) {
		return errors.New("只能分配未结束的任务")
	}
	if !channelTaskCustomerAccess(c, userID) {
		return errors.New("接收人须为客户负责人或共享成员，请先领取、交接或共享客户")
	}
	var target models.User
	if userID == "" {
		return errors.New("请选择接收人")
	}
	if err := tx.Where("tenant_id = ? AND id = ? AND status = 1", actor.TenantID, userID).First(&target).Error; err != nil {
		return errors.New("接收人不存在、已停用或不属于本租户")
	}
	if t.AssignUserID == userID {
		return nil
	}
	oldUser := t.AssignUserID
	if err := tx.Model(t).Updates(map[string]any{"assign_user_id": userID, "version": t.Version + 1}).Error; err != nil {
		return err
	}
	return channelTaskLog(tx, *t, actor.ID, action, t.Status, oldUser, "分配不变更客户归属")
}
func (s CrmChannelTaskService) Reopen(ctx context.Context, id, note string, version int64) error {
	note, err := taskNote(note)
	if err != nil {
		return err
	}
	return mutateChannelTask(ctx, id, func(tx *gorm.DB, actor models.User, manager bool, c models.CrmCustomer, t *models.CrmChannelTask) error {
		if !manager {
			return errors.New("仅管理员或经理可以重新打开任务")
		}
		if err := taskVersion(*t, version); err != nil {
			return err
		}
		if t.Status != "done" && t.Status != "skipped" {
			return errors.New("仅已结束任务可重新打开")
		}
		if c.PoolStatus != 0 || c.IsBlacklist || c.ReassignStatus != 0 {
			return errors.New("客户当前不可跟进")
		}
		old := t.Status
		due := time.Now().Add(48 * time.Hour)
		if err := tx.Model(t).Updates(map[string]any{"status": "pending", "due_at": due, "resolution": "", "handled_by": nil, "handled_at": nil, "version": t.Version + 1}).Error; err != nil {
			return err
		}
		return channelTaskLog(tx, *t, actor.ID, "reopened", old, t.AssignUserID, note)
	})
}
func (s CrmChannelTaskService) Logs(ctx context.Context, id string, pager *app.Pager) ([]models.CrmChannelTaskLog, int64, error) {
	db, _, _, err := channelTaskReadDB(ctx, models.DB)
	if err != nil {
		return nil, 0, err
	}
	var n int64
	if err = db.Where("t.id = ?", id).Count(&n).Error; err != nil {
		return nil, 0, err
	}
	if n != 1 {
		return nil, 0, ErrCustomerAccess
	}
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	q := models.DB.WithContext(ctx).Model(&models.CrmChannelTaskLog{}).Where("tenant_id = ? AND task_id = ?", identity.TenantID(ctx), id)
	if err = q.Count(&n).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.CrmChannelTaskLog
	err = q.Order("created_at desc, id desc").Offset(pager.GetOffset()).Limit(pager.GetLimit()).Find(&rows).Error
	return rows, n, err
}

// Overdue is actionable. Skip locked rows, process bounded batches, and audit atomically.
func (s CrmChannelTaskService) Expire(ctx context.Context) error {
	for {
		count := 0
		err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var rows []models.CrmChannelTask
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status = 'pending' AND due_at < ?", time.Now()).Order("due_at, id").Limit(200).Find(&rows).Error; err != nil {
				return err
			}
			count = len(rows)
			for _, t := range rows {
				if err := tx.Model(&t).Updates(map[string]any{"status": "timeout", "version": t.Version + 1}).Error; err != nil {
					return err
				}
				if err := channelTaskLog(tx, t, "", "timeout", "pending", t.AssignUserID, "超过处理截止时间，仍可继续跟进"); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("channel task timeout: %w", err)
		}
		if count == 0 {
			return nil
		}
	}
}

// Assignees searches only the customer's current owner/shared members, not every employee.
func (s CrmChannelTaskService) Assignees(ctx context.Context, id, keyword string) ([]models.User, error) {
	actor, manager, err := customerActor(ctx, models.DB)
	if err != nil {
		return nil, err
	}
	if !manager {
		return nil, errors.New("仅管理员或经理可以分配任务")
	}
	var task models.CrmChannelTask
	if err = models.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", actor.TenantID, id).First(&task).Error; err != nil {
		return nil, err
	}
	var c models.CrmCustomer
	if err = models.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", actor.TenantID, task.CustomerID).First(&c).Error; err != nil {
		return nil, err
	}
	if c.PoolStatus != 0 || c.IsBlacklist || c.ReassignStatus != 0 {
		return nil, errors.New("请先完成客户领取或交接")
	}
	ids := append([]string{}, c.SharedStaffIDs...)
	if c.OwnerUserID != "" {
		ids = append(ids, c.OwnerUserID)
	}
	var users []models.User
	if len(ids) == 0 {
		return users, nil
	}
	q := models.DB.WithContext(ctx).Model(&models.User{}).Select("id, name, username, status").Where("tenant_id = ? AND id IN ? AND status = 1", actor.TenantID, ids)
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		q = q.Where("(name ILIKE ? OR username ILIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}
	err = q.Order("name, id").Limit(100).Find(&users).Error
	return users, err
}
