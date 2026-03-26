package aliyundrive

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"crypto/md5"

	"github.com/google/uuid"
	"github.com/hefeiyu25/pan-client/internal"
	"github.com/hefeiyu25/pan-client/pan"
	reqlib "github.com/imroc/req/v3"
)

type AliDrive struct {
	sessionClient *reqlib.Client
	pan.PropertiesOperate[*AliDriveProperties]
	pan.CacheOperate
	pan.CommonOperate
	pan.BaseOperate

	driveId string
	userId  string
}

type AliDriveProperties struct {
	Id             string `mapstructure:"id" json:"id" yaml:"id"`
	RefreshToken   string `mapstructure:"refresh_token" json:"refresh_token" yaml:"refresh_token"`
	AccessToken    string `mapstructure:"access_token" json:"access_token" yaml:"access_token"`
	OrderBy        string `mapstructure:"order_by" json:"order_by" yaml:"order_by"`
	OrderDirection string `mapstructure:"order_direction" json:"order_direction" yaml:"order_direction"`
	RapidUpload    bool   `mapstructure:"rapid_upload" json:"rapid_upload" yaml:"rapid_upload"`
	InternalUpload bool   `mapstructure:"internal_upload" json:"internal_upload" yaml:"internal_upload"`
}

func (p *AliDriveProperties) OnlyImportProperties() {}

func (p *AliDriveProperties) GetId() string {
	if p.Id == "" {
		p.Id = uuid.NewString()
	}
	return p.Id
}

func (p *AliDriveProperties) GetDriverType() pan.DriverType {
	return pan.Aliyundrive
}

func (d *AliDrive) Init() (string, error) {
	driverId := d.GetId()
	d.sessionClient = reqlib.C().
		SetCommonHeaders(map[string]string{
			"User-Agent": "Mozilla/5.0",
			"Accept":     "application/json",
		}).
		SetTimeout(30 * time.Second)
	if d.ProxyConfig.ProxyURL != "" {
		d.sessionClient.SetProxyURL(d.ProxyConfig.ProxyURL)
	}
	if err := d.refreshToken(); err != nil {
		return driverId, fmt.Errorf("failed to refresh token: %w", err)
	}
	res, err := d.request(API_URL+"/v2/user/get", http.MethodPost, nil, nil)
	if err != nil {
		return driverId, fmt.Errorf("failed to get user info: %s", err.GetMsg())
	}
	var userInfo struct {
		DefaultDriveId string `json:"default_drive_id"`
		UserId         string `json:"user_id"`
	}
	_ = json.Unmarshal(res, &userInfo)
	d.driveId = userInfo.DefaultDriveId
	d.userId = userInfo.UserId

	state := getOrCreateState(d.userId)
	signData(state, d.userId)

	d.startTokenRefreshCron()

	internal.GetLogger().Info("aliyundrive init",
		"drive_id", d.driveId, "user_id", d.userId, "driver_id", driverId)
	return driverId, nil
}

func (d *AliDrive) Close() error {
	d.Cancel()
	d.StopCache()
	return nil
}

func (d *AliDrive) Disk() (*pan.DiskResp, error) {
	res, err := d.request(API_URL+"/adrive/v1/user/driveCapacityDetails", http.MethodPost, nil, nil)
	if err != nil {
		return nil, err
	}
	var result struct {
		DriveUsedSize  int64 `json:"drive_used_size"`
		DriveTotalSize int64 `json:"drive_total_size"`
	}
	_ = json.Unmarshal(res, &result)
	totalMB := result.DriveTotalSize / 1024 / 1024
	usedMB := result.DriveUsedSize / 1024 / 1024
	return &pan.DiskResp{
		Total: totalMB,
		Used:  usedMB,
		Free:  totalMB - usedMB,
	}, nil
}

