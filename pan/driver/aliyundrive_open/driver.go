package aliyundrive_open

import (
	"crypto/sha1"
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
	"github.com/hefeiyu25/pan-client/internal"
	"github.com/hefeiyu25/pan-client/pan"
	reqlib "github.com/imroc/req/v3"
)

type AliyundriveOpen struct {
	sessionClient *reqlib.Client
	pan.PropertiesOperate[*AliyundriveOpenProperties]
	pan.CacheOperate
	pan.CommonOperate
	pan.BaseOperate

	driveId string
}

type AliyundriveOpenProperties struct {
	Id             string `mapstructure:"id" json:"id" yaml:"id"`
	ClientId       string `mapstructure:"client_id" json:"client_id" yaml:"client_id"`
	ClientSecret   string `mapstructure:"client_secret" json:"client_secret" yaml:"client_secret"`
	RefreshToken   string `mapstructure:"refresh_token" json:"refresh_token" yaml:"refresh_token"`
	AccessToken    string `mapstructure:"access_token" json:"access_token" yaml:"access_token"`
	DriveType      string `mapstructure:"drive_type" json:"drive_type" yaml:"drive_type" default:"resource"`
	OrderBy        string `mapstructure:"order_by" json:"order_by" yaml:"order_by"`
	OrderDirection string `mapstructure:"order_direction" json:"order_direction" yaml:"order_direction"`
	RapidUpload    bool   `mapstructure:"rapid_upload" json:"rapid_upload" yaml:"rapid_upload"`
	RemoveWay      string `mapstructure:"remove_way" json:"remove_way" yaml:"remove_way" default:"trash"`
}

func (p *AliyundriveOpenProperties) OnlyImportProperties() {}

func (p *AliyundriveOpenProperties) GetId() string {
	if p.Id == "" {
		p.Id = uuid.NewString()
	}
	return p.Id
}

func (p *AliyundriveOpenProperties) GetDriverType() pan.DriverType {
	return pan.AliyundriveOpen
}

func (d *AliyundriveOpen) Init() (string, error) {
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
	if d.Properties.AccessToken == "" {
		if err := d.refreshToken(); err != nil {
			return driverId, fmt.Errorf("failed to refresh token: %w", err)
		}
	}
	res, err := d.request("/adrive/v1.0/user/getDriveInfo", http.MethodPost, nil, nil)
	if err != nil {
		return driverId, fmt.Errorf("failed to get drive info: %s", err.GetMsg())
	}
	var driveInfo DriveInfoResp
	_ = json.Unmarshal(res, &driveInfo)
	switch d.Properties.DriveType {
	case "resource":
		d.driveId = driveInfo.ResourceDriveId
	case "backup":
		d.driveId = driveInfo.BackupDriveId
	default:
		d.driveId = driveInfo.DefaultDriveId
	}
	if d.driveId == "" {
		d.driveId = driveInfo.DefaultDriveId
	}
	internal.GetLogger().Info("aliyundrive open init",
		"drive_id", d.driveId, "drive_type", d.Properties.DriveType, "driver_id", driverId)
	return driverId, nil
}

func (d *AliyundriveOpen) Close() error {
	d.Cancel()
	d.StopCache()
	return nil
}

func (d *AliyundriveOpen) Disk() (*pan.DiskResp, error) {
	res, err := d.request("/adrive/v1.0/user/getSpaceInfo", http.MethodPost, nil, nil)
	if err != nil {
		return nil, err
	}
	var space SpaceInfoResp
	_ = json.Unmarshal(res, &space)
	totalMB := space.PersonalSpaceInfo.TotalSize / 1024 / 1024
	usedMB := space.PersonalSpaceInfo.UsedSize / 1024 / 1024
	return &pan.DiskResp{
		Total: totalMB,
		Used:  usedMB,
		Free:  totalMB - usedMB,
	}, nil
}

