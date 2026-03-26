package thunder_browser

import "time"

type ErrResp struct {
	ErrorCode        int64  `json:"error_code"`
	ErrorMsg         string `json:"error"`
	ErrorDescription string `json:"error_description"`
	//	ErrorDetails   interface{} `json:"error_details"`
}

func (e *ErrResp) IsError() bool {
	return e.ErrorCode != 0 || e.ErrorMsg != "" || e.ErrorDescription != ""
}

type CustomTime struct {
	time.Time
}

const timeFormat = time.RFC3339

func (ct *CustomTime) UnmarshalJSON(b []byte) error {
	str := string(b)
	if str == `""` {
		*ct = CustomTime{Time: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)}
		return nil
	}

	t, err := time.Parse(`"`+timeFormat+`"`, str)
	if err != nil {
		return err
	}
	*ct = CustomTime{Time: t}
	return nil
}

type UserMeResp struct {
	Sub         string `json:"sub"`
	Name        string `json:"name"`
	Picture     string `json:"picture"`
	PhoneNumber string `json:"phone_number"`
	Providers   []struct {
		Id             string `json:"id"`
		ProviderUserId string `json:"provider_user_id"`
	} `json:"providers"`
	Password string `json:"password"`
	Status   string `json:"status"`
	Group    []struct {
		Id        string    `json:"id"`
		ExpiresAt time.Time `json:"expires_at"`
	} `json:"group"`
	CreatedAt         time.Time `json:"created_at"`
	PasswordUpdatedAt time.Time `json:"password_updated_at"`
	Id                string    `json:"id"`
	Vips              []struct {
		Id        string    `json:"id"`
		ExpiresAt time.Time `json:"expires_at"`
	} `json:"vips"`
	VipInfo []struct {
		Register   string `json:"register"`
		Autodeduct string `json:"autodeduct"`
		Daily      string `json:"daily"`
		Expire     string `json:"expire"`
		Grow       string `json:"grow"`
		IsVip      string `json:"is_vip"`
		LastPay    string `json:"last_pay"`
		Level      string `json:"level"`
		PayId      string `json:"pay_id"`
		Remind     string `json:"remind"`
		IsYear     string `json:"is_year"`
		UserVas    string `json:"user_vas"`
		VasType    string `json:"vas_type"`
		VipDetail  []struct {
			IsVIP      int       `json:"IsVIP"`
			VasType    string    `json:"VasType"`
			Start      time.Time `json:"Start"`
			End        time.Time `json:"End"`
			SurplusDay int       `json:"SurplusDay"`
		} `json:"vip_detail"`
		VipIcon struct {
			General string `json:"general"`
			Small   string `json:"small"`
		} `json:"vip_icon"`
		ExpireTime time.Time `json:"expire_time"`
	} `json:"vip_info"`
}

type LogInRequest struct {
	CaptchaToken string `json:"captcha_token"`

	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`

	Username string `json:"username"`
	Password string `json:"password"`
}

type SignInRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Provider     string `json:"provider"`
	SigninToken  string `json:"signin_token"`
}

type CoreLoginRequest struct {
	ProtocolVersion string `json:"protocolVersion"`
	SequenceNo      string `json:"sequenceNo"`
	PlatformVersion string `json:"platformVersion"`
	IsCompressed    string `json:"isCompressed"`
	Appid           string `json:"appid"`
	ClientVersion   string `json:"clientVersion"`
	PeerID          string `json:"peerID"`
	AppName         string `json:"appName"`
	SdkVersion      string `json:"sdkVersion"`
	Devicesign      string `json:"devicesign"`
	NetWorkType     string `json:"netWorkType"`
	ProviderName    string `json:"providerName"`
	DeviceModel     string `json:"deviceModel"`
	DeviceName      string `json:"deviceName"`
	OSVersion       string `json:"OSVersion"`
	Creditkey       string `json:"creditkey"`
	Hl              string `json:"hl"`
	UserName        string `json:"userName"`
	PassWord        string `json:"passWord"`
	VerifyKey       string `json:"verifyKey"`
	VerifyCode      string `json:"verifyCode"`
	IsMd5Pwd        string `json:"isMd5Pwd"`
}

type CoreLoginResp struct {
	Account            string `json:"account"`
	Creditkey          string `json:"creditkey"`
	ExpiresIn          int    `json:"expires_in"`
	IsCompressed       string `json:"isCompressed"`
	IsSetPassWord      string `json:"isSetPassWord"`
	KeepAliveMinPeriod string `json:"keepAliveMinPeriod"`
	KeepAlivePeriod    string `json:"keepAlivePeriod"`
	LoginKey           string `json:"loginKey"`
	NickName           string `json:"nickName"`
	PlatformVersion    string `json:"platformVersion"`
	ProtocolVersion    string `json:"protocolVersion"`
	SecureKey          string `json:"secureKey"`
	SequenceNo         string `json:"sequenceNo"`
	SessionID          string `json:"sessionID"`
	Timestamp          string `json:"timestamp"`
	UserID             string `json:"userID"`
	UserName           string `json:"userName"`
	UserNewNo          string `json:"userNewNo"`
	Version            string `json:"version"`
}

type RefreshTokenRequest struct {
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`

	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type TokenResp struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`

	Sub    string `json:"sub"`
	UserID string `json:"user_id"`

	Token string `json:"token"` // "超级保险箱" 访问Token
}

/*
* 验证码Token
**/
type CaptchaTokenRequest struct {
	Action       string            `json:"action"`
	CaptchaToken string            `json:"captcha_token"`
	ClientID     string            `json:"client_id"`
	DeviceID     string            `json:"device_id"`
	Meta         map[string]string `json:"meta"`
	RedirectUri  string            `json:"redirect_uri"`
}

type CaptchaTokenResponse struct {
	CaptchaToken string `json:"captcha_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Url          string `json:"url"`
}

