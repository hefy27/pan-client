package aliyundrive

import "time"

type RespErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AliyunFile struct {
	DriveId       string     `json:"drive_id"`
	FileId        string     `json:"file_id"`
	ParentFileId  string     `json:"parent_file_id"`
	Name          string     `json:"name"`
	Size          int64      `json:"size"`
	FileExtension string     `json:"file_extension"`
	Category      string     `json:"category"`
	Type          string     `json:"type"`
	Thumbnail     string     `json:"thumbnail"`
	Url           string     `json:"url"`
	CreatedAt     *time.Time `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type FilesResp struct {
	Items      []AliyunFile `json:"items"`
	NextMarker string       `json:"next_marker"`
}

type TokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type UploadResp struct {
	FileId       string `json:"file_id"`
	UploadId     string `json:"upload_id"`
	RapidUpload  bool   `json:"rapid_upload"`
	PartInfoList []struct {
		UploadUrl         string `json:"upload_url"`
		InternalUploadUrl string `json:"internal_upload_url"`
	} `json:"part_info_list"`
}
