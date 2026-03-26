package quark

import (
	"io"
)

// Resp 基础序列化器
type Resp struct {
	Status    int      `json:"status"`
	Code      int      `json:"code"`
	Msg       string   `json:"message"`
	Timestamp int      `json:"timestamp"`
	Metadata  SortMeta `json:"metadata"`
}

// RespData 基础序列化器
type RespData[T any] struct {
	Status    int    `json:"status"`
	Code      int    `json:"code"`
	Msg       string `json:"message"`
	Timestamp int    `json:"timestamp"`
	Data      T      `json:"data,omitempty"`
}

// RespData 基础序列化器
type RespDataWithMeta[T any, M any] struct {
	Status    int    `json:"status"`
	Code      int    `json:"code"`
	Msg       string `json:"message"`
	Timestamp int    `json:"timestamp"`
	Data      T      `json:"data,omitempty"`
	Metadata  M      `json:"metadata,omitempty"`
}

type SortMeta struct {
	Size  int    `json:"_size"`
	ReqId string `json:"req_id"`
	Page  int    `json:"_page"`
	Count int    `json:"_count"`
	Total int    `json:"_total"`
}

type Config struct {
	Md5SizeLimit       int64  `json:"md5_size_limit"`
	ShareEnable        int    `json:"share_enable"`
	Sha1SizeLimit      int64  `json:"sha1_size_limit"`
	ShareSafeHost      string `json:"share_safe_host"`
	AllowCcpHashUpdate bool   `json:"allow_ccp_hash_update"`
}
type MemberMeta struct {
	RangeSize     int   `json:"range_size"`
	ServerCurTime int64 `json:"server_cur_time"`
}

type MemberData struct {
	MemberType      string `json:"member_type"`
	ExpSvipExpAt    int64  `json:"exp_svip_exp_at"`
	ImageBackup     int    `json:"image_backup"`
	DeepRecycleStat struct {
		RecycleNormalServeDays int `json:"recycle_normal_serve_days"`
		RecycleSvipServeDays   int `json:"recycle_svip_serve_days"`
		RecycleZvipServeDays   int `json:"recycle_zvip_serve_days"`
		RecycleVipServeDays    int `json:"recycle_vip_serve_days"`
		RecyclePayServeDays    int `json:"recycle_pay_serve_days"`
		DeepRecycleServeDays   int `json:"deep_recycle_serve_days"`
	} `json:"deep_recycle_stat"`
	CreatedAt  int64 `json:"created_at"`
	MemberInfo struct {
		VideoSaveToUses        int `json:"video_save_to_uses"`
		VideoSaveToRemains     int `json:"video_save_to_remains"`
		FileSaveToRemains      int `json:"file_save_to_remains"`
		OfflineDownloadRemains int `json:"offline_download_remains"`
		MemberTypeMap          struct {
			MINIVIP struct {
				VideoSaveToTotal int `json:"video_save_to_total"`
			} `json:"MINI_VIP"`
		} `json:"member_type_map"`
	} `json:"member_info"`
	AccStatus                 int   `json:"acc_status"`
	SecretUseCapacity         int64 `json:"secret_use_capacity"`
	UseCapacity               int64 `json:"use_capacity"`
	VideoBackup               int   `json:"video_backup"`
	ExtendCapacityComposition struct {
	} `json:"extend_capacity_composition"`
	IsNewUser    bool `json:"is_new_user"`
	MemberStatus struct {
		MINIVIP  string `json:"MINI_VIP"`
		SUPERVIP string `json:"SUPER_VIP"`
		VIP      string `json:"VIP"`
		ZVIP     string `json:"Z_VIP"`
	} `json:"member_status"`
	SecretTotalCapacity    int64 `json:"secret_total_capacity"`
	SubscribePayChannelMap struct {
	} `json:"subscribe_pay_channel_map"`
	ExpAt              int64 `json:"exp_at"`
	SubscribeStatusMap struct {
	} `json:"subscribe_status_map"`
	TotalCapacity int64 `json:"total_capacity"`
	VipExpAt      int64 `json:"vip_exp_at"`
}