type FileList struct {
	Kind            string   `json:"kind"`
	NextPageToken   string   `json:"next_page_token"`
	Files           []*Files `json:"files"`
	Version         string   `json:"version"`
	VersionOutdated bool     `json:"version_outdated"`
	FolderType      int8
}

type Files struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	Name     string `json:"name"`
	//UserID         string    `json:"user_id"`
	Size string `json:"size"`
	//Revision       string    `json:"revision"`
	//FileExtension  string    `json:"file_extension"`
	//MimeType       string    `json:"mime_type"`
	//Starred        bool      `json:"starred"`
	WebContentLink string     `json:"web_content_link"`
	CreatedTime    CustomTime `json:"created_time"`
	ModifiedTime   CustomTime `json:"modified_time"`
	IconLink       string     `json:"icon_link"`
	ThumbnailLink  string     `json:"thumbnail_link"`
	Md5Checksum    string     `json:"md5_checksum"`
	Hash           string     `json:"hash"`
	// Links map[string]Link `json:"links"`
	// Phase string          `json:"phase"`
	// Audit struct {
	// 	Status  string `json:"status"`
	// 	Message string `json:"message"`
	// 	Title   string `json:"title"`
	// } `json:"audit"`
	Medias []struct {
		//Category       string `json:"category"`
		//IconLink       string `json:"icon_link"`
		//IsDefault      bool   `json:"is_default"`
		//IsOrigin       bool   `json:"is_origin"`
		//IsVisible      bool   `json:"is_visible"`
		Link Link `json:"link"`
		//MediaID        string `json:"media_id"`
		//MediaName      string `json:"media_name"`
		//NeedMoreQuota  bool   `json:"need_more_quota"`
		//Priority       int    `json:"priority"`
		//RedirectLink   string `json:"redirect_link"`
		//ResolutionName string `json:"resolution_name"`
		// Video          struct {
		// 	AudioCodec string `json:"audio_codec"`
		// 	BitRate    int    `json:"bit_rate"`
		// 	Duration   int    `json:"duration"`
		// 	FrameRate  int    `json:"frame_rate"`
		// 	Height     int    `json:"height"`
		// 	VideoCodec string `json:"video_codec"`
		// 	VideoType  string `json:"video_type"`
		// 	Width      int    `json:"width"`
		// } `json:"video"`
		// VipTypes []string `json:"vip_types"`
	} `json:"medias"`
	Trashed     bool   `json:"trashed"`
	DeleteTime  string `json:"delete_time"`
	OriginalURL string `json:"original_url"`
	//Params            struct{} `json:"params"`
	//OriginalFileIndex int    `json:"original_file_index"`
	Space string `json:"space"`
	//Apps              []interface{} `json:"apps"`
	//Writable   bool   `json:"writable"`
	FolderType string `json:"folder_type"`
	//Collection interface{} `json:"collection"`
	SortName         string     `json:"sort_name"`
	UserModifiedTime CustomTime `json:"user_modified_time"`
	//SpellName         []interface{} `json:"spell_name"`
	//FileCategory      string        `json:"file_category"`
	//Tags              []interface{} `json:"tags"`
	//ReferenceEvents   []interface{} `json:"reference_events"`
	//ReferenceResource interface{}   `json:"reference_resource"`
	//Params0           struct {
	//	PlatformIcon   string `json:"platform_icon"`
	//	SmallThumbnail string `json:"small_thumbnail"`
	//} `json:"params,omitempty"`
}

type Link struct {
	URL    string    `json:"url"`
	Token  string    `json:"token"`
	Expire time.Time `json:"expire"`
	Type   string    `json:"type"`
}

