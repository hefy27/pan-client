package thunder_browser

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/hefeiyu25/pan-client/internal"
	"github.com/hefeiyu25/pan-client/pan"
	"github.com/imroc/req/v3"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ThunderBrowser struct {
	sessionClient  *req.Client
	downloadClient *req.Client
	pan.PropertiesOperate[*ThunderBrowserProperties]
	pan.CacheOperate
	pan.CommonOperate
	pan.BaseOperate
}

type ThunderBrowserProperties struct {
	Id string `mapstructure:"id" json:"id" yaml:"id"`
	// 登录方式1
	Username string `mapstructure:"username" json:"username" yaml:"username"`
	Password string `mapstructure:"password" json:"password" yaml:"password"`
	// 登录方式2
	RefreshToken string `mapstructure:"refresh_token" json:"refresh_token" yaml:"refresh_token"`

	// 验证码
	CaptchaToken string `mapstructure:"captcha_token" json:"captcha_token" yaml:"captcha_token"`

	DeviceID string `mapstructure:"device_id" json:"device_id" yaml:"device_id"`

	ExpiresIn int64 `mapstructure:"expires_in" json:"expires_in" yaml:"expires_in"`

	TokenType   string `mapstructure:"token_type" json:"token_type" yaml:"token_type"`
	AccessToken string `mapstructure:"access_token" json:"access_token" yaml:"access_token"`

	Sub    string `mapstructure:"sub" json:"sub" yaml:"sub"`
	UserID string `mapstructure:"user_id" json:"user_id" yaml:"user_id"`

	UseVideoUrl bool `mapstructure:"use_video_url" json:"use_video_url" yaml:"use_video_url"`
}

func (cp *ThunderBrowserProperties) OnlyImportProperties() {
	// do nothing
}
func (cp *ThunderBrowserProperties) GetId() string {
	if cp.Id == "" {
		cp.Id = uuid.NewString()
	}
	return cp.Id
}

func (cp *ThunderBrowserProperties) GetDriverType() pan.DriverType {
	return pan.ThunderBrowser
}
func (tb *ThunderBrowser) Init() (string, error) {
	driverId := tb.GetId()
	if (tb.Properties.Username == "" || tb.Properties.Password == "") && tb.Properties.RefreshToken == "" {
		return driverId, fmt.Errorf("please set login info ")
	}
	tb.Properties.DeviceID = internal.Md5HashStr(tb.Properties.Username + tb.Properties.Password)
	commonHeaderMap := map[string]string{
		HeaderUserAgent:    BuildCustomUserAgent(PackageName, SdkVersion, ClientVersion),
		"accept":           "application/json;charset=UTF-8",
		"x-device-id":      tb.Properties.DeviceID,
		"x-client-id":      ClientID,
		"x-client-version": ClientVersion,
	}
	tb.sessionClient = req.C().SetCommonHeaders(commonHeaderMap)

	_, userErr := tb.userMe()
	// 若能拿到用户信息，证明已经登录
	if userErr != nil {

		// refreshToken不为空，则先用token登录
		if tb.Properties.RefreshToken != "" {
			tb.Properties.DeviceID = internal.Md5HashStr(tb.Properties.RefreshToken)
			_, loginErr := tb.refreshToken(tb.Properties.RefreshToken)
			if loginErr != nil {
				_, loginErr = tb.login(tb.Properties.Username, tb.Properties.Password)
				if loginErr != nil {
					return driverId, loginErr
				}
			}
		} else {
			_, loginErr := tb.login(tb.Properties.Username, tb.Properties.Password)
			if loginErr != nil {
				return driverId, loginErr
			}
		}
	}

	tb.downloadClient = req.C().SetCommonHeader(HeaderUserAgent, DownloadUserAgent)
	// 应用代理配置
	if tb.ProxyConfig.ProxyURL != "" {
		tb.sessionClient.SetProxyURL(tb.ProxyConfig.ProxyURL)
		tb.downloadClient.SetProxyURL(tb.ProxyConfig.ProxyURL)
	}

	tb.NotifyChange()
	return driverId, nil
}

func (tb *ThunderBrowser) Close() error {
	tb.Cancel()
	tb.StopCache()
	return nil
}