type FileList struct {
	List []File
}

type File struct {
	Fid                 string  `json:"fid"`
	FileName            string  `json:"file_name"`
	PdirFid             string  `json:"pdir_fid"`
	Category            int     `json:"category"`
	FileType            int     `json:"file_type"`
	Size                int     `json:"size"`
	FormatType          string  `json:"format_type"`
	Status              int     `json:"status"`
	Tags                string  `json:"tags,omitempty"`
	OwnerUcid           string  `json:"owner_ucid"`
	LCreatedAt          int64   `json:"l_created_at,omitempty"`
	LUpdatedAt          int64   `json:"l_updated_at,omitempty"`
	Source              string  `json:"source"`
	FileSource          string  `json:"file_source"`
	NameSpace           int     `json:"name_space"`
	LShotAt             int64   `json:"l_shot_at"`
	SourceDisplay       string  `json:"source_display"`
	IncludeItems        int     `json:"include_items,omitempty"`
	SeriesDir           bool    `json:"series_dir"`
	UploadCameraRootDir bool    `json:"upload_camera_root_dir"`
	Fps                 float64 `json:"fps"`
	Like                int     `json:"like"`
	OperatedAt          int64   `json:"operated_at"`
	RiskType            int     `json:"risk_type"`
	BackupSign          int     `json:"backup_sign"`
	FileNameHlStart     int     `json:"file_name_hl_start"`
	FileNameHlEnd       int     `json:"file_name_hl_end"`
	Duration            int     `json:"duration"`
	EventExtra          struct {
		IsOpen          bool  `json:"is_open,omitempty"`
		RecentCreatedAt int64 `json:"recent_created_at,omitempty"`
		ViewAt          int64 `json:"view_at,omitempty"`
	} `json:"event_extra"`
	ScrapeStatus            int   `json:"scrape_status"`
	UpdateViewAt            int64 `json:"update_view_at"`
	Ban                     bool  `json:"ban"`
	CurVersionOrDefault     int   `json:"cur_version_or_default"`
	RawNameSpace            int   `json:"raw_name_space"`
	SaveAsSource            bool  `json:"save_as_source"`
	OfflineSource           bool  `json:"offline_source"`
	BackupSource            bool  `json:"backup_source"`
	OwnerDriveTypeOrDefault int   `json:"owner_drive_type_or_default"`
	Dir                     bool  `json:"dir"`
	File                    bool  `json:"file"`
	CreatedAt               int64 `json:"created_at"`
	UpdatedAt               int64 `json:"updated_at"`
	Extra                   struct {
	} `json:"_extra"`
	FileStruct struct {
		FirSource      string `json:"fir_source"`
		SecSource      string `json:"sec_source"`
		ThiSource      string `json:"thi_source"`
		PlatformSource string `json:"platform_source"`
		UploadMi       string `json:"upload_mi"`
		UploadDm       string `json:"upload_dm"`
	} `json:"file_struct,omitempty"`
	Thumbnail    string `json:"thumbnail,omitempty"`
	BigThumbnail string `json:"big_thumbnail,omitempty"`
	PreviewUrl   string `json:"preview_url,omitempty"`
	ObjCategory  string `json:"obj_category,omitempty"`
	LastPlayInfo struct {
		Time int `json:"time"`
	} `json:"last_play_info,omitempty"`
	PdfInfo struct {
		EncryptType      int  `json:"encrypt_type"`
		OpenEncrypted    bool `json:"open_encrypted"`
		ClassifierResult struct {
			Type              int     `json:"type"`
			ImgAreaPercentage float64 `json:"img_area_percentage"`
			AvgImgNumber      float64 `json:"avg_img_number"`
			AvgTextLength     float64 `json:"avg_text_length"`
			AvgPageArea       int     `json:"avg_page_area"`
			AvgPageHeight     int     `json:"avg_page_height"`
			AvgPageWidth      int     `json:"avg_page_width"`
			PagesNumber       int     `json:"pages_number"`
		} `json:"classifier_result"`
	} `json:"pdf_info,omitempty"`
	ShareFidToken string `json:"share_fid_token"`
}
type Dir struct {
	Finish bool   `json:"finish"`
	Fid    string `json:"fid"`
}