type UploadTaskResponse struct {
	UploadType string `json:"upload_type"`

	Url struct {
		Kind string `json:"kind"`
	} `json:"url"`
	//UPLOAD_TYPE_FORM
	Form struct {
		Headers    struct{} `json:"headers"`
		Kind       string   `json:"kind"`
		Method     string   `json:"method"`
		MultiParts struct {
			OSSAccessKeyID string `json:"OSSAccessKeyId"`
			Signature      string `json:"Signature"`
			Callback       string `json:"callback"`
			Key            string `json:"key"`
			Policy         string `json:"policy"`
			XUserData      string `json:"x:user_data"`
		} `json:"multi_parts"`
		URL string `json:"url"`
	} `json:"form"`

	//UPLOAD_TYPE_RESUMABLE
	Resumable struct {
		Kind   string `json:"kind"`
		Params struct {
			AccessKeyID     string    `json:"access_key_id"`
			AccessKeySecret string    `json:"access_key_secret"`
			Bucket          string    `json:"bucket"`
			Endpoint        string    `json:"endpoint"`
			Expiration      time.Time `json:"expiration"`
			Key             string    `json:"key"`
			SecurityToken   string    `json:"security_token"`
		} `json:"params"`
		Provider string `json:"provider"`
	} `json:"resumable"`

	File Files `json:"file"`
	Task Task  `json:"task"`
}

type Task struct {
	Kind       string        `json:"kind"`
	Id         string        `json:"id"`
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	UserId     string        `json:"user_id"`
	Statuses   []interface{} `json:"statuses"`
	StatusSize int           `json:"status_size"`
	Params     struct {
		FolderType   string `json:"folder_type"`
		PredictSpeed string `json:"predict_speed"`
		PredictType  string `json:"predict_type"`
	} `json:"params"`
	FileId            string            `json:"file_id"`
	FileName          string            `json:"file_name"`
	FileSize          string            `json:"file_size"`
	Message           string            `json:"message"`
	CreatedTime       CustomTime        `json:"created_time"`
	UpdatedTime       CustomTime        `json:"updated_time"`
	ThirdTaskId       string            `json:"third_task_id"`
	Phase             string            `json:"phase"`
	Progress          int               `json:"progress"`
	IconLink          string            `json:"icon_link"`
	Callback          string            `json:"callback"`
	ReferenceResource ReferenceResource `json:"reference_resource"`
	Space             string            `json:"space"`
}

type ReferenceResource struct {
	Type     string `json:"@type"`
	Kind     string `json:"kind"`
	Id       string `json:"id"`
	ParentId string `json:"parent_id"`
	Name     string `json:"name"`
	Size     string `json:"size"`
	MimeType string `json:"mime_type"`
	IconLink string `json:"icon_link"`
	Hash     string `json:"hash"`
	Phase    string `json:"phase"`
	Audit    struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Title   string `json:"title"`
	} `json:"audit"`
	ThumbnailLink string `json:"thumbnail_link"`
	Params        struct {
		PlatformIcon string `json:"platform_icon"`
		UrlInfoId    string `json:"url_info_id"`
	} `json:"params"`
	Space   string        `json:"space"`
	Medias  []interface{} `json:"medias"`
	Starred bool          `json:"starred"`
	Tags    []interface{} `json:"tags"`
}

type UploadTaskRequest struct {
	Space      string `json:"space"`
	Kind       string `json:"kind"`
	ParentId   string `json:"parent_id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Hash       string `json:"hash"`
	UploadType string `json:"upload_type"`
	FolderType string `json:"folder_type"`
	Url        Url    `json:"url"`
	Params     Params `json:"params"`
}

type Url struct {
	Url   string   `json:"url"`
	Files []string `json:"files"`
}

type Params struct {
	RequireLinks string `json:"require_links"`
	WebTitle     string `json:"web_title"`
}

type TaskQueryRequest struct {
	Space string `json:"space"`
	// 参考 TaskTypeOffline，TASK_TYPE_MOVE，TASK_TYPE_UPLOAD，TASK_TYPE_EVENT_DELETION,TASK_TYPE_DELETEFILE
	Types []string `json:"type"`
	// 任务ID
	Ids []string `json:"ids"`
	// 参考 PhaseTypeError，PHASE_TYPE_RUNNING，PHASE_TYPE_PENDING，PHASE_TYPE_COMPLETE
	Phases []string `json:"phases"`
	// 参考 reference_resource
	With  string `json:"with"`
	Limit int64  `json:"size"`
}

type TaskQueryResponse struct {
	Tasks         []*Task `json:"tasks"`
	NextPageToken string  `json:"next_page_token"`
	ExpiresIn     int     `json:"expires_in"`
	ExpiresInMs   int     `json:"expires_in_ms"`
}

type MkdirResponse struct {
	UploadType string      `json:"upload_type"`
	File       *Files      `json:"file"`
	Task       interface{} `json:"task"`
}

type ShareListResp struct {
	Data          []*ShareInfo `json:"data"`
	NextPageToken string       `json:"next_page_token"`
}

type UserInfo struct {
	UserId      string `json:"user_id"`
	PortraitUrl string `json:"portrait_url"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
}

