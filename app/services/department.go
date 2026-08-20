package services

import (
	"context"

	"github.com/pkg/errors"
	"core/app/entities"
	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"core/common/log"
	"core/common/we_work"
)

type Department struct {
	model models.Department
}

// Query
// Description: 查部门列表
func (d Department) Query(ctx context.Context, req entities.GetSubDepartmentReq, extCorpID string) (res []models.Department, total int64, err error) {
	res = make([]models.Department, 0)
	_ = identity.TenantID(ctx) // 2D 后续：legacy 查询补 TenantScope
	res, total, err = d.model.Query(req.ExtParentId, extCorpID, req.ExtIDs, &req.Sorter, &req.Pager)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	return
}

func (d Department) Sync(ctx context.Context, extCorpID string) error {
	wxClient, err := we_work.Clients.Get(extCorpID)
	if err != nil {
		return err
	}
	depts, err := wxClient.Contact.SimpleListAllDepartments()
	if err != nil {
		log.Sugar.Error("get all departments from wx failed", err)
		return err
	}

	// 只维护企业微信的部门关系（企业内部）
	departments := make([]models.Department, 0)

	for _, dept := range depts {
		var department models.Department
		department.ID = id_generator.StringID()
		department.TenantID = identity.TenantID(ctx)
		department.ExtCorpID = extCorpID
		department.ExtID = dept.ID
		department.ExtParentID = dept.ParentID
		department.Order = dept.Order
		departments = append(departments, department)
	}

	err = d.model.Upsert(departments...)
	if err != nil {
		log.Sugar.Error("create department failed", err)
		return err
	}

	return nil
}

func (d Department) Get(ctx context.Context, extDeptID int64, extCorpID string) (models.Department, error) {
	_ = ctx
	return d.model.Get(extDeptID, extCorpID)
}

func NewDepartment() *Department {
	return &Department{model: models.Department{}}
}