type TaskDoing struct {
	TaskId string `json:"task_id"`
	Finish bool   `json:"finish"`
}

type Task struct {
	TaskId          string `json:"task_id"`
	TaskType        int    `json:"task_type"`
	TaskTitle       string `json:"task_title"`
	Status          int    `json:"status"`
	CreatedAt       int64  `json:"created_at"`
	AffectedFileNum int    `json:"affected_file_num"`
	ShareId         string `json:"share_id"`
}

type TaskMeta struct {
	TqGap int `json:"tq_gap"`
}

type FileUpPreMeta struct {
	PartThread int    `json:"part_thread"`
	Acc2       string `json:"acc2"`
	Acc1       string `json:"acc1"`
	PartSize   int    `json:"part_size"` // 分片大小
}

type FileUpCallback struct {
	CallbackUrl  string `json:"callbackUrl"`
	CallbackBody string `json:"callbackBody"`
}

type FileUpPre struct {
	TaskId     string         `json:"task_id"`
	Finish     bool           `json:"finish"`
	UploadId   string         `json:"upload_id"`
	ObjKey     string         `json:"obj_key"`
	UploadUrl  string         `json:"upload_url"`
	Fid        string         `json:"fid"`
	Bucket     string         `json:"bucket"`
	Callback   FileUpCallback `json:"callback"`
	FormatType string         `json:"format_type"`
	Size       int            `json:"size"`
	AuthInfo   string         `json:"auth_info"`
}

