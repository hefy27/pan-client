package baidu_netdisk

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hefy27/pan-client/internal"
	"github.com/hefy27/pan-client/pan"
	"github.com/imroc/req/v3"
)

type BaiduNetdisk struct {
	sessionClient *req.Client
	pan.PropertiesOperate[*BaiduProperties]
	pan.CacheOperate
	pan.CommonOperate
	pan.BaseOperate

	vipType int
}

type BaiduProperties struct {
	Id           string `mapstructure:"id" json:"id" yaml:"id"`
	ClientId     string `mapstructure:"client_id" json:"client_id" yaml:"client_id"`
	ClientSecret string `mapstructure:"client_secret" json:"client_secret" yaml:"client_secret"`
	RefreshToken string `mapstructure:"refresh_token" json:"refresh_token" yaml:"refresh_token"`
	AccessToken  string `mapstructure:"access_token" json:"access_token" yaml:"access_token"`
	UploadThread int    `mapstructure:"upload_thread" json:"upload_thread" yaml:"upload_thread" default:"3"`
}

func (p *BaiduProperties) OnlyImportProperties() {}

func (p *BaiduProperties) GetId() string {
	if p.Id == "" {
		p.Id = uuid.NewString()
	}
	return p.Id
}

func (p *BaiduProperties) GetDriverType() pan.DriverType {
	return pan.BaiduNetdisk
}

func (d *BaiduNetdisk) Init() (string, error) {
	driverId := d.GetId()
	d.sessionClient = req.C().
		SetCommonHeaders(map[string]string{
			"User-Agent": "pan.baidu.com",
			"Accept":     "application/json, text/plain, */*",
		}).
		SetTimeout(30 * time.Second)
	if d.ProxyConfig.ProxyURL != "" {
		d.sessionClient.SetProxyURL(d.ProxyConfig.ProxyURL)
	}
	if d.Properties.AccessToken == "" {
		if err := d.refreshToken(); err != nil {
			return driverId, fmt.Errorf("failed to refresh token: %w", err)
		}
	}
	vip, err := d.getUserInfo()
	if err != nil {
		return driverId, fmt.Errorf("failed to get user info: %s", err.GetMsg())
	}
	d.vipType = vip
	internal.GetLogger().Info("baidu netdisk init",
		"vip_type", d.vipType, "driver_id", driverId)
	return driverId, nil
}

func (d *BaiduNetdisk) Close() error {
	d.Cancel()
	d.StopCache()
	return nil
}

func (d *BaiduNetdisk) Disk() (*pan.DiskResp, error) {
	var resp QuotaResp
	_, err := d.request(QUOTA_URL, http.MethodGet, nil, &resp)
	if err != nil {
		return nil, err
	}
	totalMB := resp.Total / 1024 / 1024
	usedMB := resp.Used / 1024 / 1024
	return &pan.DiskResp{
		Total: totalMB,
		Used:  usedMB,
		Free:  totalMB - usedMB,
	}, nil
}

func (d *BaiduNetdisk) List(req pan.ListReq) ([]*pan.PanObj, error) {
	queryDir := req.Dir
	dirPath := strings.TrimRight(queryDir.Path, "/") + "/" + queryDir.Name
	if queryDir.Path == "/" && queryDir.Name == "" {
		dirPath = "/"
	}
	cacheKey := cacheDirectoryPrefix + dirPath
	if req.Reload {
		d.Del(cacheKey)
	}
	result, err := d.GetOrLoad(cacheKey, func() (interface{}, error) {
		files, e := d.getFiles(dirPath)
		if e != nil {
			return nil, e
		}
		panObjs := make([]*pan.PanObj, 0, len(files))
		for _, f := range files {
			fileType := "file"
			if f.Isdir == 1 {
				fileType = "dir"
			}
			name := f.ServerFilename
			if name == "" {
				name = filepath.Base(f.Path)
			}
			panObjs = append(panObjs, &pan.PanObj{
				Id:     strconv.FormatInt(f.FsId, 10),
				Name:   name,
				Path:   dirPath,
				Size:   f.Size,
				Type:   fileType,
				Parent: req.Dir,
			})
		}
		return panObjs, nil
	})
	if err != nil {
		return make([]*pan.PanObj, 0), err
	}
	if objs, ok := result.([]*pan.PanObj); ok {
		return objs, nil
	}
	return make([]*pan.PanObj, 0), nil
}

