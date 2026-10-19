package services

import (
	"context"
	"core/app/dto"
	"core/app/models"
	"core/common/id_generator"
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
)

type Material struct {
	materialRepo    models.Material
	materialTagRepo models.MaterialLibTag
	corpSettingRepo models.CorpSetting
}

func NewMaterial() *Material {
	return &Material{
		materialRepo:    models.Material{},
		materialTagRepo: models.MaterialLibTag{},
	}
}

func (m Material) Create(ctx context.Context, req dto.UploadMaterialReq, extCorpID string, staffID string) (material models.Material, err error) {
	material = models.Material{
		ExtCorpModel: models.ExtCorpModel{ID: id_generator.StringID(), ExtCorpID: extCorpID, ExtCreatorID: staffID},
	}
	err = copier.CopyWithOption(&material, req, copier.Option{IgnoreEmpty: true, DeepCopy: true})
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	err = m.materialRepo.Create(material)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	return
}

func (m Material) Update(ctx context.Context, id string, req dto.UpdateMaterialReq, extCorpID string) error {
	material := models.Material{ExtCorpModel: models.ExtCorpModel{ID: id, ExtCorpID: extCorpID}}
	err := copier.CopyWithOption(&material, req, copier.Option{IgnoreEmpty: true, DeepCopy: true})
	if err != nil {
		err = errors.WithStack(err)
		return err
	}
	return m.materialRepo.Update(material)
}

func (m Material) Delete(ctx context.Context, ids []string, extCorpID string) (int64, error) {
	return m.materialRepo.Delete(ids, extCorpID)
}

func (m Material) Query(ctx context.Context, req dto.QueryMaterialReq, extCorpID string) (res []models.MaterialWithTags, total int64, err error) {
	param := models.Material{}
	err = copier.Copy(&param, req)
	if err != nil {
		err = errors.Wrap(err, "copier.Copy failed")
		return nil, 0, err
	}
	var material []models.Material
	material, total, err = m.materialRepo.Query(req, extCorpID)
	if err != nil {
		err = errors.WithStack(err)
		return
	}

	res = make([]models.MaterialWithTags, 0)
	err = copier.Copy(&res, material)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	for i, _ := range material {
		if len(res[i].MaterialTagList) > 0 {
			res[i].Tags, err = m.materialTagRepo.GetByIDs(res[i].MaterialTagList.ToStringArray())
			if err != nil {
				err = errors.WithStack(err)
				return
			}
		} else {
			res[i].Tags = []models.MaterialLibTag{}
		}
	}
	return
}

// GetSidebarStatus
// Description: 素材库侧边栏使用状态
func (m Material) GetSidebarStatus(ctx context.Context, extCorpID string) (dto.GetSidebarStatusResp, error) {

	var resp dto.GetSidebarStatusResp
	corpSetting, err := m.corpSettingRepo.Get(extCorpID)
	if err != nil {
		err = errors.WithStack(err)
		return resp, err
	}
	resp.Status = corpSetting.IsMaterialUsed
	return resp, nil
}

func (m Material) UpdateGetSidebarStatus(ctx context.Context, req dto.UpdateGetSidebarStatusReq, extCorpID string) error {
	return m.corpSettingRepo.Update(models.CorpSetting{
		ID:             id_generator.StringID(),
		ExtCorpID:      extCorpID,
		IsMaterialUsed: req.Status,
	})
}
