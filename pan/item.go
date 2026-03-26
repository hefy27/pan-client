package pan

import (
	"context"
	"io"
	"time"
)

type Json map[string]interface{}

type DiskResp struct {
	Used  int64 `json:"used,omitempty"`
	Free  int64 `json:"free,omitempty"`
	Total int64 `json:"total,omitempty"`
	Ext   Json  `json:"ext,omitempty"`
}

type ListReq struct {
	Reload bool    `json:"reload,omitempty"`
	Dir    *PanObj `json:"dir,omitempty"`
}

type MkdirReq struct {
	NewPath string  `json:"newPath,omitempty"`
	Parent  *PanObj `json:"parent,omitempty"`
}

type DeleteReq struct {
	Items []*PanObj `json:"items,omitempty"`
}

type MovieReq struct {
	Items     []*PanObj `json:"items,omitempty"`
	TargetObj *PanObj   `json:"targetObj,omitempty"`
}

type ObjRenameReq struct {
	Obj     *PanObj `json:"obj,omitempty"`
	NewName string  `json:"newName,omitempty"`
}

type BatchRenameFunc func(obj *PanObj) string
type BatchRenameReq struct {
	Path *PanObj `json:"path,omitempty"`
	Func BatchRenameFunc
}

type PanObj struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	Type string `json:"type"`
	// 额外的数据
	Ext    Json    `json:"ext"`
	Parent *PanObj `json:"parent"`
}
type RemoteTransfer func(remote string) string

// TransferResult 传输任务结果
type TransferResult struct {
	TaskId    string              `json:"taskId"`              // 任务 ID（来自网盘的目录/文件 ID）
	FileItems []*TransferFileItem `json:"fileItems,omitempty"` // 目录任务时，各子文件信息
}

// TransferFileItem 单个文件的传输信息
type TransferFileItem struct {
	FileTaskId string `json:"fileTaskId"` // 文件 ID（来自网盘）
	FileName   string `json:"fileName"`   // 文件名
}

// ProgressEvent 传输进度事件
type ProgressEvent struct {
	TaskId    string  // 调用方传入的任务 ID（可选）
	FileId    string  // 网盘返回的文件 ID
	FileName  string  // 当前文件名
	Operated  int64   // 已传输字节
	TotalSize int64   // 总字节
	Percent   float64 // 百分比 0-100
	Speed     float64 // KB/s
	Done      bool    // 是否完成
}

// ProgressCallback 传输进度回调
type ProgressCallback func(event ProgressEvent)

type UploadFileReq struct {
	LocalFile          string          `json:"localFile,omitempty"`
	RemotePath         string          `json:"remotePath,omitempty"`
	OnlyFast           bool            `json:"onlyFast,omitempty"`
	Resumable          bool            `json:"resumable,omitempty"`
	SuccessDel         bool            `json:"successDel,omitempty"`
	TaskId             string          `json:"taskId,omitempty"` // 调用方传入的任务 ID（可选），回调中会包含
	Ctx                context.Context `json:"-"`                // Per-upload context for cancellation
	RemotePathTransfer RemoteTransfer
	RemoteNameTransfer RemoteTransfer
	ProgressCallback   ProgressCallback
}

type UploadPathReq struct {
	LocalPath          string   `json:"localPath,omitempty"`
	RemotePath         string   `json:"remotePath,omitempty"`
	Resumable          bool     `json:"resumable,omitempty"`
	SkipFileErr        bool     `json:"skipFileErr,omitempty"`
	SuccessDel         bool     `json:"successDel,omitempty"`
	OnlyFast           bool     `json:"onlyFast,omitempty"`
	IgnorePaths        []string `json:"ignorePaths,omitempty"`
	IgnoreFiles        []string `json:"ignoreFiles,omitempty"`
	Extensions         []string `json:"extensions,omitempty"`
	IgnoreExtensions   []string `json:"ignoreExtensions,omitempty"`
	RemotePathTransfer RemoteTransfer
	RemoteNameTransfer RemoteTransfer
	ProgressCallback   ProgressCallback
}

type DownloadCallback func(taskId, fileTaskId, localPath, localFile string)

type DownloadPathReq struct {
	RemotePath  *PanObj `json:"remotePath,omitempty"`
	LocalPath   string  `json:"localPath,omitempty"`
	Concurrency int     `json:"concurrency,omitempty"`
	ChunkSize   int64   `json:"chunkSize,omitempty"`
	OverCover   bool    `json:"overCover,omitempty"`
	// 不遍历子目录
	NotTraverse        bool     `json:"notTraverse,omitempty"`
	SkipFileErr        bool     `json:"skipFileErr,omitempty"`
	IgnorePaths        []string `json:"ignorePaths,omitempty"`
	IgnoreFiles        []string `json:"ignoreFiles,omitempty"`
	Extensions         []string `json:"extensions,omitempty"`
	IgnoreExtensions   []string `json:"ignoreExtensions,omitempty"`
	RemoteNameTransfer RemoteTransfer
	DownloadCallback
	ProgressCallback ProgressCallback
}