type ShareInfo struct {
	ShareId               string     `json:"share_id"`
	ShareStatus           string     `json:"share_status"`
	ShareStatusText       string     `json:"share_status_text"`
	Title                 string     `json:"title"`
	IconLink              string     `json:"icon_link"`
	ThumbnailLink         string     `json:"thumbnail_link"`
	PassCode              string     `json:"pass_code"`
	FileNum               string     `json:"file_num"`
	RestoreLimit          string     `json:"restore_limit"`
	ExpirationDays        string     `json:"expiration_days"`
	ExpirationAt          string     `json:"expiration_at"`
	RestoreCount          string     `json:"restore_count"`
	ExpirationLeft        string     `json:"expiration_left"`
	ExpirationLeftSeconds string     `json:"expiration_left_seconds"`
	ViewCount             string     `json:"view_count"`
	CreateTime            CustomTime `json:"create_time"`
	UserInfo              UserInfo   `json:"user_info"`
	ShareUrl              string     `json:"share_url"`
	FileId                string     `json:"file_id"`
	FileKind              string     `json:"file_kind"`
	FileSize              string     `json:"file_size"`
	ShareTo               string     `json:"share_to"`
	Params                struct {
		MimeType string `json:"mime_type"`
		Name     string `json:"name"`
	} `json:"params"`
}

type CreateShareReq struct {
	FileIds        []string          `json:"file_ids"`
	ShareTo        string            `json:"share_to"`
	Params         CreateShareParams `json:"params"`
	Title          string            `json:"title"`
	RestoreLimit   string            `json:"restore_limit"`
	ExpirationDays string            `json:"expiration_days"`
}

type CreateShareParams struct {
	SubscribePush      string `json:"subscribe_push"`
	WithPassCodeInLink string `json:"WithPassCodeInLink"`
}

type CreateShareResp struct {
	ShareId         string        `json:"share_id"`
	ShareUrl        string        `json:"share_url"`
	PassCode        string        `json:"pass_code"`
	ShareText       string        `json:"share_text"`
	ShareList       []interface{} `json:"share_list"`
	ShareErrorFiles []interface{} `json:"share_error_files"`
	ShareTextExt    string        `json:"share_text_ext"`
}

type ShareDetailResp struct {
	ShareStatus                  string      `json:"share_status"`
	ShareStatusText              string      `json:"share_status_text"`
	FileNum                      string      `json:"file_num"`
	ExpirationLeft               string      `json:"expiration_left"`
	ExpirationLeftSeconds        string      `json:"expiration_left_seconds"`
	ExpirationAt                 string      `json:"expiration_at"`
	RestoreCountLeft             string      `json:"restore_count_left"`
	Files                        []*Files    `json:"files"`
	UserInfo                     UserInfo    `json:"user_info"`
	NextPageToken                string      `json:"next_page_token"`
	PassCodeToken                string      `json:"pass_code_token"`
	Title                        string      `json:"title"`
	IconLink                     string      `json:"icon_link"`
	ThumbnailLink                string      `json:"thumbnail_link"`
	ContainSensitiveResourceText string      `json:"contain_sensitive_resource_text"`
	Params                       interface{} `json:"params"`
}

type ShareDetailReq struct {
	ShareId, PassCode, ParentId, PassCodeToken string
}

type AboutResp struct {
	Kind      string           `json:"kind"`
	Quota     Quota            `json:"quota"`
	ExpiresAt string           `json:"expires_at"`
	Quotas    map[string]Quota `json:"quotas"`
}

type Quota struct {
	Kind           string `json:"kind"`
	Limit          string `json:"limit"`
	Usage          string `json:"usage"`
	UsageInTrash   string `json:"usage_in_trash"`
	PlayTimesLimit string `json:"play_times_limit"`
	PlayTimesUsage string `json:"play_times_usage"`
	IsUnlimited    bool   `json:"is_unlimited"`
}

type RestoreReq struct {
	ParentId        string        `json:"parent_id"`
	ShareId         string        `json:"share_id"`
	PassCodeToken   string        `json:"pass_code_token"`
	AncestorIds     []interface{} `json:"ancestor_ids"`
	FileIds         []string      `json:"file_ids"`
	SpecifyParentId bool          `json:"specify_parent_id"`
}

type RestoreResp struct {
	ShareStatus     string      `json:"share_status"`
	ShareStatusText string      `json:"share_status_text"`
	FileId          string      `json:"file_id"`
	RestoreStatus   string      `json:"restore_status"`
	RestoreTaskId   string      `json:"restore_task_id"`
	Params          interface{} `json:"params"`
}
