package quark

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hefeiyu25/pan-client/internal"
	"github.com/hefeiyu25/pan-client/pan"
	"github.com/imroc/req/v3"
	netscapecookiejar "github.com/vanym/golang-netscape-cookiejar"
)

type Quark struct {
	sessionClient *req.Client
	defaultClient *req.Client
	pan.PropertiesOperate[*QuarkProperties]
	pan.CacheOperate
	pan.CommonOperate
	pan.BaseOperate
}

type QuarkProperties struct {
	Id          string `mapstructure:"id" json:"id" yaml:"id"`
	CookieFile  string `mapstructure:"cookie_file" json:"cookie_file" yaml:"cookie_file"` // cookies.txt 文件路径（Netscape 格式）
	RefreshTime int64  `mapstructure:"refresh_time" json:"refresh_time" yaml:"refresh_time" default:"0"`
	ChunkSize   int64  `mapstructure:"chunk_size" json:"chunk_size" yaml:"chunk_size" default:"52428800"` // 50M
}

func (cp *QuarkProperties) OnlyImportProperties() {
	// do nothing
}

func (cp *QuarkProperties) GetId() string {
	if cp.Id == "" {
		cp.Id = uuid.NewString()
	}
	return cp.Id
}

func (cp *QuarkProperties) GetDriverType() pan.DriverType {
	return pan.Quark
}

func (q *Quark) Init() (string, error) {
	driverId := q.GetId()

	if q.Properties.CookieFile == "" {
		return driverId, fmt.Errorf("please set cookie_file to a valid cookies.txt path")
	}

	// 创建支持自动回写的 Netscape cookie jar
	subJar, err := cookiejar.New(nil)
	if err != nil {
		return driverId, fmt.Errorf("failed to create cookie jar: %w", err)
	}
	jar, err := netscapecookiejar.New(&netscapecookiejar.Options{
		SubJar:        subJar,
		AutoWritePath: q.Properties.CookieFile,
		WriteHeader:   true,
	})
	if err != nil {
		return driverId, fmt.Errorf("failed to create netscape cookie jar: %w", err)
	}

	// 从 cookies.txt 加载 cookies
	file, err := os.Open(q.Properties.CookieFile)
	if err != nil {
		return driverId, fmt.Errorf("failed to open cookies.txt: %w", err)
	}
	_, err = jar.ReadFrom(file)
	file.Close()
	if err != nil {
		return driverId, fmt.Errorf("failed to read cookies.txt: %w", err)
	}
	internal.GetLogger().Info("loaded cookies", "file", q.Properties.CookieFile)

	q.sessionClient = req.C().
		SetCommonHeaders(map[string]string{
			HeaderUserAgent: DefaultUserAgent,
			"Accept":        "application/json, text/plain, */*",
			"Referer":       "https://pan.quark.cn",
		}).
		SetCommonQueryParam("pr", "ucpro").
		SetCommonQueryParam("fr", "pc").
		SetCookieJar(jar).
		SetTimeout(30 * time.Minute).SetBaseURL("https://drive.quark.cn/1/clouddrive")
	q.defaultClient = req.C().SetTimeout(30 * time.Minute)
	// 应用代理配置
	if q.ProxyConfig.ProxyURL != "" {
		q.sessionClient.SetProxyURL(q.ProxyConfig.ProxyURL)
		q.defaultClient.SetProxyURL(q.ProxyConfig.ProxyURL)
	}
	// 若一小时内更新过，则不重新刷session
	if q.Properties.RefreshTime == 0 || time.Now().UnixMilli()-q.Properties.RefreshTime > 60*60*1000 {
		_, err = q.config()
		if err != nil {
			return driverId, err
		} else {
			q.Properties.RefreshTime = time.Now().UnixMilli()
			q.NotifyChange()
		}
	}
	return driverId, nil
}

func (q *Quark) Close() error {
	q.Cancel()
	q.StopCache()
	return nil
}

