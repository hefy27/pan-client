package cloudreve

import (
	"context"
	"github.com/hefeiyu25/pan-client/pan"
	"time"
)

// Resp 基础序列化器
type Resp struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	Error string `json:"error,omitempty"`
}

// RespData 基础序列化器
type RespData[T any] struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	Error string `json:"error,omitempty"`
	Data  T      `json:"data,omitempty"`
}

type SiteConfig struct {
	SiteName             string   `json:"title"`
	LoginCaptcha         bool     `json:"loginCaptcha"`
	RegCaptcha           bool     `json:"regCaptcha"`
	ForgetCaptcha        bool     `json:"forgetCaptcha"`
	EmailActive          bool     `json:"emailActive"`
	Themes               string   `json:"themes"`
	DefaultTheme         string   `json:"defaultTheme"`
	HomepageViewMethod   string   `json:"home_view_method"`
	ShareViewMethod      string   `json:"share_view_method"`
	Authn                bool     `json:"authn"`
	User                 User     `json:"user"`
	ReCaptchaKey         string   `json:"captcha_ReCaptchaKey"`
	CaptchaType          string   `json:"captcha_type"`
	TCaptchaCaptchaAppId string   `json:"tcaptcha_captcha_app_id"`
	RegisterEnabled      bool     `json:"registerEnabled"`
	AppPromotion         bool     `json:"app_promotion"`
	WopiExts             []string `json:"wopi_exts"`
}

// User 用户序列化器
type User struct {
	ID             string    `json:"id"`
	Email          string    `json:"user_name"`
	Nickname       string    `json:"nickname"`
	Status         int       `json:"status"`
	Avatar         string    `json:"avatar"`
	CreatedAt      time.Time `json:"created_at"`
	PreferredTheme string    `json:"preferred_theme"`
	Anonymous      bool      `json:"anonymous"`
	Group          group     `json:"group"`
	Tags           []Tag     `json:"tags"`
}

type group struct {
	ID                   uint   `json:"id"`
	Name                 string `json:"name"`
	AllowShare           bool   `json:"allowShare"`
	AllowRemoteDownload  bool   `json:"allowRemoteDownload"`
	AllowArchiveDownload bool   `json:"allowArchiveDownload"`
	ShareDownload        bool   `json:"shareDownload"`
	CompressEnabled      bool   `json:"compress"`
	WebDAVEnabled        bool   `json:"webdav"`
	SourceBatchSize      int    `json:"sourceBatch"`
	AdvanceDelete        bool   `json:"advanceDelete"`
	AllowWebDAVProxy     bool   `json:"allowWebDAVProxy"`
}

type Tag struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Icon       string `json:"icon"`
	Color      string `json:"color"`
	Type       int    `json:"type"`
	Expression string `json:"expression"`
}

type Storage struct {
	Used  uint64 `json:"used"`
	Free  uint64 `json:"free"`
	Total uint64 `json:"total"`
}

// CreateUploadSessionReq 获取上传凭证服务
type CreateUploadSessionReq struct {
	Path         string `json:"path"`
	Size         uint64 `json:"size"`
	Name         string `json:"name" `
	PolicyID     string `json:"policy_id" `
	LastModified int64  `json:"last_modified"`
	MimeType     string `json:"mime_type"`
}

// UploadCredential 返回给客户端的上传凭证
type UploadCredential struct {
	SessionID   string   `json:"sessionID"`
	ChunkSize   uint64   `json:"chunkSize"` // 分块大小，0 为部分快
	Expires     int64    `json:"expires"`   // 上传凭证过期时间， Unix 时间戳
	UploadURLs  []string `json:"uploadURLs,omitempty"`
	Credential  string   `json:"credential,omitempty"`
	UploadID    string   `json:"uploadID,omitempty"`
	Callback    string   `json:"callback,omitempty"` // 回调地址
	Path        string   `json:"path,omitempty"`     // 存储路径
	AccessKey   string   `json:"ak,omitempty"`
	KeyTime     string   `json:"keyTime,omitempty"` // COS用有效期
	Policy      string   `json:"policy,omitempty"`
	CompleteURL string   `json:"completeURL,omitempty"`
}

// Sources 获取外链的结果响应
type Sources struct {
	Url    string `json:"url"`
	Name   string `json:"name"`
	Parent uint   `json:"parent"`
	Error  string `json:"error,omitempty"`
}

// ObjectProps 文件、目录对象的详细属性信息
type ObjectProps struct {
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Policy         string    `json:"policy"`
	Size           uint64    `json:"size"`
	ChildFolderNum int       `json:"child_folder_num"`
	ChildFileNum   int       `json:"child_file_num"`
	Path           string    `json:"path"`

	QueryDate time.Time `json:"query_date"`
}

// ObjectList 文件、目录列表
type ObjectList struct {
	Parent  string         `json:"parent,omitempty"`
	Objects []Object       `json:"objects"`
	Policy  *PolicySummary `json:"policy,omitempty"`
}