func (d *BaiduNetdisk) ObjRename(req pan.ObjRenameReq) error {
	obj := req.Obj
	objPath := strings.TrimRight(obj.Path, "/") + "/" + obj.Name
	data := []map[string]string{
		{"path": objPath, "newname": req.NewName},
	}
	_, err := d.manage("rename", data)
	if err != nil {
		return err
	}
	d.Del(cacheDirectoryPrefix + obj.Path)
	return nil
}

func (d *BaiduNetdisk) BatchRename(req pan.BatchRenameReq) error {
	return d.BaseBatchRename(req, d.List, d.ObjRename, d.BatchRename)
}

func (d *BaiduNetdisk) Mkdir(req pan.MkdirReq) (*pan.PanObj, error) {
	if req.NewPath == "" {
		return &pan.PanObj{
			Id: "0", Name: "", Path: "/", Type: "dir",
		}, nil
	}
	targetPath := "/" + strings.Trim(req.NewPath, "/")
	if req.Parent != nil && req.Parent.Path != "" {
		parentPath := strings.TrimRight(req.Parent.Path, "/")
		if req.Parent.Name != "" {
			parentPath += "/" + req.Parent.Name
		}
		targetPath = parentPath + "/" + strings.Trim(req.NewPath, "/")
	}
	var newDir BaiduFile
	_, err := d.create(targetPath, 0, 1, "", "", &newDir, 0, 0)
	if err != nil {
		return nil, err
	}
	dirName := filepath.Base(targetPath)
	parentDir := filepath.Dir(targetPath)
	parentDir = strings.ReplaceAll(parentDir, "\\", "/")
	return &pan.PanObj{
		Id:   strconv.FormatInt(newDir.FsId, 10),
		Name: dirName,
		Path: parentDir,
		Type: "dir",
	}, nil
}

func (d *BaiduNetdisk) Move(req pan.MovieReq) error {
	targetObj := req.TargetObj
	if targetObj.Type == "file" {
		return pan.OnlyMsg("target must be a directory")
	}
	targetPath := strings.TrimRight(targetObj.Path, "/") + "/" + targetObj.Name
	if targetObj.Path == "/" && targetObj.Name == "" {
		targetPath = "/"
	}
	data := make([]map[string]string, 0, len(req.Items))
	reloadDirs := make(map[string]bool)
	for _, item := range req.Items {
		srcPath := strings.TrimRight(item.Path, "/") + "/" + item.Name
		data = append(data, map[string]string{
			"path":    srcPath,
			"dest":    targetPath,
			"newname": item.Name,
		})
		reloadDirs[item.Path] = true
	}
	_, err := d.manage("move", data)
	if err != nil {
		return err
	}
	for dir := range reloadDirs {
		d.Del(cacheDirectoryPrefix + dir)
	}
	d.Del(cacheDirectoryPrefix + targetPath)
	return nil
}

func (d *BaiduNetdisk) Delete(req pan.DeleteReq) error {
	if len(req.Items) == 0 {
		return nil
	}
	paths := make([]string, 0, len(req.Items))
	reloadDirs := make(map[string]bool)
	for _, item := range req.Items {
		paths = append(paths, strings.TrimRight(item.Path, "/")+"/"+item.Name)
		reloadDirs[item.Path] = true
	}
	_, err := d.manage("delete", paths)
	if err != nil {
		return err
	}
	for dir := range reloadDirs {
		d.Del(cacheDirectoryPrefix + dir)
	}
	return nil
}

func (d *BaiduNetdisk) UploadPath(req pan.UploadPathReq) (*pan.TransferResult, error) {
	err := d.BaseUploadPath(req, d.UploadFile)
	return nil, err
}