func (tb *ThunderBrowser) Disk() (*pan.DiskResp, error) {
	about, err := tb.about()
	if err != nil {
		return nil, err
	}
	total, _ := strconv.ParseInt(about.Quota.Limit, 10, 64)
	usage, _ := strconv.ParseInt(about.Quota.Usage, 10, 64)
	return &pan.DiskResp{
		Total: total / 1024 / 1024,
		Free:  (total - usage) / 1024 / 1024,
		Used:  usage / 1024 / 1024,
		Ext: map[string]interface{}{
			QuotaCreateOfflineTaskLimit: about.Quotas[QuotaCreateOfflineTaskLimit],
		},
	}, nil
}
func (tb *ThunderBrowser) List(req pan.ListReq) ([]*pan.PanObj, error) {
	queryDir := req.Dir
	if queryDir.Path == "/" && queryDir.Name == "" {
		queryDir.Id = "0"
	}
	if queryDir.Id == "" {
		obj, err := tb.GetPanObj(strings.TrimRight(queryDir.Path, "/")+"/"+queryDir.Name, true, tb.List)
		if err != nil {
			return nil, err
		}
		queryDir = obj
	}
	cacheKey := cacheDirectoryPrefix + queryDir.Id
	if req.Reload {
		tb.Del(cacheKey)
	}
	result, err := tb.GetOrLoad(cacheKey, func() (interface{}, error) {
		files, e := tb.getFiles(queryDir.Id)
		if e != nil {
			internal.GetLogger().Error("get files error", "error", e)
			return nil, e
		}
		panObjs := make([]*pan.PanObj, 0)
		for _, item := range files {
			fileType := "file"
			if item.Kind == "drive#folder" {
				fileType = "dir"
			}
			path := strings.TrimRight(queryDir.Path, "/") + "/" + queryDir.Name
			if queryDir.Id == "" {
				path = "/"
			}
			size, _ := strconv.ParseInt(item.Size, 10, 64)
			panObjs = append(panObjs, &pan.PanObj{
				Id:     item.ID,
				Name:   item.Name,
				Path:   path,
				Size:   size,
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
func (tb *ThunderBrowser) ObjRename(req pan.ObjRenameReq) error {
	if req.Obj.Id == "0" || (req.Obj.Path == "/" && req.Obj.Name == "") {
		return pan.OnlyMsg("not support rename root path")
	}
	object := req.Obj
	if object.Id == "" {
		path := strings.Trim(req.Obj.Path, "/") + "/" + req.Obj.Name
		obj, err := tb.GetPanObj(path, true, tb.List)
		if err != nil {
			return err
		}
		object = obj
	}
	newFile, err := tb.rename(object.Id, req.NewName)
	if err != nil {
		return err
	}
	tb.Del(cacheDirectoryPrefix + newFile.ParentID)
	return nil
}
func (tb *ThunderBrowser) BatchRename(req pan.BatchRenameReq) error {
	return tb.BaseBatchRename(req, tb.List, tb.ObjRename, tb.BatchRename)
}
func (tb *ThunderBrowser) Mkdir(req pan.MkdirReq) (*pan.PanObj, error) {
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
	obj, err := tb.GetPanObj(targetPath, false, tb.List)
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
		for _, s := range split {
			resp, err := tb.makeDir(s, targetDirId)
			if err != nil {
				return nil, pan.OnlyError(err)
			}
			// 这里有问题
			targetDirId = resp.File.ID
		}
		tb.Del(cacheDirectoryPrefix + obj.Id)
		return tb.Mkdir(req)
	}
}
func (tb *ThunderBrowser) Move(req pan.MovieReq) error {
	targetObj := req.TargetObj
	if targetObj.Type == "file" {
		return pan.OnlyMsg("target is a file")
	}
	// 重新直接创建目标目录
	if targetObj.Id == "" {
		create, err := tb.Mkdir(pan.MkdirReq{
			NewPath: strings.Trim(targetObj.Path, "/") + "/" + targetObj.Name,
		})
		if err != nil {
			return err
		}
		targetObj = create
	}
	collected := pan.CollectItemIds(req.Items, tb.GetPanObj, tb.List, false)
	err := tb.move(collected.ObjIds, targetObj.Id)
	if err != nil {
		return pan.OnlyError(err)
	}
	for key := range collected.ReloadDirIds {
		tb.Del(cacheDirectoryPrefix + key)
	}
	return nil
}
func (tb *ThunderBrowser) Delete(req pan.DeleteReq) error {
	if len(req.Items) == 0 {
		return nil
	}
	collected := pan.CollectItemIds(req.Items, tb.GetPanObj, tb.List, true)
	if len(collected.ObjIds) > 0 {
		err := tb.remove(collected.ObjIds)
		if err != nil {
			return err
		}
		for key := range collected.ReloadDirIds {
			tb.Del(cacheDirectoryPrefix + key)
		}
	}
	return nil
}

func (tb *ThunderBrowser) UploadPath(req pan.UploadPathReq) (*pan.TransferResult, error) {
	err := tb.BaseUploadPath(req, tb.UploadFile)
	return nil, err
}

func (tb *ThunderBrowser) UploadFile(req pan.UploadFileReq) (*pan.TransferResult, error) {
	if req.Resumable {
		internal.GetLogger().Warn("thunder_browser is not support resumeable")
	}
	if req.OnlyFast {
		return nil, pan.OnlyMsg("thunder_browser is not support fast upload")
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
	_, err = tb.GetPanObj(remoteAllPath, true, tb.List)
	// 没有报错证明文件已经存在
	if err == nil {
		return nil, pan.CodeMsg(CodeObjectExist, remoteAllPath+" is exist")
	}
	dir, err := tb.Mkdir(pan.MkdirReq{
		NewPath: remotePath,
	})
	if err != nil {
		return nil, pan.MsgError(remotePath+" create error", err)
	}

	gcid, err := internal.GetFileGcid(req.LocalFile)
	if err != nil {
		return nil, err
	}
	parentId := dir.Id
	if parentId == "0" {
		parentId = ""
	}
	resp, err := tb.uploadTask(UploadTaskRequest{
		Kind:       FILE,
		ParentId:   parentId,
		Name:       remoteName,
		Size:       stat.Size(),
		Hash:       gcid,
		UploadType: UploadTypeResumable,
		Space:      ThunderDriveSpace,
	})

	if err != nil {
		return nil, err
	}

	// 使用网盘返回的文件 ID
	fileTaskId := resp.File.ID
	result := &pan.TransferResult{TaskId: fileTaskId}

	param := resp.Resumable.Params
	if resp.UploadType == UploadTypeResumable {
		param.Endpoint = strings.TrimLeft(param.Endpoint, param.Bucket+".")
		s3Client := s3.New(s3.Options{
			Credentials:  credentials.NewStaticCredentialsProvider(param.AccessKeyID, param.AccessKeySecret, param.SecurityToken),
			Region:       "xunlei",
			BaseEndpoint: aws.String(param.Endpoint),
		})
		uploader := manager.NewUploader(s3Client)
		if stat.Size() > int64(manager.MaxUploadParts)*manager.DefaultUploadPartSize {
			uploader.PartSize = stat.Size() / (int64(manager.MaxUploadParts) - 1)
		}
		file, err := os.Open(req.LocalFile)
		if err != nil {
			return result, err
		}
		uploadCtx := context.Background()
		if req.Ctx != nil {
			uploadCtx = req.Ctx
		}
		pw := pan.NewProgressWriter(req.LocalFile, stat.Size(), req.ProgressCallback)
		if req.TaskId != "" {
			pw.SetTaskId(req.TaskId)
		}
		pw.SetFileId(fileTaskId)
		_, err = uploader.Upload(uploadCtx, &s3.PutObjectInput{
			Bucket:  aws.String(param.Bucket),
			Key:     aws.String(param.Key),
			Expires: aws.Time(param.Expiration),
			Body:    io.TeeReader(file, pw),
		})
		_ = file.Close()
		if err == nil && req.SuccessDel {
			err = os.Remove(req.LocalFile)
			if err != nil {
				internal.GetLogger().Error("delete fail", "file", req.LocalFile, "error", err)
			} else {
				internal.GetLogger().Info("delete success", "file", req.LocalFile)
			}
		}
		return result, err
	}

	return result, nil
}

func (tb *ThunderBrowser) DownloadPath(req pan.DownloadPathReq) (*pan.TransferResult, error) {
	err := tb.BaseDownloadPath(req, tb.List, tb.DownloadFile)
	return nil, err
}
// resolveDownloadLink extracts the best download URL from a Files object.
// When UseVideoUrl is true, transcoded media links are preferred over the raw download link.
func (tb *ThunderBrowser) resolveDownloadLink(link *Files, fileName string) (string, error) {
	downloadLink := ""
	if tb.Properties.UseVideoUrl {
		for _, media := range link.Medias {
			if media.Link.URL != "" {
				downloadLink = media.Link.URL
				break
			}
		}
	}
	if downloadLink == "" {
		downloadLink = link.WebContentLink
	}
	if downloadLink == "" {
		for _, media := range link.Medias {
			if media.Link.URL != "" {
				downloadLink = media.Link.URL
				break
			}
		}
	}
	if downloadLink == "" {
		internal.GetLogger().Debug("cant get link", "name", fileName, "link", link)
		return "", pan.OnlyMsg(fmt.Sprintf("cant get link:%s", fileName))
	}
	return downloadLink, nil
}

func (tb *ThunderBrowser) DownloadFile(req pan.DownloadFileReq) (*pan.TransferResult, error) {
	err := tb.BaseDownloadFile(req, tb.downloadClient, func(req pan.DownloadFileReq) (string, error) {
		link, err := tb.getLink(req.RemoteFile.Id)
		if err != nil {
			return "", err
		}
		return tb.resolveDownloadLink(link, req.RemoteFile.Name)
	})
	return nil, err
}

func (tb *ThunderBrowser) OfflineDownload(req pan.OfflineDownloadReq) (*pan.Task, error) {
	dir, err := tb.Mkdir(pan.MkdirReq{
		NewPath: req.RemotePath,
	})
	if err != nil {
		return nil, pan.MsgError(req.RemotePath+" create error", err)
	}
	parentId := dir.Id
	if parentId == "0" {
		parentId = ""
	}
	remoteName := req.RemoteName
	if remoteName == "" {
		remoteName = req.Url
	}
	taskResp, e := tb.uploadTask(UploadTaskRequest{
		Kind:       FILE,
		ParentId:   parentId,
		Name:       remoteName,
		UploadType: UploadTypeUrl,
		Space:      ThunderDriveSpace,
		Url: Url{
			Url:   req.Url,
			Files: []string{},
		},
	})

	if e != nil {
		return nil, e
	}
	task := taskResp.Task
	return &pan.Task{
		Id:          task.Id,
		Name:        task.Name,
		Type:        task.Type,
		Phase:       task.Phase,
		CreatedTime: task.CreatedTime.Time,
		UpdatedTime: task.UpdatedTime.Time,
	}, nil
}

func (tb *ThunderBrowser) TaskList(req pan.TaskListReq) ([]*pan.Task, error) {
	tasks, err := tb.taskQuery(TaskQueryRequest{
		Space:  ThunderDriveSpace,
		Types:  req.Types,
		Ids:    req.Ids,
		Phases: req.Phases,
		With:   "reference_resource",
		Limit:  100,
	})
	if err != nil {
		return nil, err
	}
	panTasks := make([]*pan.Task, 0)
	for _, task := range tasks {
		panTasks = append(panTasks, &pan.Task{
			Id:          task.Id,
			Name:        task.Name,
			Type:        task.Type,
			Phase:       task.Phase,
			CreatedTime: task.CreatedTime.Time,
			UpdatedTime: task.UpdatedTime.Time,
		})
	}

	return panTasks, nil
}

func (tb *ThunderBrowser) ShareList(req pan.ShareListReq) ([]*pan.ShareData, error) {
	shareList, err := tb.shareList(req.ShareIds...)
	if err != nil {
		return nil, err
	}
	result := make([]*pan.ShareData, 0)
	for _, share := range shareList {
		result = append(result, &pan.ShareData{
			ShareId:  share.ShareId,
			ShareUrl: share.ShareUrl,
			PassCode: share.PassCode,
			Title:    share.Title,
		})
	}
	return result, nil
}
func (tb *ThunderBrowser) NewShare(req pan.NewShareReq) (*pan.ShareData, error) {
	share, err := tb.createShare(CreateShareReq{
		FileIds: req.Fids,
		ShareTo: "copy",
		Params: CreateShareParams{
			SubscribePush:      "false",
			WithPassCodeInLink: strconv.FormatBool(req.NeedPassCode),
		},
		Title:          req.Title,
		RestoreLimit:   "-1",
		ExpirationDays: strconv.Itoa(req.ExpiredType),
	})
	if err != nil {
		return nil, err
	}
	return &pan.ShareData{
		ShareId:  share.ShareId,
		ShareUrl: share.ShareUrl,
		PassCode: share.PassCode,
	}, nil
}
func (tb *ThunderBrowser) DeleteShare(req pan.DelShareReq) error {
	shareIds := req.ShareIds
	for _, shareId := range shareIds {
		err := tb.deleteShare(shareId)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tb *ThunderBrowser) ShareRestore(req pan.ShareRestoreReq) error {
	passCode := req.PassCode
	shareId := req.ShareId
	targetDir := req.TargetDir
	if targetDir == "" {
		targetDir = "/"
	}
	if req.ShareId == "" {
		if req.ShareUrl == "" {
			return pan.OnlyMsg("share url is null")
		}
		// 解析URL
		parsedURL, err := url.Parse(req.ShareUrl)
		if err != nil {
			return err
		}

		// 获取查询参数
		queryParams := parsedURL.Query()
		shareId = strings.TrimLeft(parsedURL.Path, "/s/")
		// 从查询参数中提取分享ID和密码
		passCode = queryParams.Get("pwd")
	}
	parentDir, err := tb.Mkdir(pan.MkdirReq{
		NewPath: targetDir,
	})
	if err != nil {
		return err
	}
	share, err := tb.getShare(ShareDetailReq{
		ShareId:  shareId,
		PassCode: passCode,
	})
	if err != nil {
		return err
	}
	fileIds := make([]string, 0)
	for _, file := range share.Files {
		fileIds = append(fileIds, file.ID)
	}
	restore, err := tb.restore(RestoreReq{
		ParentId:        parentDir.Id,
		ShareId:         shareId,
		PassCodeToken:   share.PassCodeToken,
		AncestorIds:     nil,
		FileIds:         fileIds,
		SpecifyParentId: true,
	})
	if err != nil {
		return err
	}
	for {
		info, err := tb.taskInfo(restore.RestoreTaskId)
		if err != nil {
			return err
		}
		if info.Phase == PhaseTypeComplete {
			break
		}
		time.Sleep(time.Second)
	}
	return nil
}

func (tb *ThunderBrowser) DirectLink(req pan.DirectLinkReq) ([]*pan.DirectLink, error) {
	return nil, pan.OnlyMsg("direct link not support")
}

func (tb *ThunderBrowser) ProxyFile(req pan.ProxyFileReq) (*pan.ProxyFileResp, error) {
	downloadUrl := func(dlReq pan.DownloadFileReq) (string, error) {
		link, err := tb.getLink(dlReq.RemoteFile.Id)
		if err != nil {
			return "", err
		}
		return tb.resolveDownloadLink(link, dlReq.RemoteFile.Name)
	}

	doRequest := func(httpReq *http.Request) (*http.Response, error) {
		return tb.downloadClient.GetClient().Do(httpReq)
	}

	return tb.BaseProxyFile(req, downloadUrl, doRequest)
}

func init() {
	pan.RegisterDriver(pan.ThunderBrowser, func() pan.Driver {
		return &ThunderBrowser{
			PropertiesOperate: pan.PropertiesOperate[*ThunderBrowserProperties]{
				DriverType: pan.ThunderBrowser,
			},
			CacheOperate:  pan.NewCacheOperate(),
			CommonOperate: pan.CommonOperate{},
		}
	})
}

func BuildCustomUserAgent(appName, sdkVersion, clientVersion string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("ANDROID-%s/%s ", appName, clientVersion))
	sb.WriteString("networkType/WIFI ")
	sb.WriteString(fmt.Sprintf("appid/%s ", "22062"))
	sb.WriteString(fmt.Sprintf("deviceName/Xiaomi_M2004j7ac "))
	sb.WriteString(fmt.Sprintf("deviceModel/M2004J7AC "))
	sb.WriteString(fmt.Sprintf("OSVersion/13 "))
	sb.WriteString(fmt.Sprintf("protocolVersion/301 "))
	sb.WriteString(fmt.Sprintf("platformversion/10 "))
	sb.WriteString(fmt.Sprintf("sdkVersion/%s ", sdkVersion))
	sb.WriteString(fmt.Sprintf("Oauth2Client/0.9 (Linux 4_9_337-perf-sn-uotan-gd9d488809c3d) (JAVA 0) "))
	return sb.String()
}
