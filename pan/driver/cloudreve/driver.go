package cloudreve

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/hefy27/pan-client/internal"
	"github.com/hefy27/pan-client/pan"
	"github.com/imroc/req/v3"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Cloudreve struct {
	sessionClient *req.Client
	defaultClient *req.Client
	pan.PropertiesOperate[*CloudreveProperties]
	pan.CacheOperate
	pan.CommonOperate
	pan.BaseOperate
}

type CloudreveProperties struct {
	Id           string            `mapstructure:"id" json:"id" yaml:"id"`
	Url          string            `mapstructure:"url" json:"url" yaml:"url"`
	Type         string            `mapstructure:"type" json:"type" yaml:"type" default:"now61"`
	Session      string            `mapstructure:"session" json:"session" yaml:"session"`
	RefreshTime  int64             `mapstructure:"refresh_time" json:"refresh_time" yaml:"refresh_time" default:"0"`
	ChunkSize    int64             `mapstructure:"chunk_size" json:"chunk_size" yaml:"chunk_size" default:"104857600"` // 100M
	SkipVerify   bool              `mapstructure:"skip_verify" json:"skip_verify" yaml:"skip_verify" default:"false"`  // 100M
	OtherCookies map[string]string `mapstructure:"other_cookies" json:"other_cookies" yaml:"other_cookies"`
}

func (cp *CloudreveProperties) OnlyImportProperties() {
	// do nothing
}

func (cp *CloudreveProperties) GetId() string {
	if cp.Id == "" {
		cp.Id = uuid.NewString()
	}
	return cp.Id
}

func (cp *CloudreveProperties) GetDriverType() pan.DriverType {
	return pan.Cloudreve
}

func (c *Cloudreve) Init() (string, error) {
	driverId := c.GetId()
	if c.Properties.Url == "" || c.Properties.Session == "" {
		return driverId, fmt.Errorf("please set cloudreve url and session")
	}
	c.sessionClient = req.C().SetCommonHeader(HeaderUserAgent, DefaultUserAgent).
		SetCommonHeader("Accept", "application/json, text/plain, */*").
		SetTimeout(30 * time.Minute).SetBaseURL(c.Properties.Url + "/api/v3").
		SetCommonCookies(&http.Cookie{Name: CookieSessionKey, Value: c.Properties.Session})
	if c.Properties.SkipVerify {
		c.sessionClient.EnableInsecureSkipVerify()
	}
	if len(c.Properties.OtherCookies) > 0 {
		for k, v := range c.Properties.OtherCookies {
			internal.GetLogger().Info("set cookie", "name", k, "value", v)
			c.sessionClient.SetCommonCookies(&http.Cookie{Name: k, Value: v})
		}
	}
	c.defaultClient = req.C().SetCommonHeader(HeaderUserAgent, DefaultUserAgent).SetTimeout(2 * time.Hour)
	c.defaultClient.GetTransport().
		WrapRoundTripFunc(func(rt http.RoundTripper) req.HttpRoundTripFunc {
			return func(req *http.Request) (resp *http.Response, err error) {
				// 由于内容长度部分是由后台计算的，所以这里需要手动设置,http默认会过滤掉header.reqWriteExcludeHeader
				if req.ContentLength <= 0 {
					if req.Header.Get("Content-Length") != "" {
						req.ContentLength, _ = strconv.ParseInt(req.Header.Get("Content-Length"), 10, 64)
					}
				}
				return rt.RoundTrip(req)
			}
		})
	// 应用代理配置
	if c.ProxyConfig.ProxyURL != "" {
		c.sessionClient.SetProxyURL(c.ProxyConfig.ProxyURL)
		c.defaultClient.SetProxyURL(c.ProxyConfig.ProxyURL)
	}
	// 若一小时内更新过，则不重新刷session
	if c.Properties.RefreshTime == 0 || time.Now().UnixMilli()-c.Properties.RefreshTime > 60*60*1000 {
		_, configErr := c.config()
		if configErr != nil {
			return driverId, configErr
		}
		c.Properties.RefreshTime = time.Now().UnixMilli()
		c.NotifyChange()
	}
	return driverId, nil
}