func (d *BaiduNetdisk) UploadFile(req pan.UploadFileReq) (*pan.TransferResult, error) {
	stat, err := os.Stat(req.LocalFile)
	if err != nil {
		return nil, err
	}
	if stat.Size() == 0 {
		return nil, pan.OnlyMsg("baidu does not allow empty file upload")
	}

	remoteName := stat.Name()
	remotePath := strings.TrimRight(req.RemotePath, "/")
	if req.RemotePathTransfer != nil {
		remotePath = req.RemotePathTransfer(remotePath)
	}
	if req.RemoteNameTransfer != nil {
		remoteName = req.RemoteNameTransfer(remoteName)
	}
	remoteFullPath := remotePath + "/" + remoteName

	// ensure parent directory
	parentDir := filepath.Dir(remoteFullPath)
	parentDir = strings.ReplaceAll(parentDir, "\\", "/")
	_, err = d.Mkdir(pan.MkdirReq{NewPath: parentDir})
	if err != nil {
		return nil, pan.MsgError("create parent dir error", err)
	}

	fileSize := stat.Size()
	sliceSize := d.getSliceSize(fileSize)
	count := int((fileSize + sliceSize - 1) / sliceSize)
	if count == 0 {
		count = 1
	}

	f, err := os.Open(req.LocalFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fileMd5H := md5.New()
	sliceMd5H := md5.New()
	sliceMd5First256K := md5.New()
	first256KWriter := &limitWriter{w: sliceMd5First256K, remaining: FirstSliceSize}
	blockList := make([]string, 0, count)
	lastSliceSize := fileSize % sliceSize
	if lastSliceSize == 0 {
		lastSliceSize = sliceSize
	}

	for i := 0; i < count; i++ {
		thisSlice := sliceSize
		if i == count-1 {
			thisSlice = lastSliceSize
		}
		sliceMd5H.Reset()
		_, e := io.CopyN(io.MultiWriter(fileMd5H, sliceMd5H, first256KWriter), f, thisSlice)
		if e != nil && e != io.EOF {
			return nil, e
		}
		blockList = append(blockList, hex.EncodeToString(sliceMd5H.Sum(nil)))
	}

	contentMd5 := hex.EncodeToString(fileMd5H.Sum(nil))
	sliceMd5Str := hex.EncodeToString(sliceMd5First256K.Sum(nil))
	blockListStr, _ := json.Marshal(blockList)

	// step 1: precreate
	preResp, preErr := d.precreate(remoteFullPath, fileSize, string(blockListStr), contentMd5, sliceMd5Str, 0, 0)
	if preErr != nil {
		return nil, preErr
	}
	if preResp.ReturnType == 2 {
		internal.GetLogger().Info("rapid upload success (precreate)", "file", req.LocalFile)
		if req.SuccessDel {
			_ = os.Remove(req.LocalFile)
		}
		return &pan.TransferResult{TaskId: strconv.FormatInt(preResp.File.FsId, 10)}, nil
	}

	// step 2: upload slices
	uploadUrl := d.getUploadUrl(remoteFullPath, preResp.Uploadid)
	totalParts := len(preResp.BlockList)

	for idx, partseq := range preResp.BlockList {
		if partseq < 0 {
			continue
		}
		offset := int64(partseq) * sliceSize
		thisSize := sliceSize
		if partseq == count-1 {
			thisSize = lastSliceSize
		}

		params := map[string]string{
			"method":       "upload",
			"access_token": d.Properties.AccessToken,
			"type":         "tmpfile",
			"path":         remoteFullPath,
			"uploadid":     preResp.Uploadid,
			"partseq":      strconv.Itoa(partseq),
		}
		var lastSliceErr error
		for attempt := 0; attempt < MaxUploadRetry; attempt++ {
			_, _ = f.Seek(offset, io.SeekStart)
			section := io.LimitReader(f, thisSize)
			if attempt > 0 {
				wait := time.Duration(attempt) * UploadRetryWait
				internal.GetLogger().Warn("upload slice retry",
					"file", req.LocalFile, "partseq", partseq,
					"attempt", attempt+1, "wait", wait)
				time.Sleep(wait)
			}
			lastSliceErr = d.uploadSlice(uploadUrl, params, remoteName, section, thisSize)
			if lastSliceErr == nil {
				break
			}
		}
		if lastSliceErr != nil {
			return nil, lastSliceErr
		}
		if req.ProgressCallback != nil {
			progress := float64(idx+1) / float64(totalParts) * 100
			req.ProgressCallback(pan.ProgressEvent{
				FileName:  remoteName,
				Operated:  offset + thisSize,
				TotalSize: fileSize,
				Percent:   progress,
			})
		}
	}

	// step 3: create file (commit)
	var newFile BaiduFile
	_, createErr := d.create(remoteFullPath, fileSize, 0, preResp.Uploadid, string(blockListStr), &newFile, 0, 0)
	if createErr != nil {
		return nil, createErr
	}
	internal.GetLogger().Info("upload success", "file", req.LocalFile, "fsid", newFile.FsId)
	if req.SuccessDel {
		_ = os.Remove(req.LocalFile)
	}
	return &pan.TransferResult{TaskId: strconv.FormatInt(newFile.FsId, 10)}, nil
}

func (d *BaiduNetdisk) DownloadPath(req pan.DownloadPathReq) (*pan.TransferResult, error) {
	err := d.BaseDownloadPath(req, d.List, d.DownloadFile)
	return nil, err
}

func (d *BaiduNetdisk) DownloadFile(req pan.DownloadFileReq) (*pan.TransferResult, error) {
	err := d.BaseDownloadFile(req, d.sessionClient, func(dlReq pan.DownloadFileReq) (string, error) {
		link, dlErr := d.getDownloadLink(dlReq.RemoteFile.Id)
		if dlErr != nil {
			return "", dlErr
		}
		return link, nil
	})
	return nil, err
}

func (d *BaiduNetdisk) OfflineDownload(req pan.OfflineDownloadReq) (*pan.Task, error) {
	return nil, pan.OnlyMsg("offline download not supported for baidu netdisk")
}

func (d *BaiduNetdisk) TaskList(req pan.TaskListReq) ([]*pan.Task, error) {
	return nil, pan.OnlyMsg("task list not supported for baidu netdisk")
}

func (d *BaiduNetdisk) DirectLink(req pan.DirectLinkReq) ([]*pan.DirectLink, error) {
	return nil, pan.OnlyMsg("direct link not supported for baidu netdisk")
}

func (d *BaiduNetdisk) ProxyFile(req pan.ProxyFileReq) (*pan.ProxyFileResp, error) {
	downloadUrl := func(dlReq pan.DownloadFileReq) (string, error) {
		link, dlErr := d.getDownloadLink(dlReq.RemoteFile.Id)
		if dlErr != nil {
			return "", dlErr
		}
		return link, nil
	}
	doRequest := func(httpReq *http.Request) (*http.Response, error) {
		httpReq.Header.Set("User-Agent", "pan.baidu.com")
		return d.sessionClient.GetClient().Do(httpReq)
	}
	return d.BaseProxyFile(req, downloadUrl, doRequest)
}

// Share interface stubs
func (d *BaiduNetdisk) ShareList(req pan.ShareListReq) ([]*pan.ShareData, error) {
	return nil, pan.OnlyMsg("share not supported yet for baidu netdisk")
}

func (d *BaiduNetdisk) NewShare(req pan.NewShareReq) (*pan.ShareData, error) {
	return nil, pan.OnlyMsg("share not supported yet for baidu netdisk")
}

func (d *BaiduNetdisk) DeleteShare(req pan.DelShareReq) error {
	return pan.OnlyMsg("share not supported yet for baidu netdisk")
}

func (d *BaiduNetdisk) ShareRestore(req pan.ShareRestoreReq) error {
	return pan.OnlyMsg("share not supported yet for baidu netdisk")
}

func init() {
	pan.RegisterDriver(pan.BaiduNetdisk, func() pan.Driver {
		return &BaiduNetdisk{
			PropertiesOperate: pan.PropertiesOperate[*BaiduProperties]{
				DriverType: pan.BaiduNetdisk,
			},
			CacheOperate:  pan.NewCacheOperate(),
			CommonOperate: pan.CommonOperate{},
		}
	})
}

// limitWriter writes at most `remaining` bytes to the underlying writer, then discards.
type limitWriter struct {
	w         io.Writer
	remaining int64
}

func (lw *limitWriter) Write(p []byte) (int, error) {
	if lw.remaining <= 0 {
		return len(p), nil
	}
	n := int64(len(p))
	if n > lw.remaining {
		n = lw.remaining
	}
	written, err := lw.w.Write(p[:n])
	lw.remaining -= int64(written)
	if err != nil {
		return written, err
	}
	return len(p), nil
}
