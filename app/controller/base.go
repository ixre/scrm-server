package controller

import (
	"github.com/pkg/errors"
	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/ecode"
)

type Base struct {
}

// GetConsoleInfo 从session中获取企业普通管理员信息
func (o *Base) GetConsoleInfo(handler *app.Handler) (consoleStaff models.Staff, err error) {
	err = handler.GetSessionVal(constants.ConsoleSessionName, constants.StaffInfo, &consoleStaff)
	if err != nil {
		err = errors.Wrap(err, "get session failed")
		return
	}

	if consoleStaff.ID == "" {
		err = ecode.InvalidSessionError
		return
	}

	item, err := (&models.Staff{}).Get(consoleStaff.ExtID, "", false)
	if err != nil {
		err = errors.Wrap(err, "CachedGet failed")
		return
	}

	consoleStaff = *item

	return
}

// GetStaffInfo 从session中获取企业成员信息
func (o *Base) GetStaffInfo(handler *app.Handler) (consoleStaff models.Staff, err error) {
	err = handler.GetSessionVal(constants.StaffSessionName, constants.StaffInfo, &consoleStaff)
	if err != nil {
		err = errors.Wrap(err, "get session failed")
		return
	}

	if consoleStaff.ID == "" {
		err = ecode.InvalidSessionError
		return
	}

	item, err := (&models.Staff{}).Get(consoleStaff.ExtID, "", false)
	if err != nil {
		err = errors.Wrap(err, "CachedGet failed")
		return
	}

	consoleStaff = *item

	return
}