func (c *Cloudreve) Close() error {
	c.Cancel()
	c.StopCache()
	return nil
}

func (c *Cloudreve) Disk() (*pan.DiskResp, error) {
	storageResp, err := c.userStorage()
	if err != nil {
		return nil, err
	}
	return &pan.DiskResp{
		Total: int64(storageResp.Data.Total / 1024 / 1024),
		Free:  int64(storageResp.Data.Free / 1024 / 1024),
		Used:  int64(storageResp.Data.Used / 1024 / 1024),
	}, nil
}
func (c *Cloudreve) List(req pan.ListReq) ([]*pan.PanObj, error) {
	if req.Dir.Path == "/" && req.Dir.Name == "" {
		req.Dir.Id = "0"
	}
	cacheKey := cacheDirectoryPrefix + req.Dir.Id
	if req.Reload {
		c.Del(cacheKey)
	}
	result, err := c.GetOrLoad(cacheKey, func() (interface{}, error) {
		directory, e := c.listDirectory(strings.TrimRight(req.Dir.Path, "/") + "/" + req.Dir.Name)
		if e != nil {
			internal.GetLogger().Error("list directory error", "error", e)
			return nil, e
		}
		panObjs := make([]*pan.PanObj, 0)
		for _, item := range directory.Data.Objects {
			panObjs = append(panObjs, &pan.PanObj{
				Id:     item.ID,
				Name:   item.Name,
				Path:   item.Path,
				Size:   int64(item.Size),
				Type:   item.Type,
				Parent: req.Dir,
			})
		}
		c.Set(cachePolicy, directory.Data.Policy)
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
func (c *Cloudreve) ObjRename(req pan.ObjRenameReq) error {
	if req.Obj.Id == "0" || (req.Obj.Path == "/" && req.Obj.Name == "") {
		return pan.OnlyMsg("not support rename root path")
	}
	object := req.Obj
	if object.Id == "" {
		path := strings.Trim(req.Obj.Path, "/") + "/" + req.Obj.Name
		obj, err := c.GetPanObj(path, true, c.List)
		if err != nil {
			return err
		}
		object = obj
	}
	item := Item{}
	if object.Type == "dir" {
		item.Dirs = []string{object.Id}
	} else {
		item.Items = []string{object.Id}
	}
	_, err := c.objectRename(ItemRenameReq{Src: item,
		NewName: req.NewName})
	if err != nil {
		return err
	}
	c.Del(cacheDirectoryPrefix + object.Parent.Id)
	return nil
}
func (c *Cloudreve) BatchRename(req pan.BatchRenameReq) error {
	return c.BaseBatchRename(req, c.List, c.ObjRename, c.BatchRename)
}
func (c *Cloudreve) Mkdir(req pan.MkdirReq) (*pan.PanObj, error) {
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
	obj, err := c.GetPanObj(targetPath, false, c.List)
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

		_, err = c.createDirectory(existPath + "/" + split[0])
		if err != nil {
			return nil, pan.OnlyError(err)
		}
		c.Del(cacheDirectoryPrefix + obj.Id)
		return c.Mkdir(req)
	}
}
func (c *Cloudreve) Move(req pan.MovieReq) error {
	sameSrc := make(map[string][]*pan.PanObj)
	for _, item := range req.Items {
		objs, ok := sameSrc[item.Path]
		if ok {
			objs = append(objs, item)
			sameSrc[item.Path] = objs
		} else {
			sameSrc[item.Path] = []*pan.PanObj{item}
		}
	}
	targetObj := req.TargetObj
	if targetObj.Type == "file" {
		return pan.OnlyMsg("target is a file")
	}
	// 重新直接创建目标目录
	if targetObj.Id == "" {
		create, err := c.Mkdir(pan.MkdirReq{
			NewPath: strings.Trim(targetObj.Path, "/") + "/" + targetObj.Name,
		})
		if err != nil {
			return err
		}
		targetObj = create
	}
	for src, items := range sameSrc {
		reloadDirId := make(map[string]any)
		itemIds := make([]string, 0)
		dirIds := make([]string, 0)
		for _, item := range items {
			if item.Id != "0" && item.Id != "" {
				if item.Type == "dir" {
					dirIds = append(dirIds, item.Id)
					reloadDirId[item.Id] = true
				} else {
					itemIds = append(itemIds, item.Id)
				}
			} else if item.Path != "" && item.Path != "/" {
				obj, err := c.GetPanObj(strings.Trim(item.Path, "/")+"/"+item.Name, true, c.List)
				if err == nil {
					if obj.Type == "dir" {
						dirIds = append(dirIds, obj.Id)
						reloadDirId[obj.Id] = true
					} else {
						itemIds = append(itemIds, obj.Id)
						reloadDirId[obj.Parent.Id] = true
					}
				}
			}
		}
		_, err := c.objectMove(ItemMoveReq{
			SrcDir: src,
			Dst:    strings.Trim(targetObj.Path, "/") + "/" + targetObj.Name,
			Src: Item{
				Items: itemIds,
				Dirs:  dirIds,
			},
		})
		if err != nil {
			return pan.OnlyError(err)
		}
		reloadDirId[targetObj.Id] = true
		for key, _ := range reloadDirId {
			c.Del(cacheDirectoryPrefix + key)
		}
	}
	return nil
}
func (c *Cloudreve) Delete(req pan.DeleteReq) error {
	if len(req.Items) == 0 {
		return nil
	}
	reloadDirId := make(map[string]any)
	itemIds := make([]string, 0)
	dirIds := make([]string, 0)
	for _, item := range req.Items {
		if item.Id != "0" && item.Id != "" {
			if item.Type == "dir" {
				dirIds = append(dirIds, item.Id)
				reloadDirId[item.Id] = true
			} else {
				itemIds = append(itemIds, item.Id)
				if item.Parent.Id != "" {
					reloadDirId[item.Parent.Id] = true
				}
			}
		} else if item.Path != "" && item.Path != "/" {
			obj, err := c.GetPanObj(item.Path, true, c.List)
			if err == nil {
				if obj.Type == "dir" {
					dirIds = append(dirIds, obj.Id)
					reloadDirId[obj.Id] = true
				} else {
					itemIds = append(itemIds, obj.Id)
					reloadDirId[obj.Parent.Id] = true
				}
			}
		}
	}
	if len(itemIds) > 0 || len(dirIds) > 0 {
		_, err := c.objectDelete(ItemReq{
			Item: Item{
				Items: itemIds,
				Dirs:  dirIds,
			},
			Force:      true,
			UnlinkOnly: false,
		})
		if err != nil {
			return err
		}
		for key, _ := range reloadDirId {
			c.Del(cacheDirectoryPrefix + key)
		}
	}

	return nil
}

func (c *Cloudreve) UploadPath(req pan.UploadPathReq) (*pan.TransferResult, error) {
	if req.OnlyFast {
		return nil, pan.OnlyMsg("cloudreve is not support fast upload")
	}
	err := c.BaseUploadPath(req, c.UploadFile)
	return nil, err
}

func (c *Cloudreve) uploadErrAfter(md5Key string, uploadedSize int64, session UploadCredential) {
	c.Set(cacheChunkPrefix+md5Key, uploadedSize)
	errorTimesVal, err := c.GetOrLoad(cacheSessionErrPrefix+md5Key, func() (interface{}, error) {
		return 0, nil
	})
	if err != nil {
		errorTimesVal = 0
	}
	i := errorTimesVal.(int)
	if i > 3 {
		if session.SessionID != "" {
			_, _ = c.fileUploadDeleteUploadSession(session.SessionID)
		} else {
			_, _ = c.fileUploadDeleteAllUploadSession()
		}
		c.Del(cacheSessionPrefix + md5Key)
		c.Del(cacheChunkPrefix + md5Key)
		c.Del(cacheSessionErrPrefix + md5Key)
	}
	c.Set(cacheSessionErrPrefix+md5Key, i+1)
}

func (c *Cloudreve) UploadFile(req pan.UploadFileReq) (*pan.TransferResult, error) {
	if req.OnlyFast {
		return nil, pan.OnlyMsg("cloudreve is not support fast upload")
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
	_, err = c.GetPanObj(remoteAllPath, true, c.List)
	// 没有报错证明文件已经存在
	if err == nil {
		return nil, pan.CodeMsg(CodeObjectExist, remoteAllPath+" is exist")
	}
	_, err = c.Mkdir(pan.MkdirReq{
		NewPath: remotePath,
	})
	if err != nil {
		return nil, pan.MsgError(remotePath+" create error", err)
	}
	md5Key := internal.Md5HashStr(remoteAllPath)
	if !req.Resumable {
		c.Del(cacheSessionPrefix + md5Key)
		c.Del(cacheChunkPrefix + md5Key)
		c.Del(cacheSessionErrPrefix + md5Key)
	}
	var uploadedSize int64 = 0
	if obj, exist := c.Get(cacheChunkPrefix + md5Key); exist {
		uploadedSize = obj.(int64)
	}
	var session UploadCredential
	data, e := c.GetOrLoad(cacheSessionPrefix+md5Key, func() (interface{}, error) {
		policy, exist := c.Get(cachePolicy)
		if !exist {
			return nil, pan.OnlyMsg(cachePolicy + " is not exist")
		}
		summary := policy.(*PolicySummary)
		resp, e := c.fileUploadGetUploadSession(CreateUploadSessionReq{
			Path:         "/" + remotePath,
			Size:         uint64(stat.Size()),
			Name:         remoteName,
			PolicyID:     summary.ID,
			LastModified: stat.ModTime().UnixMilli(),
		})
		if e != nil {
			if e.GetCode() == CodeConflictUploadOngoing {
				_, _ = c.fileUploadDeleteAllUploadSession()
				sResp, secE := c.fileUploadGetUploadSession(CreateUploadSessionReq{
					Path:         "/" + remotePath,
					Size:         uint64(stat.Size()),
					Name:         remoteName,
					PolicyID:     summary.ID,
					LastModified: stat.ModTime().UnixMilli(),
				})
				if secE != nil {
					return nil, secE
				}
				return sResp.Data, nil
			}
			return nil, e
		}
		return resp.Data, nil
	})
	if e != nil {
		return nil, e
	}
	if data != nil {
		session = data.(UploadCredential)
	}

	// 使用 SessionID 作为文件任务 ID（Cloudreve 无预创建文件 ID）
	fileTaskId := session.SessionID
	result := &pan.TransferResult{TaskId: fileTaskId}

	switch c.Properties.Type {
	case Now61, Yiandrive, Wuaipan:
		uploadedSize, err = c.notKnowUpload(NotKnowUploadReq{
			UploadUrl:        session.UploadURLs[0],
			Credential:       session.Credential,
			LocalFile:        req.LocalFile,
			UploadedSize:     uploadedSize,
			ChunkSize:        int64(session.ChunkSize),
			TaskId:           req.TaskId,
			FileId:           fileTaskId,
			Ctx:              req.Ctx,
			ProgressCallback: req.ProgressCallback,
		})
		if err != nil {
			c.uploadErrAfter(md5Key, uploadedSize, session)
			return result, err
		}
	case Huang1111, Hefamily, Hucl:
		uploadedSize, err = c.oneDriveUpload(OneDriveUploadReq{
			UploadUrl:        session.UploadURLs[0],
			LocalFile:        req.LocalFile,
			UploadedSize:     uploadedSize,
			ChunkSize:        min(int64(session.ChunkSize), c.Properties.ChunkSize),
			TaskId:           req.TaskId,
			FileId:           fileTaskId,
			Ctx:              req.Ctx,
			ProgressCallback: req.ProgressCallback,
		})
		if err != nil {
			c.uploadErrAfter(md5Key, uploadedSize, session)
			return result, err
		}

		_, err = c.oneDriveCallback(session.SessionID)
		if err != nil {
			c.uploadErrAfter(md5Key, uploadedSize, session)
			return result, err
		}
	default:
		return result, pan.OnlyMsg("not support Type")
	}

	if req.Resumable {
		c.Del(cacheSessionPrefix + md5Key)
		c.Del(cacheChunkPrefix + md5Key)
		c.Del(cacheSessionErrPrefix + md5Key)
	}
	internal.GetLogger().Info("upload success", "file", req.LocalFile, "sessionId", fileTaskId)
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

func (c *Cloudreve) DownloadPath(req pan.DownloadPathReq) (*pan.TransferResult, error) {
	err := c.BaseDownloadPath(req, c.List, c.DownloadFile)
	return nil, err
}
func (c *Cloudreve) DownloadFile(req pan.DownloadFileReq) (*pan.TransferResult, error) {
	err := c.BaseDownloadFile(req, c.defaultClient, func(req pan.DownloadFileReq) (string, error) {
		resp, err := c.fileCreateDownloadSession(req.RemoteFile.Id)
		if err != nil {
			return "", err
		}
		return resp.Data, nil
	})
	return nil, err
}

func (c *Cloudreve) OfflineDownload(req pan.OfflineDownloadReq) (*pan.Task, error) {
	return nil, pan.OnlyMsg("offline download not support")
}

func (c *Cloudreve) TaskList(req pan.TaskListReq) ([]*pan.Task, error) {
	return nil, pan.OnlyMsg("task list not support")
}

func (c *Cloudreve) ShareList(req pan.ShareListReq) ([]*pan.ShareData, error) {
	return nil, pan.OnlyMsg("share list not support")
}
func (c *Cloudreve) NewShare(req pan.NewShareReq) (*pan.ShareData, error) {
	return nil, pan.OnlyMsg("new share not support")
}
func (c *Cloudreve) DeleteShare(req pan.DelShareReq) error {
	return pan.OnlyMsg("delete share not support")
}
func (c *Cloudreve) ShareRestore(req pan.ShareRestoreReq) error {
	return pan.OnlyMsg("share restore not support ")
}

func (c *Cloudreve) ProxyFile(req pan.ProxyFileReq) (*pan.ProxyFileResp, error) {
	downloadUrl := func(dlReq pan.DownloadFileReq) (string, error) {
		resp, err := c.fileCreateDownloadSession(dlReq.RemoteFile.Id)
		if err != nil {
			return "", err
		}
		return resp.Data, nil
	}

	doRequest := func(httpReq *http.Request) (*http.Response, error) {
		return c.defaultClient.GetClient().Do(httpReq)
	}

	return c.BaseProxyFile(req, downloadUrl, doRequest)
}

func (c *Cloudreve) DirectLink(req pan.DirectLinkReq) ([]*pan.DirectLink, error) {
	fileList := req.List
	fids := make([]string, 0)
	for _, file := range fileList {
		fids = append(fids, file.FileId)
	}
	resp, err := c.fileGetSource(ItemReq{
		Item: Item{Items: fids},
	})
	if err != nil {
		return nil, err
	}
	sources := resp.Data
	nameUrlMap := make(map[string]string)
	for _, source := range sources {
		nameUrlMap[source.Name] = source.Url
	}
	for _, file := range fileList {
		url, ok := nameUrlMap[file.Name]
		if ok {
			file.Link = url
		}
	}
	return fileList, nil
}

func init() {
	pan.RegisterDriver(pan.Cloudreve, func() pan.Driver {
		return &Cloudreve{
			PropertiesOperate: pan.PropertiesOperate[*CloudreveProperties]{
				DriverType: pan.Cloudreve,
			},
			CacheOperate:  pan.NewCacheOperate(),
			CommonOperate: pan.CommonOperate{},
		}
	})
}
