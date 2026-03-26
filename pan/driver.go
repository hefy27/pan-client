package pan

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hefy27/pan-client/internal"
	"github.com/imroc/req/v3"
)

const (
	DefaultCacheTTL = 12 * time.Hour
)

type DriverConstructor func() Driver

type DriverType string

const (
	Cloudreve       DriverType = "cloudreve"
	Quark           DriverType = "quark"
	ThunderBrowser  DriverType = "thunder_browser"
	BaiduNetdisk    DriverType = "baidu_netdisk"
	AliyundriveOpen DriverType = "aliyundrive_open"
	Aliyundrive     DriverType = "aliyundrive"
)

type Properties interface {
	// OnlyImportProperties 仅仅定义一个接口来进行继承
	OnlyImportProperties()
	GetId() string
	GetDriverType() DriverType
}

type Driver interface {
	Meta
	Operate
	Share
}

type Meta interface {
	GetId() string
	Init() (string, error)
	// Close releases all resources held by the driver (cache, goroutines, etc.).
	Close() error
	GetProperties() Properties
	Get(key string) (interface{}, bool)
	GetOrLoad(key string, loader func() (interface{}, error)) (interface{}, error)
	Set(key string, value interface{})
	SetWithTTL(key string, value interface{}, d time.Duration)
	Del(key string)
}

type Operate interface {
	Disk() (*DiskResp, error)
	List(req ListReq) ([]*PanObj, error)
	ObjRename(req ObjRenameReq) error
	BatchRename(req BatchRenameReq) error
	Mkdir(req MkdirReq) (*PanObj, error)
	Move(req MovieReq) error
	Delete(req DeleteReq) error
	UploadPath(req UploadPathReq) (*TransferResult, error)
	UploadFile(req UploadFileReq) (*TransferResult, error)
	DownloadPath(req DownloadPathReq) (*TransferResult, error)
	DownloadFile(req DownloadFileReq) (*TransferResult, error)
	OfflineDownload(req OfflineDownloadReq) (*Task, error)
	TaskList(req TaskListReq) ([]*Task, error)
	DirectLink(req DirectLinkReq) ([]*DirectLink, error)
	ProxyFile(req ProxyFileReq) (*ProxyFileResp, error)
}

// ProxyConfig holds proxy settings for a driver instance.
type ProxyConfig struct {
	ProxyURL string // 支持 http://host:port 或 socks5://host:port
}

type BaseOperate struct {
	DownloadConfig DownloadConfig
	ProxyConfig    ProxyConfig
	Ctx            context.Context
	cancelFunc     context.CancelFunc
}

// NewBaseOperate creates a BaseOperate with the given config, context, and cancel function.
func NewBaseOperate(dc DownloadConfig, pc ProxyConfig, ctx context.Context, cancel context.CancelFunc) BaseOperate {
	return BaseOperate{
		DownloadConfig: dc,
		ProxyConfig:    pc,
		Ctx:            ctx,
		cancelFunc:     cancel,
	}
}

// Cancel cancels the client's context, aborting all in-flight operations.
func (b *BaseOperate) Cancel() {
	if b.cancelFunc != nil {
		b.cancelFunc()
	}
}