type FileUpPreReq struct {
	ParentId string `json:"parent_id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	MimeType string `json:"mime_type"`
}

type FileUpHashReq struct {
	Md5    string `json:"md5"`
	Sha1   string `json:"sha1"`
	TaskId string `json:"task_id"`
}
type FileUpHash struct {
	Finish     bool   `json:"finish"`
	Fid        string `json:"fid"`
	Thumbnail  string `json:"thumbnail"`
	FormatType string `json:"format_type"`
}

type FileUpPartReq struct {
	ObjKey     string `json:"obj_key"`
	Bucket     string `json:"bucket"`
	UploadId   string `json:"upload_id"`
	AuthInfo   string `json:"auth_info"`
	UploadUrl  string `json:"upload_url"`
	MineType   string `json:"mine_type"`
	PartNumber int    `json:"part_number"`
	TaskId     string `json:"task_id"`
	Reader     io.Reader
}

type FileUpCommitReq struct {
	ObjKey    string         `json:"obj_key"`
	Bucket    string         `json:"bucket"`
	UploadId  string         `json:"upload_id"`
	AuthInfo  string         `json:"auth_info"`
	UploadUrl string         `json:"upload_url"`
	MineType  string         `json:"mine_type"`
	TaskId    string         `json:"task_id"`
	Callback  FileUpCallback `json:"callback"`
}

type FileUpAuth struct {
	AuthKey string        `json:"auth_key"`
	Speed   int           `json:"speed"`
	Headers []interface{} `json:"headers"`
}

type FileUpFinishReq struct {
	ObjKey string `json:"obj_key"`
	TaskId string `json:"task_id"`
}

type ShareReq struct {
	FidList []string `json:"fid_list"`
	// 分享名称
	Title string `json:"title"`
	// 1 无密码 2 要密码
	UrlType int `json:"url_type"`
	// 1 无限期 2 1天 3 7天 4 30天
	ExpiredType int `json:"expired_type"`
	// 要密码的时候自动生成
	Passcode string `json:"passcode"`
}

type SharePasswordData struct {
	Title           string `json:"title"`
	SubTitle        string `json:"sub_title"`
	Thumbnail       string `json:"thumbnail"`
	ShareType       int    `json:"share_type"`
	PwdId           string `json:"pwd_id"`
	ShareUrl        string `json:"share_url"`
	UrlType         int    `json:"url_type"`
	Passcode        string `json:"passcode"`
	ExpiredType     int    `json:"expired_type"`
	FileNum         int    `json:"file_num"`
	ExpiredAt       int64  `json:"expired_at"`
	ExpireTimestamp int64  `json:"expire_timestamp"`
	FirstFile       struct {
		Fid                     string  `json:"fid"`
		Category                int     `json:"category"`
		FileType                int     `json:"file_type"`
		FormatType              string  `json:"format_type"`
		NameSpace               int     `json:"name_space"`
		SeriesDir               bool    `json:"series_dir"`
		UploadCameraRootDir     bool    `json:"upload_camera_root_dir"`
		Fps                     float64 `json:"fps"`
		Like                    int     `json:"like"`
		RiskType                int     `json:"risk_type"`
		FileNameHlStart         int     `json:"file_name_hl_start"`
		FileNameHlEnd           int     `json:"file_name_hl_end"`
		Duration                int     `json:"duration"`
		ScrapeStatus            int     `json:"scrape_status"`
		Ban                     bool    `json:"ban"`
		CurVersionOrDefault     int     `json:"cur_version_or_default"`
		OwnerDriveTypeOrDefault int     `json:"owner_drive_type_or_default"`
		BackupSource            bool    `json:"backup_source"`
		SaveAsSource            bool    `json:"save_as_source"`
		OfflineSource           bool    `json:"offline_source"`
		Dir                     bool    `json:"dir"`
		File                    bool    `json:"file"`
		Extra                   struct {
		} `json:"_extra"`
	} `json:"first_file"`
	PathInfo                 string `json:"path_info"`
	PartialViolation         bool   `json:"partial_violation"`
	FirstLayerFileCategories []int  `json:"first_layer_file_categories"`
	DownloadPvlimited        bool   `json:"download_pvlimited"`
}
type ShareList struct {
	Title       string `json:"title"`
	ShareType   int    `json:"share_type"`
	ShareId     string `json:"share_id"`
	PwdId       string `json:"pwd_id"`
	ShareUrl    string `json:"share_url"`
	UrlType     int    `json:"url_type"`
	FirstFid    string `json:"first_fid"`
	ExpiredType int    `json:"expired_type"`
	FileNum     int    `json:"file_num"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
	ExpiredAt   int64  `json:"expired_at"`
	ExpiredLeft int64  `json:"expired_left"`
	AuditStatus int    `json:"audit_status"`
	Status      int    `json:"status"`
	ClickPv     int    `json:"click_pv"`
	SavePv      int    `json:"save_pv"`
	DownloadPv  int    `json:"download_pv"`
	FirstFile   struct {
		Fid                     string  `json:"fid"`
		Category                int     `json:"category"`
		FileType                int     `json:"file_type"`
		FormatType              string  `json:"format_type"`
		NameSpace               int     `json:"name_space"`
		SeriesDir               bool    `json:"series_dir"`
		UploadCameraRootDir     bool    `json:"upload_camera_root_dir"`
		Fps                     float64 `json:"fps"`
		Like                    int     `json:"like"`
		RiskType                int     `json:"risk_type"`
		FileNameHlStart         int     `json:"file_name_hl_start"`
		FileNameHlEnd           int     `json:"file_name_hl_end"`
		Duration                int     `json:"duration"`
		ScrapeStatus            int     `json:"scrape_status"`
		Ban                     bool    `json:"ban"`
		CurVersionOrDefault     int     `json:"cur_version_or_default"`
		BackupSource            bool    `json:"backup_source"`
		SaveAsSource            bool    `json:"save_as_source"`
		OfflineSource           bool    `json:"offline_source"`
		OwnerDriveTypeOrDefault int     `json:"owner_drive_type_or_default"`
		Dir                     bool    `json:"dir"`
		File                    bool    `json:"file"`
		Extra                   struct {
		} `json:"_extra"`
	} `json:"first_file"`
	PathInfo                 string `json:"path_info"`
	PartialViolation         bool   `json:"partial_violation"`
	ViolationCnt             int    `json:"violation_cnt,omitempty"`
	Size                     int    `json:"size"`
	FirstLayerFileCategories []int  `json:"first_layer_file_categories"`
	PicTotal                 int    `json:"pic_total"`
	VideoTotal               int    `json:"video_total"`
	IsAllImageFile           int    `json:"is_all_image_file"`
	IsOwner                  int    `json:"is_owner"`
	FileOnlyNum              int    `json:"file_only_num"`
	DownloadPvlimited        bool   `json:"download_pvlimited"`
	ExpiredDays              int    `json:"expired_days,omitempty"`
	Thumbnail                string `json:"thumbnail,omitempty"`
	Passcode                 string `json:"passcode,omitempty"`
}

