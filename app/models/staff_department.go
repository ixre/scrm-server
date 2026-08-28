package models

import (
	"gorm.io/gorm/clause"
	"core/app/constants"
)

type StaffDepartment struct {
	ExtCorpID       string            `json:"ext_corp_id" gorm:"index;type:char(18);uniqueIndex:idx_staff_department_corp_staff_dept"`
	ExtStaffID      string            `json:"ext_staff_id" gorm:"type:varchar(64);index;uniqueIndex:idx_staff_department_corp_staff_dept"`
	ExtDepartmentID int64             `json:"ext_department_id" gorm:"type:bigint;uniqueIndex:idx_staff_department_corp_staff_dept"`
	StaffID         string            `json:"staff_id" gorm:"primaryKey;type:bigint" `
	DepartmentID    string            `json:"department_id" gorm:"primaryKey;type:bigint;" `
	IsLeader        constants.Boolean `json:"is_leader" gorm:"type:smallint;comment:是否是所在部门的领导"`
	Order           uint32            `json:"order" gorm:"type:bigint;comment:所在部门的排序"`
}

func (s StaffDepartment) Upsert(sd ...StaffDepartment) error {
	if len(sd) == 0 { return nil }
 err := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "ext_corp_id"}, {Name: "ext_staff_id"}, {Name: "ext_department_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"order", "is_leader", "staff_id", "department_id"}),
	}).CreateInBatches(&sd, len(sd)).Error
	if err != nil {
		return err
	}

	return err
}

func (s StaffDepartment) Delete(sd ...StaffDepartment) error {
	for _, staffDepartment := range sd {
		err := DB.Model(&StaffDepartment{}).
			Where("ext_corp_id = ? and ext_staff_id = ? and ext_department_id = ?",
				staffDepartment.ExtCorpID, staffDepartment.ExtStaffID, staffDepartment.ExtDepartmentID).Delete(&StaffDepartment{}).Error
		if err != nil {
			return err
		}
	}
	return nil
}