func (b *BaseOperate) BaseUploadPath(req UploadPathReq, UploadFile func(req UploadFileReq) (*TransferResult, error)) error {
	localPath := req.LocalPath
	if localPath != "" {
		fileInfo, err := os.Stat(localPath)
		if err != nil {
			internal.GetLogger().Error("file read error", "file", localPath, "error", err)
			return OnlyError(err)
		}
		if !fileInfo.IsDir() {
			_, err = UploadFile(UploadFileReq{
				LocalFile:          localPath,
				RemotePath:         req.RemotePath,
				Resumable:          req.Resumable,
				OnlyFast:           req.OnlyFast,
				SuccessDel:         req.SuccessDel,
				RemotePathTransfer: req.RemotePathTransfer,
				RemoteNameTransfer: req.RemotePathTransfer,
				ProgressCallback:   req.ProgressCallback,
			})
			return err
		}
		internal.GetLogger().Info("start upload dir", "local", localPath, "remote", req.RemotePath)
		err = filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				for _, ignorePath := range req.IgnorePaths {
					if filepath.Base(path) == ignorePath {
						return filepath.SkipDir
					}
				}
			} else {
				// 获取相对于root的相对路径
				relPath, _ := filepath.Rel(localPath, path)
				relPath = strings.Replace(relPath, "\\", "/", -1)
				relPath = strings.TrimSuffix(relPath, info.Name())
				NotUpload := false
				for _, extension := range req.Extensions {
					if strings.HasSuffix(info.Name(), extension) {
						NotUpload = false
						break
					}
					NotUpload = true
				}
				for _, ignoreFile := range req.IgnoreFiles {
					if info.Name() == ignoreFile {
						NotUpload = true
						break
					}
				}
				for _, extension := range req.IgnoreExtensions {
					if strings.HasSuffix(info.Name(), extension) {
						NotUpload = true
						break
					}
				}
				if !NotUpload {
					internal.GetLogger().Info("start upload file", "local", path, "remote", strings.TrimRight(req.RemotePath, "/")+"/"+relPath)
					_, err = UploadFile(UploadFileReq{
						LocalFile:          path,
						RemotePath:         strings.TrimRight(req.RemotePath, "/") + "/" + relPath,
						OnlyFast:           req.OnlyFast,
						Resumable:          req.Resumable,
						SuccessDel:         req.SuccessDel,
						RemotePathTransfer: req.RemotePathTransfer,
						RemoteNameTransfer: req.RemotePathTransfer,
						ProgressCallback:   req.ProgressCallback,
					})
					if err == nil {
						dir := filepath.Dir(path)
						internal.GetLogger().Info("uploaded success", "dir", dir)
						if req.SuccessDel {
							if dir != "." {
								empty, _ := internal.IsEmptyDir(dir)
								if empty {
									err = os.Remove(dir)
									if err != nil {
										internal.GetLogger().Error("delete fail", "dir", dir, "error", err)
									} else {
										internal.GetLogger().Info("delete success", "dir", dir)
									}
								}
							}
						}
					} else {
						if !req.SkipFileErr {
							return err
						} else {
							internal.GetLogger().Error("upload error", "error", err)
						}
					}
					internal.GetLogger().Info("end upload file", "local", path, "remote", strings.TrimRight(req.RemotePath, "/")+"/"+relPath)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
		internal.GetLogger().Info("end upload dir", "local", localPath, "remote", req.RemotePath)
		return nil
	}
	// 遍历目录
	return OnlyMsg("path is empty")
}

func (b *BaseOperate) BaseDownloadPath(req DownloadPathReq,
	List func(req ListReq) ([]*PanObj, error),
	DownloadFile func(req DownloadFileReq) (*TransferResult, error)) error {
	dir := req.RemotePath
	remotePathName := strings.Trim(dir.Path, "/") + "/" + dir.Name
	internal.GetLogger().Info("start download dir", "remote", remotePathName, "local", req.LocalPath)
	if dir.Type != "dir" {
		return OnlyMsg("only support download dir")
	}
	objs, err := List(ListReq{
		Reload: true,
		Dir:    req.RemotePath,
	})
	if err != nil {
		return err
	}
	for _, object := range objs {
		NotDownload := false
		objectName := object.Name
		if req.RemoteNameTransfer != nil {
			objectName = req.RemoteNameTransfer(objectName)
		}
		if object.Type == "dir" {
			for _, ignorePath := range req.IgnorePaths {
				if objectName == ignorePath {
					NotDownload = true
					break
				}
			}
			if !NotDownload && req.NotTraverse == false {
				err = b.BaseDownloadPath(DownloadPathReq{
					RemotePath:         object,
					LocalPath:          strings.Trim(req.LocalPath, "/") + "/" + objectName,
					Concurrency:        req.Concurrency,
					ChunkSize:          req.ChunkSize,
					OverCover:          req.OverCover,
					DownloadCallback:   req.DownloadCallback,
					ProgressCallback:   req.ProgressCallback,
					Extensions:         req.Extensions,
					IgnorePaths:        req.IgnorePaths,
					IgnoreExtensions:   req.IgnoreExtensions,
					IgnoreFiles:        req.IgnoreFiles,
					RemoteNameTransfer: req.RemoteNameTransfer,
					SkipFileErr:        req.SkipFileErr,
				}, List, DownloadFile)
				if err != nil {
					if req.SkipFileErr {
						internal.GetLogger().Error("download error", "object", objectName, "error", err)
					} else {
						return err
					}
				}
			} else {
				internal.GetLogger().Info("dir will skip", "object", objectName)
			}
		} else {
			for _, extension := range req.Extensions {
				if strings.HasSuffix(objectName, extension) {
					NotDownload = false
					break
				}
				NotDownload = true
			}
			for _, ignoreFile := range req.IgnoreFiles {
				if objectName == ignoreFile {
					NotDownload = true
					break
				}
			}
			for _, extension := range req.IgnoreExtensions {
				if strings.HasSuffix(objectName, extension) {
					NotDownload = true
					break
				}
			}
			if !NotDownload {
				_, err = DownloadFile(DownloadFileReq{
					RemoteFile:       object,
					LocalPath:        req.LocalPath,
					Concurrency:      req.Concurrency,
					ChunkSize:        req.ChunkSize,
					OverCover:        req.OverCover,
					DownloadCallback: req.DownloadCallback,
					ProgressCallback: req.ProgressCallback,
				})
				if err != nil {
					if req.SkipFileErr {
						internal.GetLogger().Error("download error", "object", objectName, "error", err)
					} else {
						return err
					}
				}
			} else {
				internal.GetLogger().Info("file will skip", "object", objectName)
			}
		}
	}
	internal.GetLogger().Info("end download dir", "remote", remotePathName, "local", req.LocalPath)
	return nil
}

type DownloadUrl func(req DownloadFileReq) (string, error)

// ProxyClient wraps the http client used for proxy requests
type ProxyClient func(req *http.Request) (*http.Response, error)

// BaseProxyFile provides a common ProxyFile implementation for all drivers.
// Drivers call this with their own downloadUrl function and http client.
func (b *BaseOperate) BaseProxyFile(
	req ProxyFileReq,
	downloadUrl DownloadUrl,
	doRequest ProxyClient,
) (*ProxyFileResp, error) {
	if req.RemoteFile == nil || req.RemoteFile.Type != "file" {
		return nil, OnlyMsg("only support file type")
	}

	url, err := downloadUrl(DownloadFileReq{RemoteFile: req.RemoteFile})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, OnlyError(err)
	}

	if req.RangeHeader != "" {
		httpReq.Header.Set("Range", req.RangeHeader)
	}

	resp, err := doRequest(httpReq)
	if err != nil {
		return nil, OnlyError(err)
	}

	proxyResp := &ProxyFileResp{
		StatusCode:    resp.StatusCode,
		ContentType:   resp.Header.Get("Content-Type"),
		ContentLength: resp.ContentLength,
		ContentRange:  resp.Header.Get("Content-Range"),
		AcceptRanges:  resp.Header.Get("Accept-Ranges"),
		TotalSize:     req.RemoteFile.Size,
		Headers:       make(map[string]string),
	}
	for k, v := range resp.Header {
		if len(v) > 0 {
			proxyResp.Headers[k] = v[0]
		}
	}

	if proxyResp.AcceptRanges == "" && resp.StatusCode == 206 {
		proxyResp.AcceptRanges = "bytes"
	}

	if req.Writer != nil {
		defer resp.Body.Close()
		_, err = io.Copy(req.Writer, resp.Body)
		if err != nil {
			return proxyResp, OnlyError(err)
		}
	} else {
		proxyResp.Body = resp.Body
	}

	return proxyResp, nil
}