// Object 文件或者目录
type Object struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Path          string    `json:"path"`
	Thumb         bool      `json:"thumb"`
	Size          uint64    `json:"size"`
	Type          string    `json:"type"`
	Date          time.Time `json:"date"`
	CreateDate    time.Time `json:"create_date"`
	Key           string    `json:"key,omitempty"`
	SourceEnabled bool      `json:"source_enabled"`
}

// PolicySummary 用于前端组件使用的存储策略概况
type PolicySummary struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	MaxSize  uint64   `json:"max_size"`
	FileType []string `json:"file_type"`
}

// ItemMoveReq 处理多文件/目录移动
type ItemMoveReq struct {
	SrcDir string `json:"src_dir"`
	Src    Item   `json:"src"`
	Dst    string `json:"dst" `
}

// ItemRenameReq 处理多文件/目录重命名
type ItemRenameReq struct {
	Src     Item   `json:"src"`
	NewName string `json:"new_name" `
}

// Item 处理多文件/目录相关服务
type Item struct {
	Items []string `json:"items"`
	Dirs  []string `json:"dirs"`
}

// ItemReq 处理多文件/目录
type ItemReq struct {
	Item
	Force      bool `json:"force"`
	UnlinkOnly bool `json:"unlink"`
}

// ItemPropertyReq 获取对象属性服务
type ItemPropertyReq struct {
	Id        string
	TraceRoot bool
	IsFolder  bool
}

// ShareCreateReq 创建新分享服务
type ShareCreateReq struct {
	SourceID        string `json:"id"`
	IsDir           bool   `json:"is_dir"`
	Password        string `json:"password"`
	RemainDownloads int    `json:"downloads"`
	Expire          int    `json:"expire"`
	Preview         bool   `json:"preview"`
}

// Share 分享信息序列化
type Share struct {
	Key        string        `json:"key"`
	Locked     bool          `json:"locked"`
	IsDir      bool          `json:"is_dir"`
	CreateDate time.Time     `json:"create_date,omitempty"`
	Downloads  int           `json:"downloads"`
	Views      int           `json:"views"`
	Expire     int64         `json:"expire"`
	Preview    bool          `json:"preview"`
	Creator    *ShareCreator `json:"creator,omitempty"`
	Source     *ShareSource  `json:"source,omitempty"`
}

type ShareCreator struct {
	Key       string `json:"key"`
	Nick      string `json:"nick"`
	GroupName string `json:"group_name"`
}

type ShareSource struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

// MyShareItem 我的分享列表条目
type MyShareItem struct {
	Key             string       `json:"key"`
	IsDir           bool         `json:"is_dir"`
	Password        string       `json:"password"`
	CreateDate      time.Time    `json:"create_date,omitempty"`
	Downloads       int          `json:"downloads"`
	RemainDownloads int          `json:"remain_downloads"`
	Views           int          `json:"views"`
	Expire          int64        `json:"expire"`
	Preview         bool         `json:"preview"`
	Source          *ShareSource `json:"source,omitempty"`
}

type ShareList struct {
	total int
	items []MyShareItem
}

// 定义枚举类型
type ShareUpdateReqProp string

// 定义枚举值
const (
	Password       ShareUpdateReqProp = "password"
	PreviewEnabled ShareUpdateReqProp = "preview_enabled"
)

// ShareUpdateReq 分享更新服务
type ShareUpdateReq struct {
	Id    string             `json:"id"`
	Prop  ShareUpdateReqProp `json:"prop"`
	Value string             `json:"value"`
}

// SearchType 查询类型
type SearchType string

// 查询类型枚举值
const (
	KEYWORDS SearchType = "keywords"
	IMAGE    SearchType = "image"
	VIDEO    SearchType = "video"
	AUDIO    SearchType = "audio"
	DOC      SearchType = "doc"
	TAG      SearchType = "tag"
)

// SearchOrderBy 排序
type SearchOrderBy string

// 查询类型枚举值
const (
	CREATED_AT SearchOrderBy = "created_at"
	DOWNLOADS  SearchOrderBy = "downloads"
	VIEWS      SearchOrderBy = "views"
)

// SearchOrder 排序
type SearchOrder string

// 查询类型枚举值
const (
	DESC SearchOrder = "DESC"
	ASC  SearchOrder = "ASC"
)

// ShareListReq 列出分享
type ShareListReq struct {
	Page     uint
	OrderBy  SearchOrderBy
	Order    SearchOrder
	Keywords string
}

type OneDriveUploadReq struct {
	UploadUrl        string
	LocalFile        string
	UploadedSize     int64
	ChunkSize        int64
	TaskId           string // 调用方传入的任务 ID（可选）
	FileId           string // 网盘返回的文件 ID
	Ctx              context.Context
	ProgressCallback pan.ProgressCallback
}

type NotKnowUploadReq struct {
	UploadUrl        string
	Credential       string
	LocalFile        string
	UploadedSize     int64
	ChunkSize        int64
	TaskId           string // 调用方传入的任务 ID（可选）
	FileId           string // 网盘返回的文件 ID
	Ctx              context.Context
	ProgressCallback pan.ProgressCallback
}