func (d *AliyundriveOpen) List(req pan.ListReq) ([]*pan.PanObj, error) {
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
			name := f.Name
			if name == "" {
				name = f.FileName
			}
			panObjs = append(panObjs, &pan.PanObj{
				Id:     f.FileId,
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

func (d *AliyundriveOpen) ObjRename(req pan.ObjRenameReq) error {
	obj := req.Obj
	if obj.Id == "" {
		resolved, err := d.GetPanObj(strings.TrimRight(obj.Path, "/")+"/"+obj.Name, true, d.List)
		if err != nil {
			return err
		}
		obj = resolved
	}
	_, err := d.request("/adrive/v1.0/openFile/update", http.MethodPost, func(r *reqlib.Request) {
		r.SetBody(map[string]string{
			"drive_id": d.driveId,
			"file_id":  obj.Id,
			"name":     req.NewName,
		})
	}, nil)
	if err != nil {
		return err
	}
	d.Del(cacheDirectoryPrefix + obj.Parent.Id)
	return nil
}

func (d *AliyundriveOpen) BatchRename(req pan.BatchRenameReq) error {
	return d.BaseBatchRename(req, d.List, d.ObjRename, d.BatchRename)
}

func (d *AliyundriveOpen) Mkdir(req pan.MkdirReq) (*pan.PanObj, error) {
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
		var newDir AliyunFile
		_, apiErr := d.request("/adrive/v1.0/openFile/create", http.MethodPost, func(r *reqlib.Request) {
			r.SetBody(map[string]interface{}{
				"drive_id":        d.driveId,
				"parent_file_id":  currentParentId,
				"name":            part,
				"type":            "folder",
				"check_name_mode": "refuse",
			})
		}, &newDir)
		if apiErr != nil {
			return nil, apiErr
		}
		if newDir.FileId == "" {
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

func (d *AliyundriveOpen) Move(req pan.MovieReq) error {
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
		_, apiErr := d.request("/adrive/v1.0/openFile/move", http.MethodPost, func(r *reqlib.Request) {
			r.SetBody(map[string]string{
				"drive_id":          d.driveId,
				"file_id":           obj.Id,
				"to_parent_file_id": targetObj.Id,
				"check_name_mode":   "ignore",
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
	d.Del(cacheDirectoryPrefix + targetObj.Id)
	return nil
}

func (d *AliyundriveOpen) Delete(req pan.DeleteReq) error {
	if len(req.Items) == 0 {
		return nil
	}
	uri := "/adrive/v1.0/openFile/recyclebin/trash"
	if d.Properties.RemoveWay == "delete" {
		uri = "/adrive/v1.0/openFile/delete"
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
		_, apiErr := d.request(uri, http.MethodPost, func(r *reqlib.Request) {
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

func (d *AliyundriveOpen) UploadPath(req pan.UploadPathReq) (*pan.TransferResult, error) {
	err := d.BaseUploadPath(req, d.UploadFile)
	return nil, err
}

func (d *AliyundriveOpen) UploadFile(req pan.UploadFileReq) (*pan.TransferResult, error) {
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

	partSize := calPartSize(fileSize)
	count := calPartCount(fileSize, partSize)

	f, err := os.Open(req.LocalFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// try rapid upload if enabled
	if d.Properties.RapidUpload && fileSize > 100*1024 {
		preHashBuf := make([]byte, 1024)
		n, _ := f.Read(preHashBuf)
		preHashSha1 := sha1.Sum(preHashBuf[:n])
		preHash := hex.EncodeToString(preHashSha1[:])
		_, _ = f.Seek(0, io.SeekStart)

		_, raw, preErr := d.createFile(parentDir.Id, remoteName, fileSize, count, "", preHash, "")
		if preErr != nil {
			var errResp ErrResp
			_ = json.Unmarshal(raw, &errResp)
			if errResp.Code == "PreHashMatched" {
				// calculate full SHA1 and proof code
				sha1H := sha1.New()
				_, _ = io.Copy(sha1H, f)
				contentHash := hex.EncodeToString(sha1H.Sum(nil))
				_, _ = f.Seek(0, io.SeekStart)

				proofCode, proofErr := calProofCode(d.Properties.AccessToken, fileSize, f)
				if proofErr != nil {
					return nil, proofErr
				}
				_, _ = f.Seek(0, io.SeekStart)

				createResp, _, createErr := d.createFile(parentDir.Id, remoteName, fileSize, count, contentHash, "", proofCode)
				if createErr != nil {
					return nil, createErr
				}
				if createResp.RapidUpload {
					internal.GetLogger().Info("rapid upload success", "file", req.LocalFile)
					if req.SuccessDel {
						_ = os.Remove(req.LocalFile)
					}
					return &pan.TransferResult{TaskId: createResp.FileId}, nil
				}
				return d.doNormalUpload(f, createResp, fileSize, partSize, count, remoteName, req)
			}
		}
		_, _ = f.Seek(0, io.SeekStart)
	}

	// normal upload
	createResp, _, createErr := d.createFile(parentDir.Id, remoteName, fileSize, count, "", "", "")
	if createErr != nil {
		return nil, createErr
	}
	return d.doNormalUpload(f, createResp, fileSize, partSize, count, remoteName, req)
}

func (d *AliyundriveOpen) doNormalUpload(f *os.File, createResp *CreateResp, fileSize, partSize int64, count int, remoteName string, req pan.UploadFileReq) (*pan.TransferResult, error) {
	preTime := time.Now()
	for i := 0; i < len(createResp.PartInfoList); i++ {
		if time.Since(preTime) > 50*time.Minute {
			newParts, refreshErr := d.getUploadUrl(count, createResp.FileId, createResp.UploadId)
			if refreshErr != nil {
				return nil, refreshErr
			}
			createResp.PartInfoList = newParts
			preTime = time.Now()
		}
		offset := int64(i) * partSize
		thisSize := partSize
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
			lastErr = d.uploadPart(createResp.PartInfoList[i].UploadUrl, section)
			if lastErr == nil {
				break
			}
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

	completeErr := d.completeUpload(createResp.FileId, createResp.UploadId)
	if completeErr != nil {
		return nil, completeErr
	}
	internal.GetLogger().Info("upload success", "file", req.LocalFile, "file_id", createResp.FileId)
	if req.SuccessDel {
		_ = os.Remove(req.LocalFile)
	}
	return &pan.TransferResult{TaskId: createResp.FileId}, nil
}

func (d *AliyundriveOpen) DownloadPath(req pan.DownloadPathReq) (*pan.TransferResult, error) {
	err := d.BaseDownloadPath(req, d.List, d.DownloadFile)
	return nil, err
}

func (d *AliyundriveOpen) DownloadFile(req pan.DownloadFileReq) (*pan.TransferResult, error) {
	err := d.BaseDownloadFile(req, d.sessionClient, func(dlReq pan.DownloadFileReq) (string, error) {
		link, dlErr := d.getDownloadUrl(dlReq.RemoteFile.Id)
		if dlErr != nil {
			return "", dlErr
		}
		return link, nil
	})
	return nil, err
}

func (d *AliyundriveOpen) OfflineDownload(req pan.OfflineDownloadReq) (*pan.Task, error) {
	return nil, pan.OnlyMsg("offline download not supported for aliyundrive open")
}

func (d *AliyundriveOpen) TaskList(req pan.TaskListReq) ([]*pan.Task, error) {
	return nil, pan.OnlyMsg("task list not supported for aliyundrive open")
}

func (d *AliyundriveOpen) DirectLink(req pan.DirectLinkReq) ([]*pan.DirectLink, error) {
	return nil, pan.OnlyMsg("direct link not supported for aliyundrive open")
}

func (d *AliyundriveOpen) ProxyFile(req pan.ProxyFileReq) (*pan.ProxyFileResp, error) {
	downloadUrl := func(dlReq pan.DownloadFileReq) (string, error) {
		link, dlErr := d.getDownloadUrl(dlReq.RemoteFile.Id)
		if dlErr != nil {
			return "", dlErr
		}
		return link, nil
	}
	doRequest := func(httpReq *http.Request) (*http.Response, error) {
		return d.sessionClient.GetClient().Do(httpReq)
	}
	return d.BaseProxyFile(req, downloadUrl, doRequest)
}

func (d *AliyundriveOpen) ShareList(req pan.ShareListReq) ([]*pan.ShareData, error) {
	return nil, pan.OnlyMsg("share not supported yet for aliyundrive open")
}
func (d *AliyundriveOpen) NewShare(req pan.NewShareReq) (*pan.ShareData, error) {
	return nil, pan.OnlyMsg("share not supported yet for aliyundrive open")
}
func (d *AliyundriveOpen) DeleteShare(req pan.DelShareReq) error {
	return pan.OnlyMsg("share not supported yet for aliyundrive open")
}
func (d *AliyundriveOpen) ShareRestore(req pan.ShareRestoreReq) error {
	return pan.OnlyMsg("share not supported yet for aliyundrive open")
}

func init() {
	pan.RegisterDriver(pan.AliyundriveOpen, func() pan.Driver {
		return &AliyundriveOpen{
			PropertiesOperate: pan.PropertiesOperate[*AliyundriveOpenProperties]{
				DriverType: pan.AliyundriveOpen,
			},
			CacheOperate:  pan.NewCacheOperate(),
			CommonOperate: pan.CommonOperate{},
		}
	})
}

// unused import guard
var _ = strconv.Itoa