func (b *BaseOperate) BaseDownloadFile(req DownloadFileReq,
	client *req.Client,
	downloadUrl DownloadUrl) error {
	object := req.RemoteFile
	if object.Type != "file" {
		return OnlyMsg("only support download file")
	}
	remoteFileName := strings.Trim(object.Path, "/") + "/" + object.Name
	internal.GetLogger().Info("start download file", "file", remoteFileName)
	outputFile := req.LocalPath + "/" + object.Name
	fileInfo, err := internal.IsExistFile(outputFile)
	if fileInfo != nil && err == nil {
		if fileInfo.Size() == object.Size {
			if !req.OverCover {
				if req.DownloadCallback != nil {
					abs, _ := filepath.Abs(outputFile)
					req.DownloadCallback("", "", filepath.Dir(abs), abs)
				}
				internal.GetLogger().Info("end download file", "file", remoteFileName, "output", outputFile)
				return nil
			} else {
				_ = os.Remove(outputFile)
			}
		}
	}
	url, err := downloadUrl(req)
	if err != nil {
		return err
	}
	var dlCtx []context.Context
	if req.Ctx != nil {
		dlCtx = append(dlCtx, req.Ctx)
	} else if b.Ctx != nil {
		dlCtx = append(dlCtx, b.Ctx)
	}
	cd := internal.NewChunkDownload(url, client).
		SetFileSize(object.Size).
		SetChunkSize(req.ChunkSize).
		SetConcurrency(req.Concurrency).
		SetOutputFile(outputFile).
		SetTempRootDir(b.DownloadConfig.TmpPath).
		SetMaxRetry(b.DownloadConfig.MaxRetry).
		SetMaxThread(b.DownloadConfig.MaxThread)
	if req.ProgressCallback != nil {
		fileId := ""
		if object != nil {
			fileId = object.Id
		}
		cd.SetProgressFunc(func(fileName string, operated, totalSize int64, percent, speed float64, done bool) {
			req.ProgressCallback(ProgressEvent{
				TaskId:    req.TaskId,
				FileId:    fileId,
				FileName:  fileName,
				Operated:  operated,
				TotalSize: totalSize,
				Percent:   percent,
				Speed:     speed,
				Done:      done,
			})
		})
	}
	e := cd.Do(dlCtx...)
	if e != nil {
		internal.GetLogger().Error("error download file", "file", remoteFileName, "error", e)
		return e
	}

	internal.GetLogger().Info("end download file", "file", remoteFileName, "output", outputFile)
	if req.DownloadCallback != nil {
		abs, _ := filepath.Abs(outputFile)
		req.DownloadCallback("", "", filepath.Dir(abs), abs)
	}
	return nil
}