type ShareDetail struct {
	NotifyFollow struct {
		Allow int `json:"allow"`
	} `json:"notify_follow"`
	List []*ShareList `json:"list"`
}

type DownloadData struct {
	Fid          string `json:"fid"`
	FileName     string `json:"file_name"`
	PdirFid      string `json:"pdir_fid"`
	Category     int    `json:"category"`
	FileType     int    `json:"file_type"`
	Size         int    `json:"size"`
	FormatType   string `json:"format_type"`
	Status       int    `json:"status"`
	Tags         string `json:"tags"`
	LCreatedAt   int64  `json:"l_created_at"`
	LUpdatedAt   int64  `json:"l_updated_at"`
	NameSpace    int    `json:"name_space"`
	Thumbnail    string `json:"thumbnail"`
	DownloadUrl  string `json:"download_url"`
	Md5          string `json:"md5"`
	RiskType     int    `json:"risk_type"`
	RangeSize    int    `json:"range_size"`
	BackupSign   int    `json:"backup_sign"`
	ObjCategory  string `json:"obj_category"`
	Duration     int    `json:"duration"`
	FileSource   string `json:"file_source"`
	File         bool   `json:"file"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	PrivateExtra struct {
	} `json:"_private_extra"`
}
type ShareTokenReq struct {
	PwdId    string `json:"pwd_id"`
	Passcode string `json:"passcode"`
}
type ShareTokenResp struct {
	Subscribed bool   `json:"subscribed"`
	Stoken     string `json:"stoken"`
	ShareType  int    `json:"share_type"`
	Author     struct {
		MemberType string `json:"member_type"`
		AvatarUrl  string `json:"avatar_url"`
		NickName   string `json:"nick_name"`
	} `json:"author"`
	ExpiredType int    `json:"expired_type"`
	ExpiredAt   int64  `json:"expired_at"`
	Title       string `json:"title"`
}

type ShareDetailReq struct {
	PwdId  string `json:"pwd_id"`
	Stoken string `json:"stoken"`
}

type ShareDetailResp struct {
	IsOwner int        `json:"is_owner"`
	Share   *ShareList `json:"share"`
	List    []*File    `json:"list"`
}

type RestoreReq struct {
	FidList      []string `json:"fid_list"`
	FidTokenList []string `json:"fid_token_list"`
	ToPdirFid    string   `json:"to_pdir_fid"`
	PwdId        string   `json:"pwd_id"`
	Stoken       string   `json:"stoken"`
	PdirFid      string   `json:"pdir_fid"`
	Scene        string   `json:"scene"`
}