func (q *Quark) Disk() (*pan.DiskResp, error) {
	memberResp, err := q.member()
	if err != nil {
		return nil, err
	}
	return &pan.DiskResp{
		Total: memberResp.Data.TotalCapacity / 1024 / 1024,
		Free:  (memberResp.Data.TotalCapacity - memberResp.Data.UseCapacity) / 1024 / 1024,
		Used:  memberResp.Data.UseCapacity / 1024 / 1024,
	}, nil
}
func (q *Quark) List(req pan.ListReq) ([]*pan.PanObj, error) {
	queryDir := req.Dir
	if queryDir.Path == "/" && queryDir.Name == "" {
		queryDir.Id = "0"
	}
	if queryDir.Id == "" {
		obj, err := q.GetPanObj(strings.TrimRight(queryDir.Path, "/")+"/"+queryDir.Name, true, q.List)
		if err != nil {
			return nil, err
		}
		queryDir = obj
	}
	cacheKey := cacheDirectoryPrefix + queryDir.Id
	if req.Reload {
		q.Del(cacheKey)
	}
	result, err := q.GetOrLoad(cacheKey, func() (interface{}, error) {
		files, e := q.fileSort(queryDir.Id)
		if e != nil {
			internal.GetLogger().Error("file sort error", "error", e)
			return nil, e
		}
		panObjs := make([]*pan.PanObj, 0)
		for _, item := range files {
			fileType := "file"
			if item.FileType == 0 {
				fileType = "dir"
			}
			path := strings.TrimRight(queryDir.Path, "/") + "/" + queryDir.Name
			if queryDir.Id == "0" {
				path = "/"
			}
			panObjs = append(panObjs, &pan.PanObj{
				Id:     item.Fid,
				Name:   item.FileName,
				Path:   path,
				Size:   int64(item.Size),
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
func (q *Quark) ObjRename(req pan.ObjRenameReq) error {
	if req.Obj.Id == "0" || (req.Obj.Path == "/" && req.Obj.Name == "") {
		return pan.OnlyMsg("not support rename root path")
	}
	object := req.Obj
	if object.Id == "" {
		path := strings.Trim(req.Obj.Path, "/") + "/" + req.Obj.Name
		obj, err := q.GetPanObj(path, true, q.List)
		if err != nil {
			return err
		}
		object = obj
	}
	err := q.objectRename(object.Id, req.NewName)
	if err != nil {
		return err
	}
	q.Del(cacheDirectoryPrefix + object.Parent.Id)
	return nil
}
func (q *Quark) BatchRename(req pan.BatchRenameReq) error {
	return q.BaseBatchRename(req, q.List, q.ObjRename, q.BatchRename)
}
func (q *Quark) Mkdir(req pan.MkdirReq) (*pan.PanObj, error) {
	if req.NewPath == "" {
		// 不处理，直接返回
		return &pan.PanObj{
			Id:   "0",
			Name: "",
			Path: "/",
			Size: 0,
			Type: "dir",
		}, nil
	}
	if filepath.Ext(req.NewPath) != "" {
		return nil, pan.OnlyMsg("please set a dir")
	}
	targetPath := "/" + strings.Trim(req.NewPath, "/")
	if req.Parent != nil && (req.Parent.Id == "0" || req.Parent.Path == "/") {
		targetPath = req.Parent.Path + "/" + strings.Trim(req.NewPath, "/")
	}
	obj, err := q.GetPanObj(targetPath, false, q.List)
	if err != nil {
		return nil, err
	}
	existPath := obj.Path + "/" + obj.Name
	if obj.Id == "0" || obj.Path == "/" {
		existPath = "/" + obj.Name
	}
	if existPath == targetPath {
		return obj, nil
	} else {
		rel, err := filepath.Rel(existPath, targetPath)
		rel = strings.Replace(rel, "\\", "/", -1)
		if err != nil {
			return nil, pan.OnlyError(err)
		}
		split := strings.Split(rel, "/")
		targetDirId := obj.Id
		var lastDirName string
		for _, s := range split {
			// 先查已有子目录，避免不必要的创建请求（防止 file is doloading）
			existFid := q.findChildDirId(targetDirId, s)
			if existFid != "" {
				targetDirId = existFid
				lastDirName = s
				continue
			}

			var resp *RespData[Dir]
			var err pan.DriverErrorInterface
			// 重试逻辑：file is doloading 时递增等待重试（最多3次）
			for attempt := 0; attempt <= 3; attempt++ {
				resp, err = q.createDirectory(s, targetDirId)
				if err == nil {
					break
				}
				if !strings.Contains(err.GetMsg(), "file is doloading") {
					break
				}
				if attempt < 3 {
					wait := time.Duration(attempt+1) * time.Second
					internal.GetLogger().Warn("file is doloading, retrying", "name", s, "attempt", attempt+1, "wait", wait)
					time.Sleep(wait)
				}
			}
			if err != nil {
				// 冲突时刷新缓存再查一次
				q.Del(cacheDirectoryPrefix + targetDirId)
				existFid = q.findChildDirId(targetDirId, s)
				if existFid != "" {
					targetDirId = existFid
					lastDirName = s
					continue
				}
				return nil, pan.OnlyError(err)
			}
			targetDirId = resp.Data.Fid
			lastDirName = s
		}
		// 清除父目录缓存以便后续 List 可见
		q.Del(cacheDirectoryPrefix + obj.Id)
		parentPath := targetPath[:len(targetPath)-len(lastDirName)-1]
		if parentPath == "" {
			parentPath = "/"
		}
		return &pan.PanObj{
			Id:   targetDirId,
			Name: lastDirName,
			Path: parentPath,
			Type: "dir",
			Parent: &pan.PanObj{
				Id:   obj.Id,
				Path: obj.Path,
				Name: obj.Name,
				Type: "dir",
			},
		}, nil
	}
}

// findChildDirId 在指定父目录下查找名为 name 的子目录，返回其 fid
func (q *Quark) findChildDirId(parentId, name string) string {
	files, err := q.fileSort(parentId)
	if err != nil {
		return ""
	}
	for _, f := range files {
		if f.FileName == name && f.Dir {
			return f.Fid
		}
	}
	return ""
}

func (q *Quark) Move(req pan.MovieReq) error {
	targetObj := req.TargetObj
	if targetObj.Type == "file" {
		return pan.OnlyMsg("target is a file")
	}
	// 重新直接创建目标目录
	if targetObj.Id == "" {
		create, err := q.Mkdir(pan.MkdirReq{
			NewPath: strings.Trim(targetObj.Path, "/") + "/" + targetObj.Name,
		})
		if err != nil {
			return err
		}
		targetObj = create
	}
	collected := pan.CollectItemIds(req.Items, q.GetPanObj, q.List, false)
	err := q.objectMove(collected.ObjIds, targetObj.Id)
	if err != nil {
		return pan.OnlyError(err)
	}
	for key := range collected.ReloadDirIds {
		q.Del(cacheDirectoryPrefix + key)
	}
	return nil
}
func (q *Quark) Delete(req pan.DeleteReq) error {
	if len(req.Items) == 0 {
		return nil
	}
	collected := pan.CollectItemIds(req.Items, q.GetPanObj, q.List, true)
	if len(collected.ObjIds) > 0 {
		err := q.objectDelete(collected.ObjIds)
		if err != nil {
			return err
		}
		for key := range collected.ReloadDirIds {
			q.Del(cacheDirectoryPrefix + key)
		}
	}
	return nil
}

func (q *Quark) UploadPath(req pan.UploadPathReq) (*pan.TransferResult, error) {
	err := q.BaseUploadPath(req, q.UploadFile)
	return nil, err
}

func (q *Quark) UploadFile(req pan.UploadFileReq) (*pan.TransferResult, error) {
	if req.Resumable {
		internal.GetLogger().Warn("quark is not support resumeable")
	}
	stat, err := os.Stat(req.LocalFile)
	if err != nil {
		return nil, err
	}
	remoteName := stat.Name()
	remotePath := strings.TrimRight(req.RemotePath, "/")
	if req.RemotePathTransfer != nil {
		remotePath = req.RemotePathTransfer(remotePath)
	}
	if req.RemoteNameTransfer != nil {
		remoteName = req.RemoteNameTransfer(remoteName)
	}
	remoteAllPath := remotePath + "/" + remoteName
	_, err = q.GetPanObj(remoteAllPath, true, q.List)
	// 没有报错证明文件已经存在
	if err == nil {
		return nil, pan.CodeMsg(CodeObjectExist, remoteAllPath+" is exist")
	}
	dir, err := q.Mkdir(pan.MkdirReq{
		NewPath: remotePath,
	})
	if err != nil {
		return nil, pan.MsgError(remotePath+" create error", err)
	}

	md5Str, err := internal.GetFileMd5(req.LocalFile)
	if err != nil {
		return nil, err
	}
	sha1Str, err := internal.GetFileSha1(req.LocalFile)
	if err != nil {
		return nil, err
	}

	mimeType := internal.GetMimeType(req.LocalFile)

	pre, err := q.FileUploadPre(FileUpPreReq{
		ParentId: dir.Id,
		FileName: remoteName,
		FileSize: stat.Size(),
		MimeType: mimeType,
	})
	if err != nil {
		return nil, err
	}

	// 使用网盘返回的文件 ID
	fileTaskId := pre.Data.Fid
	result := &pan.TransferResult{TaskId: fileTaskId}

	// hash
	finish, err := q.FileUploadHash(FileUpHashReq{
		Md5:    md5Str,
		Sha1:   sha1Str,
		TaskId: pre.Data.TaskId,
	})
	if err != nil {
		return result, err
	}
	if finish.Data.Finish {
		internal.GetLogger().Info("upload fast success", "file", req.LocalFile, "fid", fileTaskId)
		if req.SuccessDel {
			err = os.Remove(req.LocalFile)
			if err != nil {
				internal.GetLogger().Error("delete fail", "file", req.LocalFile, "error", err)
			} else {
				internal.GetLogger().Info("delete success", "file", req.LocalFile)
			}
		}
		return result, nil
	}

	if req.OnlyFast {
		internal.GetLogger().Info("upload fast error", "file", req.LocalFile)
		return result, pan.OnlyMsg("only support fast error:" + req.LocalFile)
	}

	// part up
	partSize := min(int64(pre.Metadata.PartSize), q.Properties.ChunkSize)
	total := stat.Size()
	left := total
	partNumber := 1
	pr, err := pan.NewProcessReader(req.LocalFile, partSize, 0, req.ProgressCallback)
	if err != nil {
		return result, err
	}
	if req.Ctx != nil {
		pr.SetCtx(req.Ctx)
	}
	if req.TaskId != "" {
		pr.SetTaskId(req.TaskId)
	}
	pr.SetFileId(fileTaskId)
	md5s := make([]string, 0)
	const maxPartRetry = 3
	for left > 0 {
		savedUploaded := pr.GetUploaded()

		var m string
		var lastErr error
		for attempt := 0; attempt < maxPartRetry; attempt++ {
			if attempt > 0 {
				if resetErr := pr.ResetChunk(savedUploaded); resetErr != nil {
					return result, resetErr
				}
				wait := time.Duration(attempt) * time.Second
				internal.GetLogger().Warn("chunk upload retry",
					"file", req.LocalFile, "part", partNumber,
					"attempt", attempt+1, "wait", wait, "error", lastErr)
				time.Sleep(wait)
			}
			start, end := pr.NextChunk()
			if attempt == 0 {
				left -= end - start
			}
			m, lastErr = q.FileUpPart(FileUpPartReq{
				ObjKey:     pre.Data.ObjKey,
				Bucket:     pre.Data.Bucket,
				UploadId:   pre.Data.UploadId,
				AuthInfo:   pre.Data.AuthInfo,
				UploadUrl:  pre.Data.UploadUrl,
				MineType:   mimeType,
				PartNumber: partNumber,
				TaskId:     pre.Data.TaskId,
				Reader:     pr,
			})
			if lastErr == nil {
				break
			}
		}
		if lastErr != nil {
			return result, lastErr
		}
		if m == "finish" {
			internal.GetLogger().Info("upload success", "file", req.LocalFile, "fid", fileTaskId)
			if req.SuccessDel {
				err = os.Remove(req.LocalFile)
				if err != nil {
					internal.GetLogger().Error("delete fail", "file", req.LocalFile, "error", err)
				} else {
					internal.GetLogger().Info("delete success", "file", req.LocalFile)
				}
			}
			return result, nil
		}
		md5s = append(md5s, m)
		partNumber++
	}
	err = q.FileUpCommit(FileUpCommitReq{
		ObjKey:    pre.Data.ObjKey,
		Bucket:    pre.Data.Bucket,
		UploadId:  pre.Data.UploadId,
		AuthInfo:  pre.Data.AuthInfo,
		UploadUrl: pre.Data.UploadUrl,
		MineType:  mimeType,
		TaskId:    pre.Data.TaskId,
		Callback:  pre.Data.Callback,
	}, md5s)
	if err != nil {
		return result, err
	}
	_, err = q.FileUpFinish(FileUpFinishReq{
		ObjKey: pre.Data.ObjKey,
		TaskId: pre.Data.TaskId,
	})
	if err != nil {
		return result, err
	}
	internal.GetLogger().Info("upload success", "file", req.LocalFile, "fid", fileTaskId)
	if req.SuccessDel {
		err = os.Remove(req.LocalFile)
		if err != nil {
			internal.GetLogger().Error("delete fail", "file", req.LocalFile, "error", err)
		} else {
			internal.GetLogger().Info("delete success", "file", req.LocalFile)
		}
	}
	return result, nil
}

func (q *Quark) DownloadPath(req pan.DownloadPathReq) (*pan.TransferResult, error) {
	err := q.BaseDownloadPath(req, q.List, q.DownloadFile)
	return nil, err
}
func (q *Quark) DownloadFile(req pan.DownloadFileReq) (*pan.TransferResult, error) {
	err := q.BaseDownloadFile(req, q.sessionClient, func(req pan.DownloadFileReq) (string, error) {
		resp, err := q.fileDownload(req.RemoteFile.Id)
		if err != nil {
			return "", err
		}
		return resp.Data[0].DownloadUrl, nil
	})
	return nil, err
}

func (q *Quark) OfflineDownload(req pan.OfflineDownloadReq) (*pan.Task, error) {
	return nil, pan.OnlyMsg("offline download not support")
}

func (q *Quark) TaskList(req pan.TaskListReq) ([]*pan.Task, error) {
	return nil, pan.OnlyMsg("task list not support")
}

func (q *Quark) ShareList(req pan.ShareListReq) ([]*pan.ShareData, error) {
	needFilter := len(req.ShareIds) > 0
	details, err := q.shareList()
	if err != nil {
		return nil, err
	}
	result := make([]*pan.ShareData, 0)
	for _, share := range details {
		if needFilter {
			exist := false
			for _, shareId := range req.ShareIds {
				if shareId == share.ShareId {
					exist = true
					break
				}
			}
			if !exist {
				continue
			}
		}
		result = append(result, &pan.ShareData{
			ShareId:  share.ShareId,
			ShareUrl: share.ShareUrl,
			PassCode: share.Passcode,
			Title:    share.Title,
		})
	}
	return result, nil
}
func (q *Quark) NewShare(req pan.NewShareReq) (*pan.ShareData, error) {
	urlType := 1
	if req.NeedPassCode {
		urlType = 2
	}
	shareId, err := q.share(ShareReq{
		FidList:     req.Fids,
		Title:       req.Title,
		UrlType:     urlType,
		ExpiredType: req.ExpiredType,
	})
	if err != nil {
		return nil, err
	}
	resp, err := q.sharePassword(shareId)
	if err != nil {
		return nil, err
	}
	return &pan.ShareData{
		ShareId:  shareId,
		ShareUrl: resp.Data.ShareUrl,
		PassCode: resp.Data.Passcode,
		Title:    resp.Data.Title,
	}, nil
}
func (q *Quark) DeleteShare(req pan.DelShareReq) error {
	_, err := q.shareDelete(req.ShareIds)
	return err
}

func (q *Quark) ShareRestore(req pan.ShareRestoreReq) error {
	if req.ShareUrl == "" {
		return pan.OnlyMsg("share url must not null")
	}
	// 解析URL
	parsedURL, err := url.Parse(req.ShareUrl)
	if err != nil {
		return err
	}
	pwdId := strings.TrimLeft(parsedURL.Path, "/s/")
	targetDir, err := q.Mkdir(pan.MkdirReq{
		NewPath: req.TargetDir,
	})
	if err != nil {
		return err
	}
	token, err := q.shareToken(ShareTokenReq{
		PwdId:    pwdId,
		Passcode: req.PassCode,
	})
	if err != nil {
		return err
	}
	stoken := token.Data.Stoken
	detail, err := q.shareDetail(ShareDetailReq{
		PwdId:  pwdId,
		Stoken: stoken,
	})
	if err != nil {
		return err
	}
	fidList := make([]string, 0)
	fidTokenList := make([]string, 0)
	for _, file := range detail.List {
		fidList = append(fidList, file.Fid)
		fidTokenList = append(fidTokenList, file.ShareFidToken)
	}
	err = q.shareRestore(RestoreReq{
		FidList:      fidList,
		FidTokenList: fidTokenList,
		ToPdirFid:    targetDir.Id,
		PwdId:        pwdId,
		Stoken:       stoken,
		PdirFid:      targetDir.Id,
		Scene:        "link",
	})
	return err
}

func (q *Quark) DirectLink(req pan.DirectLinkReq) ([]*pan.DirectLink, error) {
	return nil, pan.OnlyMsg("direct link not support")
}

func (q *Quark) ProxyFile(req pan.ProxyFileReq) (*pan.ProxyFileResp, error) {
	downloadUrl := func(dlReq pan.DownloadFileReq) (string, error) {
		resp, err := q.fileDownload(dlReq.RemoteFile.Id)
		if err != nil {
			return "", err
		}
		if len(resp.Data) == 0 {
			return "", pan.OnlyMsg("no download data")
		}
		return resp.Data[0].DownloadUrl, nil
	}

	doRequest := func(httpReq *http.Request) (*http.Response, error) {
		return q.sessionClient.GetClient().Do(httpReq)
	}

	return q.BaseProxyFile(req, downloadUrl, doRequest)
}

func init() {
	pan.RegisterDriver(pan.Quark, func() pan.Driver {
		return &Quark{
			PropertiesOperate: pan.PropertiesOperate[*QuarkProperties]{
				DriverType: pan.Quark,
			},
			CacheOperate:  pan.NewCacheOperate(),
			CommonOperate: pan.CommonOperate{},
		}
	})
}