type Share interface {
	ShareList(req ShareListReq) ([]*ShareData, error)
	NewShare(req NewShareReq) (*ShareData, error)
	DeleteShare(req DelShareReq) error
	ShareRestore(req ShareRestoreReq) error
}

// OnChangeFunc is a callback invoked when driver properties change.
type OnChangeFunc func(props Properties)

type PropertiesOperate[T Properties] struct {
	Properties T
	DriverType DriverType
	OnChange   OnChangeFunc
}

func (c *PropertiesOperate[T]) GetId() string {
	return c.Properties.GetId()
}

func (c *PropertiesOperate[T]) GetProperties() Properties {
	return c.Properties
}

// initDefaults sets default values from struct tags.
func (c *PropertiesOperate[T]) initDefaults() {
	internal.SetDefaultByTag(c.Properties)
}

// NotifyChange triggers the OnChange callback if set.
func (c *PropertiesOperate[T]) NotifyChange() {
	if c.OnChange != nil {
		c.OnChange(c.Properties)
	}
}

type CacheOperate struct {
	DirCache *Cache
}

func NewCacheOperate(ttl ...time.Duration) CacheOperate {
	t := DefaultCacheTTL
	if len(ttl) > 0 && ttl[0] > 0 {
		t = ttl[0]
	}
	return CacheOperate{DirCache: NewCache(t)}
}

