package dto

type UploadTempMaterialReq struct {
	FileType string `json:"file_type" validate:"required"`
	FileURL  string `json:"file_url" validate:"required" `
}