type DownloadFileReq struct {
	RemoteFile       *PanObj         `json:"remoteFile,omitempty"`
	LocalPath        string          `json:"localPath,omitempty"`
	Concurrency      int             `json:"concurrency,omitempty"`
	ChunkSize        int64           `json:"chunkSize,omitempty"`
	OverCover        bool            `json:"overCover,omitempty"`
	TaskId           string          `json:"taskId,omitempty"` // 目录级任务 ID（由 DownloadPath 传入，单文件下载可留空）
	Ctx              context.Context `json:"-"`                // Per-download context; when set, overrides the driver-level Ctx for cancellation
	DownloadCallback `json:"downloadCallback,omitempty"`
	ProgressCallback ProgressCallback
}

type OfflineDownloadReq struct {
	RemotePath string `json:"remotePath,omitempty"`
	RemoteName string `json:"remoteName,omitempty"`
	Url        string `json:"url,omitempty"`
}

type TaskListReq struct {
	Ids    []string `json:"ids,omitempty"`
	Name   string   `json:"name,omitempty"`
	Types  []string `json:"types,omitempty"`
	Phases []string `json:"phases,omitempty"`
}

type Task struct {
	Id          string    `json:"id,omitempty"`
	Name        string    `json:"name,omitempty"`
	Type        string    `json:"type,omitempty"`
	Phase       string    `json:"phase,omitempty"`
	CreatedTime time.Time `json:"createdTime"`
	UpdatedTime time.Time `json:"updatedTime"`
	Ext         Json      `json:"ext,omitempty"`
}

type DelShareReq struct {
	ShareIds []string `json:"shareIds,omitempty"`
}

type NewShareReq struct {
	// 分享的文件ID
	Fids []string `json:"fids,omitempty"`
	// 分享标题
	Title string `json:"title,omitempty"`
	// 需要密码,thunder 无效
	NeedPassCode bool `json:"needPassCode,omitempty"`
	// quark 1 无限期 2 1天 3 7天 4 30天
	// thunder -1 不限 1 1天 2 2天 3 3天 4 4天 如此类推
	ExpiredType int `json:"expiredType,omitempty"`
}

type ShareData struct {
	ShareUrl string `json:"shareUrl,omitempty"`
	ShareId  string `json:"shareId,omitempty"`
	Title    string `json:"title,omitempty"`
	PassCode string `json:"passCode,omitempty"`
	Ext      Json   `json:"ext,omitempty"`
}

type ShareListReq struct {
	ShareIds []string `json:"shareIds,omitempty"`
}

type ShareRestoreReq struct {
	// 分享的具体链接，带pwd
	ShareUrl string `json:"shareUrl,omitempty"`
	// 下面的有值会优先处理
	ShareId  string `json:"shareId,omitempty"`
	PassCode string `json:"passCode,omitempty"`
	// 保存的目录
	TargetDir string `json:"targetDir,omitempty"`
}
type DirectLinkReq struct {
	List []*DirectLink `json:"list"`
}

type DirectLink struct {
	FileId string `json:"fileId,omitempty"`
	Name   string `json:"name,omitempty"`
	Link   string `json:"link,omitempty"`
}

// ProxyFileReq is the request for ProxyFile (streaming proxy)
type ProxyFileReq struct {
	// RemoteFile is the target file to stream
	RemoteFile *PanObj

	// RangeHeader is the raw HTTP Range header from the client (e.g., "bytes=0-1023")
	// Empty string means no range (full file request)
	RangeHeader string

	// Writer is the destination for the file stream.
	// If nil, only Body in response will be set (caller manages reading).
	// If non-nil, the file data is copied to Writer and Body will be nil.
	Writer io.Writer
}

// ProxyFileResp contains the response metadata for a proxy file request
type ProxyFileResp struct {
	// StatusCode: 200 (full) or 206 (partial)
	StatusCode int

	// ContentType: MIME type (e.g., "video/mp4")
	ContentType string

	// ContentLength: body size in bytes
	ContentLength int64

	// ContentRange: e.g., "bytes 0-1023/10240" (only for 206)
	ContentRange string

	// AcceptRanges: "bytes" if the upstream supports Range
	AcceptRanges string

	// TotalSize: total file size (from PanObj.Size or upstream Content-Length)
	TotalSize int64

	// Headers: all response headers from upstream (for passthrough if needed)
	Headers map[string]string

	// Body: the response body stream (only set when Writer in request is nil)
	// Caller MUST close this when done.
	Body io.ReadCloser
}
