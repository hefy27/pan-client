package aliyundrive_open

import "time"

type ErrResp struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AliyunFile struct {
	DriveId       string    `json:"drive_id"`
	FileId        string    `json:"file_id"`
	ParentFileId  string    `json:"parent_file_id"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	FileExtension string    `json:"file_extension"`
	ContentHash   string    `json:"content_hash"`
	Category      string    `json:"category"`
	Type          string    `json:"type"`
	Thumbnail     string    `json:"thumbnail"`
	Url           string    `json:"url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	FileName      string    `json:"file_name"`
}

type FilesResp struct {
	Items      []AliyunFile `json:"items"`
	NextMarker string       `json:"next_marker"`
}

type PartInfo struct {
	PartNumber  int    `json:"part_number"`
	PartSize    int64  `json:"part_size"`
	UploadUrl   string `json:"upload_url"`
	ContentType string `json:"content_type"`
}

type CreateResp struct {
	FileId       string     `json:"file_id"`
	UploadId     string     `json:"upload_id"`
	RapidUpload  bool       `json:"rapid_upload"`
	PartInfoList []PartInfo `json:"part_info_list"`
}

type SpaceInfoResp struct {
	PersonalSpaceInfo struct {
		TotalSize int64 `json:"total_size"`
		UsedSize  int64 `json:"used_size"`
	} `json:"personal_space_info"`
}

type DriveInfoResp struct {
	UserId           string `json:"user_id"`
	DefaultDriveId   string `json:"default_drive_id"`
	ResourceDriveId  string `json:"resource_drive_id"`
	BackupDriveId    string `json:"backup_drive_id"`
}