func (d *AliDrive) List(req pan.ListReq) ([]*pan.PanObj, error) {
	queryDir := req.Dir
	if queryDir.Path == "/" && queryDir.Name == "" {
		queryDir.Id = "root"
	}
	if queryDir.Id == "" {
		obj, err := d.GetPanObj(strings.TrimRight(queryDir.Path, "/")+"/"+queryDir.Name, true, d.List)
		if err != nil {
			return nil, err
		}
		queryDir = obj
	}
	cacheKey := cacheDirectoryPrefix + queryDir.Id
	if req.Reload {
		d.Del(cacheKey)
	}
	result, err := d.GetOrLoad(cacheKey, func() (interface{}, error) {
		files, e := d.getFiles(queryDir.Id)
		if e != nil {
			return nil, e
		}
		panObjs := make([]*pan.PanObj, 0, len(files))
		dirPath := strings.TrimRight(queryDir.Path, "/") + "/" + queryDir.Name
		if queryDir.Id == "root" {
			dirPath = "/"
		}
		for _, f := range files {
			fileType := "file"
			if f.Type == "folder" {
				fileType = "dir"
			}
			panObjs = append(panObjs, &pan.PanObj{
				Id:     f.FileId,
				Name:   f.Name,
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

func (d *AliDrive) ObjRename(req pan.ObjRenameReq) error {
	obj := req.Obj
	if obj.Id == "" {
		resolved, err := d.GetPanObj(strings.TrimRight(obj.Path, "/")+"/"+obj.Name, true, d.List)
		if err != nil {
			return err
		}
		obj = resolved
	}
	_, err := d.request(API_URL+"/v3/file/update", http.MethodPost, func(r *reqlib.Request) {
		r.SetBody(map[string]string{
			"check_name_mode": "refuse",
			"drive_id":        d.driveId,
			"file_id":         obj.Id,
			"name":            req.NewName,
		})
	}, nil)
	if err != nil {
		return err
	}
	if obj.Parent != nil {
		d.Del(cacheDirectoryPrefix + obj.Parent.Id)
	}
	return nil
}

func (d *AliDrive) BatchRename(req pan.BatchRenameReq) error {
	return d.BaseBatchRename(req, d.List, d.ObjRename, d.BatchRename)
}

func (d *AliDrive) Mkdir(req pan.MkdirReq) (*pan.PanObj, error) {
	if req.NewPath == "" {
		return &pan.PanObj{Id: "root", Name: "", Path: "/", Type: "dir"}, nil
	}
	targetPath := "/" + strings.Trim(req.NewPath, "/")
	if req.Parent != nil && req.Parent.Id != "root" && req.Parent.Id != "" {
		parentPath := strings.TrimRight(req.Parent.Path, "/")
		if req.Parent.Name != "" {
			parentPath += "/" + req.Parent.Name
		}
		targetPath = parentPath + "/" + strings.Trim(req.NewPath, "/")
	}
	obj, err := d.GetPanObj(targetPath, false, d.List)
	if err != nil {
		return nil, err
	}
	existPath := strings.TrimRight(obj.Path, "/") + "/" + obj.Name
	if obj.Id == "root" || obj.Path == "/" {
		existPath = "/" + obj.Name
	}
	if existPath == targetPath && obj.Type == "dir" {
		return obj, nil
	}
	rel, _ := filepath.Rel(existPath, targetPath)
	rel = strings.ReplaceAll(rel, "\\", "/")
	parts := strings.Split(rel, "/")
	currentParentId := obj.Id
	var lastObj *pan.PanObj
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		var newDir struct {
			FileId string `json:"file_id"`
		}
		_, apiErr := d.request(API_URL+"/adrive/v2/file/createWithFolders", http.MethodPost, func(r *reqlib.Request) {
			r.SetBody(map[string]interface{}{
				"check_name_mode": "refuse",
				"drive_id":        d.driveId,
				"name":            part,
				"parent_file_id":  currentParentId,
				"type":            "folder",
			})
		}, &newDir)
		if apiErr != nil {
			children, listErr := d.getFiles(currentParentId)
			if listErr != nil {
				return nil, listErr
			}
			for _, child := range children {
				if child.Name == part && child.Type == "folder" {
					newDir.FileId = child.FileId
					break
				}
			}
			if newDir.FileId == "" {
				return nil, apiErr
			}
		}
		currentParentId = newDir.FileId
		lastObj = &pan.PanObj{
			Id:   newDir.FileId,
			Name: part,
			Path: strings.TrimRight(existPath, "/") + "/" + part,
			Type: "dir",
		}
		existPath = lastObj.Path
	}
	d.Del(cacheDirectoryPrefix + obj.Id)
	if lastObj != nil {
		return lastObj, nil
	}
	return obj, nil
}

func (d *AliDrive) Move(req pan.MovieReq) error {
	targetObj := req.TargetObj
	if targetObj.Type == "file" {
		return pan.OnlyMsg("target must be a directory")
	}
	if targetObj.Id == "" {
		created, err := d.Mkdir(pan.MkdirReq{
			NewPath: strings.TrimRight(targetObj.Path, "/") + "/" + targetObj.Name,
		})
		if err != nil {
			return err
		}
		targetObj = created
	}
	reloadDirs := make(map[string]bool)
	for _, item := range req.Items {
		obj := item
		if obj.Id == "" {
			resolved, err := d.GetPanObj(strings.TrimRight(obj.Path, "/")+"/"+obj.Name, true, d.List)
			if err != nil {
				return err
			}
			obj = resolved
		}
		apiErr := d.batch(obj.Id, targetObj.Id, "/file/move")
		if apiErr != nil {
			return apiErr
		}
		if obj.Parent != nil {
			reloadDirs[obj.Parent.Id] = true
		}
	}
	for dirId := range reloadDirs {
		d.Del(cacheDirectoryPrefix + dirId)
	}
	d.Del(cacheDirectoryPrefix + targetObj.Id)
	return nil
}

func (d *AliDrive) Delete(req pan.DeleteReq) error {
	if len(req.Items) == 0 {
		return nil
	}
	reloadDirs := make(map[string]bool)
	for _, item := range req.Items {
		obj := item
		if obj.Id == "" {
			resolved, err := d.GetPanObj(strings.TrimRight(obj.Path, "/")+"/"+obj.Name, true, d.List)
			if err != nil {
				return err
			}
			obj = resolved
		}
		_, apiErr := d.request(API_URL+"/v2/recyclebin/trash", http.MethodPost, func(r *reqlib.Request) {
			r.SetBody(map[string]string{
				"drive_id": d.driveId,
				"file_id":  obj.Id,
			})
		}, nil)
		if apiErr != nil {
			return apiErr
		}
		if obj.Parent != nil {
			reloadDirs[obj.Parent.Id] = true
		}
	}
	for dirId := range reloadDirs {
		d.Del(cacheDirectoryPrefix + dirId)
	}
	return nil
}

func (d *AliDrive) UploadPath(req pan.UploadPathReq) (*pan.TransferResult, error) {
	err := d.BaseUploadPath(req, d.UploadFile)
	return nil, err
}

func (d *AliDrive) UploadFile(req pan.UploadFileReq) (*pan.TransferResult, error) {
	stat, err := os.Stat(req.LocalFile)
	if err != nil {
		return nil, err
	}
	fileSize := stat.Size()
	remoteName := stat.Name()
	remotePath := strings.TrimRight(req.RemotePath, "/")
	if req.RemotePathTransfer != nil {
		remotePath = req.RemotePathTransfer(remotePath)
	}
	if req.RemoteNameTransfer != nil {
		remoteName = req.RemoteNameTransfer(remoteName)
	}

	parentDir, err := d.Mkdir(pan.MkdirReq{NewPath: remotePath})
	if err != nil {
		return nil, pan.MsgError("create parent dir error", err)
	}

	count := int(math.Ceil(float64(fileSize) / float64(DefaultPartSize)))
	if count == 0 {
		count = 1
	}
	partInfoList := make([]map[string]int, count)
	for i := 0; i < count; i++ {
		partInfoList[i] = map[string]int{"part_number": i + 1}
	}

	f, err := os.Open(req.LocalFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reqBody := map[string]interface{}{
		"check_name_mode": "overwrite",
		"drive_id":        d.driveId,
		"name":            remoteName,
		"parent_file_id":  parentDir.Id,
		"part_info_list":  partInfoList,
		"size":            fileSize,
		"type":            "file",
	}

	if d.Properties.RapidUpload {
		preHashBuf := make([]byte, 1024)
		n, _ := f.Read(preHashBuf)
		preHashSha1 := sha1.Sum(preHashBuf[:n])
		reqBody["pre_hash"] = hex.EncodeToString(preHashSha1[:])
		_, _ = f.Seek(0, io.SeekStart)
	} else {
		reqBody["content_hash_name"] = "none"
		reqBody["proof_version"] = "v1"
	}

	var resp UploadResp
	raw, apiErr := d.request(API_URL+"/adrive/v2/file/createWithFolders", http.MethodPost, func(r *reqlib.Request) {
		r.SetBody(reqBody)
	}, &resp)

	if apiErr != nil && d.Properties.RapidUpload {
		var errResp RespErr
		_ = json.Unmarshal(raw, &errResp)
		if errResp.Code == "PreHashMatched" {
			sha1H := sha1.New()
			_, _ = io.Copy(sha1H, f)
			contentHash := hex.EncodeToString(sha1H.Sum(nil))
			_, _ = f.Seek(0, io.SeekStart)

			delete(reqBody, "pre_hash")
			reqBody["content_hash"] = contentHash
			reqBody["content_hash_name"] = "sha1"
			reqBody["proof_version"] = "v1"

			md5Hash := md5.Sum([]byte(d.Properties.AccessToken))
			md5Str := hex.EncodeToString(md5Hash[:])
			r, _ := new(big.Int).SetString(md5Str[:16], 16)
			fsize := new(big.Int).SetInt64(fileSize)
			offset := int64(0)
			if fileSize > 0 {
				offset = r.Mod(r, fsize).Int64()
			}
			buf := make([]byte, 8)
			nRead, _ := io.NewSectionReader(f, offset, 8).Read(buf)
			reqBody["proof_code"] = base64.StdEncoding.EncodeToString(buf[:nRead])
			_, _ = f.Seek(0, io.SeekStart)

			_, createErr := d.request(API_URL+"/adrive/v2/file/createWithFolders", http.MethodPost, func(r *reqlib.Request) {
				r.SetBody(reqBody)
			}, &resp)
			if createErr != nil {
				var e2 RespErr
				_ = json.Unmarshal(raw, &e2)
				if e2.Code != "PreHashMatched" {
					return nil, createErr
				}
			}
			if resp.RapidUpload {
				internal.GetLogger().Info("rapid upload success", "file", req.LocalFile)
				if req.SuccessDel {
					_ = os.Remove(req.LocalFile)
				}
				return &pan.TransferResult{TaskId: resp.FileId}, nil
			}
			_, _ = f.Seek(0, io.SeekStart)
		}
	} else if apiErr != nil {
		return nil, apiErr
	}

	for i, partInfo := range resp.PartInfoList {
		uploadUrl := partInfo.UploadUrl
		if d.Properties.InternalUpload && partInfo.InternalUploadUrl != "" {
			uploadUrl = partInfo.InternalUploadUrl
		}
		offset := int64(i) * DefaultPartSize
		thisSize := DefaultPartSize
		if remain := fileSize - offset; thisSize > remain {
			thisSize = remain
		}
		_, _ = f.Seek(offset, io.SeekStart)
		section := io.LimitReader(f, thisSize)

		var lastErr error
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				_, _ = f.Seek(offset, io.SeekStart)
				section = io.LimitReader(f, thisSize)
				time.Sleep(time.Duration(attempt) * time.Second)
			}
			httpReq, _ := http.NewRequest(http.MethodPut, uploadUrl, section)
			httpClient := &http.Client{Timeout: 5 * time.Minute}
			httpResp, e := httpClient.Do(httpReq)
			if e != nil {
				lastErr = e
				continue
			}
			_ = httpResp.Body.Close()
			if httpResp.StatusCode == http.StatusOK || httpResp.StatusCode == http.StatusConflict {
				lastErr = nil
				break
			}
			lastErr = fmt.Errorf("upload part failed: status %d", httpResp.StatusCode)
		}
		if lastErr != nil {
			return nil, lastErr
		}
		if req.ProgressCallback != nil {
			progress := float64(i+1) / float64(count) * 100
			req.ProgressCallback(pan.ProgressEvent{
				FileName:  remoteName,
				Operated:  offset + thisSize,
				TotalSize: fileSize,
				Percent:   progress,
			})
		}
	}

	_, completeErr := d.request(API_URL+"/v2/file/complete", http.MethodPost, func(r *reqlib.Request) {
		r.SetBody(map[string]string{
			"drive_id":  d.driveId,
			"file_id":   resp.FileId,
			"upload_id": resp.UploadId,
		})
	}, nil)
	if completeErr != nil {
		return nil, completeErr
	}
	internal.GetLogger().Info("upload success", "file", req.LocalFile, "file_id", resp.FileId)
	if req.SuccessDel {
		_ = os.Remove(req.LocalFile)
	}
	return &pan.TransferResult{TaskId: resp.FileId}, nil
}

func (d *AliDrive) DownloadPath(req pan.DownloadPathReq) (*pan.TransferResult, error) {
	err := d.BaseDownloadPath(req, d.List, d.DownloadFile)
	return nil, err
}

func (d *AliDrive) DownloadFile(req pan.DownloadFileReq) (*pan.TransferResult, error) {
	err := d.BaseDownloadFile(req, d.sessionClient, func(dlReq pan.DownloadFileReq) (string, error) {
		link, dlErr := d.getDownloadUrl(dlReq.RemoteFile.Id)
		if dlErr != nil {
			return "", dlErr
		}
		return link, nil
	})
	return nil, err
}

func (d *AliDrive) OfflineDownload(req pan.OfflineDownloadReq) (*pan.Task, error) {
	return nil, pan.OnlyMsg("offline download not supported for aliyundrive")
}

func (d *AliDrive) TaskList(req pan.TaskListReq) ([]*pan.Task, error) {
	return nil, pan.OnlyMsg("task list not supported for aliyundrive")
}

func (d *AliDrive) DirectLink(req pan.DirectLinkReq) ([]*pan.DirectLink, error) {
	return nil, pan.OnlyMsg("direct link not supported for aliyundrive")
}

func (d *AliDrive) ProxyFile(req pan.ProxyFileReq) (*pan.ProxyFileResp, error) {
	downloadUrl := func(dlReq pan.DownloadFileReq) (string, error) {
		link, dlErr := d.getDownloadUrl(dlReq.RemoteFile.Id)
		if dlErr != nil {
			return "", dlErr
		}
		return link, nil
	}
	doRequest := func(httpReq *http.Request) (*http.Response, error) {
		httpReq.Header.Set("Referer", "https://www.alipan.com/")
		return d.sessionClient.GetClient().Do(httpReq)
	}
	return d.BaseProxyFile(req, downloadUrl, doRequest)
}

func (d *AliDrive) ShareList(req pan.ShareListReq) ([]*pan.ShareData, error) {
	return nil, pan.OnlyMsg("share not supported yet for aliyundrive")
}
func (d *AliDrive) NewShare(req pan.NewShareReq) (*pan.ShareData, error) {
	return nil, pan.OnlyMsg("share not supported yet for aliyundrive")
}
func (d *AliDrive) DeleteShare(req pan.DelShareReq) error {
	return pan.OnlyMsg("share not supported yet for aliyundrive")
}
func (d *AliDrive) ShareRestore(req pan.ShareRestoreReq) error {
	return pan.OnlyMsg("share not supported yet for aliyundrive")
}

func init() {
	pan.RegisterDriver(pan.Aliyundrive, func() pan.Driver {
		return &AliDrive{
			PropertiesOperate: pan.PropertiesOperate[*AliDriveProperties]{
				DriverType: pan.Aliyundrive,
			},
			CacheOperate:  pan.NewCacheOperate(),
			CommonOperate: pan.CommonOperate{},
		}
	})
}