func (c *CacheOperate) StopCache() {
	if c.DirCache != nil {
		c.DirCache.Stop()
	}
}

func (c *CacheOperate) Get(key string) (interface{}, bool) {
	return c.DirCache.Get(key)
}

func (c *CacheOperate) GetOrLoad(key string, loader func() (interface{}, error)) (interface{}, error) {
	return c.DirCache.GetOrLoad(key, loader)
}

func (c *CacheOperate) Set(key string, value interface{}) {
	c.DirCache.Set(key, value)
}

func (c *CacheOperate) SetWithTTL(key string, value interface{}, d time.Duration) {
	c.DirCache.SetWithTTL(key, value, d)
}

func (c *CacheOperate) Del(key string) {
	c.DirCache.Del(key)
}

// BaseBatchRename provides a default BatchRename implementation.
// Drivers can call this with their own List and ObjRename methods.
func (b *BaseOperate) BaseBatchRename(req BatchRenameReq,
	List func(req ListReq) ([]*PanObj, error),
	ObjRename func(req ObjRenameReq) error,
	BatchRename func(req BatchRenameReq) error) error {
	objs, err := List(ListReq{
		Reload: true,
		Dir:    req.Path,
	})
	if err != nil {
		return err
	}
	for _, object := range objs {
		if object.Type == "dir" {
			err = BatchRename(BatchRenameReq{
				Path: object,
				Func: req.Func,
			})
			if err != nil {
				return err
			}
		}
		newName := req.Func(object)
		if newName != object.Name {
			err = ObjRename(ObjRenameReq{
				Obj:     object,
				NewName: newName,
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// CollectItemIds collects IDs from PanObj items, resolving paths when needed.
// Used by Move/Delete operations in quark and thunder_browser drivers.
type CollectResult struct {
	ObjIds       []string
	ReloadDirIds map[string]bool
}

func CollectItemIds(items []*PanObj, getPanObj func(path string, mustExist bool, list func(req ListReq) ([]*PanObj, error)) (*PanObj, error), list func(req ListReq) ([]*PanObj, error), forDelete bool) *CollectResult {
	result := &CollectResult{
		ObjIds:       make([]string, 0),
		ReloadDirIds: make(map[string]bool),
	}
	for _, item := range items {
		if item.Id != "0" && item.Id != "" {
			result.ObjIds = append(result.ObjIds, item.Id)
			if item.Type == "dir" {
				result.ReloadDirIds[item.Id] = true
			} else if forDelete && item.Parent != nil && item.Parent.Id != "" {
				result.ReloadDirIds[item.Parent.Id] = true
			}
		} else if item.Path != "" && item.Path != "/" {
			obj, err := getPanObj(item.Path, true, list)
			if err == nil {
				result.ObjIds = append(result.ObjIds, obj.Id)
				if obj.Type == "dir" {
					result.ReloadDirIds[obj.Id] = true
				} else if forDelete && item.Parent != nil && item.Parent.Id != "" {
					result.ReloadDirIds[item.Parent.Id] = true
				}
			}
		}
	}
	return result
}

// DownloadConfig holds per-client download settings.
type DownloadConfig struct {
	TmpPath   string // temp directory for chunk downloads, default "./download_tmp"
	MaxThread int    // max concurrent download goroutines, default 50
	MaxRetry  int    // max retry count per chunk, default 3
}

func (dc *DownloadConfig) ApplyDefaults() {
	if dc.TmpPath == "" {
		dc.TmpPath = "./download_tmp"
	}
	if dc.MaxThread <= 0 {
		dc.MaxThread = 50
	}
	if dc.MaxRetry <= 0 {
		dc.MaxRetry = 3
	}
}

type CommonOperate struct {
}

func (c *CommonOperate) GetPanObj(path string, mustExist bool, list func(req ListReq) ([]*PanObj, error)) (*PanObj, error) {
	truePath := strings.Trim(path, "/")
	paths := strings.Split(truePath, "/")

	target := &PanObj{
		Id:   "0",
		Name: "",
		Path: "/",
		Size: 0,
		Type: "dir",
	}
	for _, pathStr := range paths {
		if pathStr == "" {
			continue
		}
		currentChildren, err := list(ListReq{
			// 因为mustExist必须再重新来查一次
			Reload: false,
			Dir:    target,
		})
		if err != nil {
			return nil, OnlyError(err)
		}
		exist := false
		for _, file := range currentChildren {
			if file.Name == pathStr {
				target = file
				exist = true
				break
			}
		}

		// 如果必须存在且不存在，则返回错误
		if mustExist && !exist {
			currentChildren, err = list(ListReq{
				// 因为mustExist必须再重新来查一次
				Reload: false,
				Dir:    target,
			})
			if err != nil {
				return nil, OnlyError(err)
			}
			for _, file := range currentChildren {
				if file.Name == pathStr {
					target = file
					exist = true
					break
				}
			}
			if !exist {
				return nil, OnlyMsg(fmt.Sprintf("%s not found", path))
			}
		}

		// 如果不需要必须存在且不存在，则跳出循环
		if !exist {
			break
		}
	}
	return target, nil
}

type ProgressReader struct {
	readCloser       io.ReadCloser
	file             *os.File
	uploaded         int64
	chunkSize        int64
	totalSize        int64
	currentSize      int64
	currentUploaded  int64
	currentChunkNum  int64
	finish           bool
	startTime        time.Time
	chunkStartTime   time.Time
	progressCallback ProgressCallback
	ctx              context.Context
	taskId           string
	fileId           string
}

func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	if pr.ctx != nil {
		if err := pr.ctx.Err(); err != nil {
			return 0, err
		}
	}
	n, err = pr.readCloser.Read(p)
	if n > 0 {
		pr.currentUploaded += int64(n)
		uploaded := pr.uploaded + pr.currentUploaded
		// 相等即已经处理完毕
		if pr.currentSize == pr.currentUploaded {
			pr.uploaded += pr.currentSize
		}
		if pr.uploaded == pr.totalSize {
			pr.finish = true
		}
		startTime := pr.chunkStartTime
		if pr.finish {
			startTime = pr.startTime
		}
		internal.LogProgress("uploading", pr.file.Name(), startTime, pr.currentUploaded, uploaded, pr.totalSize, false)
		if pr.progressCallback != nil {
			elapsed := time.Since(startTime).Seconds()
			var speed float64
			if elapsed > 0 {
				speed = float64(pr.currentUploaded) / 1024 / elapsed
			}
			percent := float64(uploaded) / float64(pr.totalSize) * 100
			pr.progressCallback(ProgressEvent{
				TaskId:    pr.taskId,
				FileId:    pr.fileId,
				FileName:  pr.file.Name(),
				Operated:  uploaded,
				TotalSize: pr.totalSize,
				Percent:   percent,
				Speed:     speed,
				Done:      pr.finish,
			})
		}
	}
	return n, err
}
func (pr *ProgressReader) NextChunk() (int64, int64) {
	pr.readCloser = io.NopCloser(&io.LimitedReader{
		R: pr.file,
		N: pr.chunkSize,
	})
	startSize := pr.uploaded
	endSize := min(pr.totalSize, pr.uploaded+pr.chunkSize)
	pr.currentSize = endSize - startSize
	pr.currentUploaded = 0
	pr.chunkStartTime = time.Now()
	internal.LogProgress("uploading", pr.file.Name(), pr.startTime, pr.uploaded, pr.uploaded, pr.totalSize, true)
	return startSize, endSize
}

// ResetChunk seeks the underlying file back to savedUploaded and resets
// chunk-level state so the same chunk can be re-read for retry.
func (pr *ProgressReader) ResetChunk(savedUploaded int64) error {
	pr.uploaded = savedUploaded
	pr.currentUploaded = 0
	_, err := pr.file.Seek(savedUploaded, io.SeekStart)
	return err
}

func (pr *ProgressReader) Close() {
	if pr.file != nil {
		pr.file.Close()
	}
}
func (pr *ProgressReader) GetTotal() int64 {
	return pr.totalSize
}

func (pr *ProgressReader) GetUploaded() int64 {
	return pr.uploaded
}

func (pr *ProgressReader) IsFinish() bool {
	return pr.finish
}

func NewProcessReader(localFile string, chunkSize, uploaded int64, progressCb ...ProgressCallback) (*ProgressReader, DriverErrorInterface) {
	file, err := os.Open(localFile)
	if err != nil {
		return nil, OnlyError(err)
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, OnlyError(err)
	}
	// 判断是否目录，目录则无法处理
	if stat.IsDir() {
		return nil, OnlyMsg(localFile + " not a file")
	}
	// 计算剩余字节数
	totalSize := stat.Size()
	leftSize := totalSize - uploaded

	chunkNum := (leftSize / chunkSize) + 1
	if uploaded > 0 {
		// 将文件指针移动到指定的分片位置
		ret, _ := file.Seek(uploaded, 0)
		if ret == 0 {
			return nil, OnlyMsg(localFile + " seek file failed")
		}
	}
	var cb ProgressCallback
	if len(progressCb) > 0 {
		cb = progressCb[0]
	}
	return &ProgressReader{
		file:             file,
		uploaded:         uploaded,
		chunkSize:        chunkSize,
		totalSize:        totalSize,
		currentChunkNum:  chunkNum,
		startTime:        time.Now(),
		chunkStartTime:   time.Now(),
		progressCallback: cb,
	}, nil
}

// SetCtx sets a context for cancellation support on the reader.
func (pr *ProgressReader) SetCtx(ctx context.Context) {
	pr.ctx = ctx
}

// SetTaskId sets the task ID for progress events.
func (pr *ProgressReader) SetTaskId(taskId string) {
	pr.taskId = taskId
}

// SetFileId sets the file ID (from cloud storage) for progress events.
func (pr *ProgressReader) SetFileId(fileId string) {
	pr.fileId = fileId
}

type ProgressWriter struct {
	startTime        time.Time
	totalSize        int64
	uploaded         int64
	filename         string
	taskId           string
	fileId           string
	progressCallback ProgressCallback
}

func (pw *ProgressWriter) Write(b []byte) (n int, err error) {
	n = len(b)
	pw.uploaded += int64(n)
	internal.LogProgress("uploading", pw.filename, pw.startTime, pw.uploaded, pw.uploaded, pw.totalSize, false)
	if pw.progressCallback != nil {
		elapsed := time.Since(pw.startTime).Seconds()
		var speed float64
		if elapsed > 0 {
			speed = float64(pw.uploaded) / 1024 / elapsed
		}
		percent := float64(pw.uploaded) / float64(pw.totalSize) * 100
		done := pw.uploaded >= pw.totalSize
		pw.progressCallback(ProgressEvent{
			TaskId:    pw.taskId,
			FileId:    pw.fileId,
			FileName:  pw.filename,
			Operated:  pw.uploaded,
			TotalSize: pw.totalSize,
			Percent:   percent,
			Speed:     speed,
			Done:      done,
		})
	}
	return
}

func NewProgressWriter(filename string, total int64, progressCb ...ProgressCallback) *ProgressWriter {
	var cb ProgressCallback
	if len(progressCb) > 0 {
		cb = progressCb[0]
	}
	return &ProgressWriter{
		startTime:        time.Now(),
		totalSize:        total,
		uploaded:         0,
		filename:         filename,
		progressCallback: cb,
	}
}

// SetTaskId sets the task ID for progress events.
func (pw *ProgressWriter) SetTaskId(taskId string) {
	pw.taskId = taskId
}

// SetFileId sets the file ID (from cloud storage) for progress events.
func (pw *ProgressWriter) SetFileId(fileId string) {
	pw.fileId = fileId
}
