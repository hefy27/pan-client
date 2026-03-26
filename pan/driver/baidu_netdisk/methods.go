package baidu_netdisk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hefy27/pan-client/internal"
	"github.com/hefy27/pan-client/pan"
	"github.com/imroc/req/v3"
)

func (d *BaiduNetdisk) refreshToken() error {
	if d.Properties.ClientId == "" || d.Properties.ClientSecret == "" {
		return fmt.Errorf("empty ClientId or ClientSecret")
	}
	r := d.sessionClient.R()
	var tokenResp TokenResp
	var tokenErr TokenErrResp
	r.SetSuccessResult(&tokenResp).SetErrorResult(&tokenErr)
	r.SetQueryParams(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": d.Properties.RefreshToken,
		"client_id":     d.Properties.ClientId,
		"client_secret": d.Properties.ClientSecret,
	})
	response, err := r.Get(TOKEN_URL)
	if err != nil {
		return err
	}
	if response.IsErrorState() || tokenErr.Error != "" {
		return fmt.Errorf("refresh token error: %s - %s", tokenErr.Error, tokenErr.ErrorDescription)
	}
	if tokenResp.RefreshToken == "" {
		return fmt.Errorf("empty refresh token returned, possibly wrong refresh token")
	}
	d.Properties.AccessToken = tokenResp.AccessToken
	d.Properties.RefreshToken = tokenResp.RefreshToken
	d.NotifyChange()
	return nil
}

func (d *BaiduNetdisk) request(furl string, method string, callback func(r *req.Request), resp interface{}) ([]byte, pan.DriverErrorInterface) {
	var lastErr pan.DriverErrorInterface
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		r := d.sessionClient.R()
		r.SetQueryParam("access_token", d.Properties.AccessToken)
		if callback != nil {
			callback(r)
		}
		if resp != nil {
			r.SetSuccessResult(resp)
		}
		var response *req.Response
		var err error
		switch method {
		case http.MethodGet:
			response, err = r.Get(furl)
		case http.MethodPost:
			response, err = r.Post(furl)
		default:
			response, err = r.Send(method, furl)
		}
		if err != nil {
			lastErr = pan.OnlyError(err)
			continue
		}
		body := response.Bytes()
		var errCheck struct {
			Errno int `json:"errno"`
		}
		_ = json.Unmarshal(body, &errCheck)
		if errCheck.Errno == 0 {
			return body, nil
		}
		if errCheck.Errno == 111 || errCheck.Errno == -6 {
			internal.GetLogger().Info("baidu token expired, refreshing")
			if refreshErr := d.refreshToken(); refreshErr != nil {
				return nil, pan.OnlyError(refreshErr)
			}
			lastErr = pan.OnlyMsg(fmt.Sprintf("errno: %d, retrying after token refresh", errCheck.Errno))
			continue
		}
		return nil, pan.OnlyMsg(fmt.Sprintf("baidu api errno: %d, url: %s", errCheck.Errno, furl))
	}
	return nil, lastErr
}

func (d *BaiduNetdisk) get(pathname string, params map[string]string, resp interface{}) ([]byte, pan.DriverErrorInterface) {
	return d.request(API_URL+pathname, http.MethodGet, func(r *req.Request) {
		r.SetQueryParams(params)
	}, resp)
}

func (d *BaiduNetdisk) postForm(pathname string, params map[string]string, form map[string]string, resp interface{}) ([]byte, pan.DriverErrorInterface) {
	return d.request(API_URL+pathname, http.MethodPost, func(r *req.Request) {
		r.SetQueryParams(params)
		r.SetFormData(form)
	}, resp)
}

func (d *BaiduNetdisk) getFiles(dir string) ([]BaiduFile, pan.DriverErrorInterface) {
	start := 0
	limit := 1000
	params := map[string]string{
		"method": "list",
		"dir":    dir,
		"web":    "web",
	}
	result := make([]BaiduFile, 0)
	for {
		params["start"] = strconv.Itoa(start)
		params["limit"] = strconv.Itoa(limit)
		var resp ListResp
		_, err := d.get("/xpan/file", params, &resp)
		if err != nil {
			return nil, err
		}
		if len(resp.List) == 0 {
			break
		}
		result = append(result, resp.List...)
		if len(resp.List) < limit {
			break
		}
		start += limit
	}
	return result, nil
}

func (d *BaiduNetdisk) manage(opera string, filelist interface{}) ([]byte, pan.DriverErrorInterface) {
	params := map[string]string{
		"method": "filemanager",
		"opera":  opera,
	}
	marshal, _ := json.Marshal(filelist)
	return d.postForm("/xpan/file", params, map[string]string{
		"async":    "0",
		"filelist": string(marshal),
		"ondup":    "fail",
	}, nil)
}

func (d *BaiduNetdisk) create(path string, size int64, isdir int, uploadid, blockList string, resp interface{}, mtime, ctime int64) ([]byte, pan.DriverErrorInterface) {
	params := map[string]string{
		"method": "create",
	}
	form := map[string]string{
		"path":  path,
		"size":  strconv.FormatInt(size, 10),
		"isdir": strconv.Itoa(isdir),
		"rtype": "3",
	}
	if mtime != 0 && ctime != 0 {
		form["local_mtime"] = strconv.FormatInt(mtime, 10)
		form["local_ctime"] = strconv.FormatInt(ctime, 10)
	}
	if uploadid != "" {
		form["uploadid"] = uploadid
	}
	if blockList != "" {
		form["block_list"] = blockList
	}
	return d.postForm("/xpan/file", params, form, resp)
}

func (d *BaiduNetdisk) precreate(path string, size int64, blockListStr, contentMd5, sliceMd5 string, ctime, mtime int64) (*PrecreateResp, pan.DriverErrorInterface) {
	params := map[string]string{"method": "precreate"}
	form := map[string]string{
		"path":       path,
		"size":       strconv.FormatInt(size, 10),
		"isdir":      "0",
		"autoinit":   "1",
		"rtype":      "3",
		"block_list": blockListStr,
	}
	if contentMd5 != "" && sliceMd5 != "" {
		form["content-md5"] = contentMd5
		form["slice-md5"] = sliceMd5
	}
	if mtime != 0 && ctime != 0 {
		form["local_mtime"] = strconv.FormatInt(mtime, 10)
		form["local_ctime"] = strconv.FormatInt(ctime, 10)
	}
	var resp PrecreateResp
	_, err := d.postForm("/xpan/file", params, form, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (d *BaiduNetdisk) getDownloadLink(fileId string) (string, pan.DriverErrorInterface) {
	var resp DownloadResp
	params := map[string]string{
		"method": "filemetas",
		"fsids":  fmt.Sprintf("[%s]", fileId),
		"dlink":  "1",
	}
	_, err := d.get("/xpan/multimedia", params, &resp)
	if err != nil {
		return "", err
	}
	if len(resp.List) == 0 || resp.List[0].Dlink == "" {
		return "", pan.OnlyMsg("no download link available")
	}
	dlink := fmt.Sprintf("%s&access_token=%s", resp.List[0].Dlink, d.Properties.AccessToken)
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	headReq, _ := http.NewRequest("HEAD", dlink, nil)
	headReq.Header.Set("User-Agent", "pan.baidu.com")
	headResp, headErr := client.Do(headReq)
	if headErr != nil {
		return "", pan.OnlyError(headErr)
	}
	defer headResp.Body.Close()
	location := headResp.Header.Get("Location")
	if location != "" {
		return location, nil
	}
	return dlink, nil
}

func (d *BaiduNetdisk) getUploadUrl(path, uploadId string) string {
	params := map[string]string{
		"method":         "locateupload",
		"appid":          "250528",
		"path":           path,
		"uploadid":       uploadId,
		"upload_version": "2.0",
	}
	var resp UploadServerResp
	_, err := d.request("https://d.pcs.baidu.com/rest/2.0/pcs/file", http.MethodGet, func(r *req.Request) {
		r.SetQueryParams(params)
	}, &resp)
	if err != nil {
		return DefaultUploadAPI
	}
	if len(resp.Servers) > 0 {
		return resp.Servers[0].Server
	}
	if len(resp.BakServers) > 0 {
		return resp.BakServers[0].Server
	}
	return DefaultUploadAPI
}

func (d *BaiduNetdisk) uploadSlice(uploadUrl string, params map[string]string, fileName string, reader io.Reader, size int64) error {
	b := bytes.NewBuffer(make([]byte, 0, 512))
	mw := multipart.NewWriter(b)
	_, err := mw.CreateFormFile("file", fileName)
	if err != nil {
		return err
	}
	headSize := b.Len()
	_ = mw.Close()
	head := bytes.NewReader(b.Bytes()[:headSize])
	tail := bytes.NewReader(b.Bytes()[headSize:])
	body := io.MultiReader(head, reader, tail)

	httpReq, err := http.NewRequest(http.MethodPost, uploadUrl+"/rest/2.0/pcs/superfile2", body)
	if err != nil {
		return err
	}
	q := httpReq.URL.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	httpReq.URL.RawQuery = q.Encode()
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	httpReq.ContentLength = int64(b.Len()) + size

	client := &http.Client{Timeout: UploadSliceTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	respStr := strings.ToLower(string(respBody))
	if strings.Contains(respStr, "uploadid") &&
		(strings.Contains(respStr, "invalid") || strings.Contains(respStr, "expired") || strings.Contains(respStr, "not found")) {
		return fmt.Errorf("uploadid expired")
	}
	var errCheck struct {
		ErrorCode int `json:"error_code"`
		Errno     int `json:"errno"`
	}
	_ = json.Unmarshal(respBody, &errCheck)
	if errCheck.ErrorCode != 0 || errCheck.Errno != 0 {
		return fmt.Errorf("upload slice error: %s", string(respBody))
	}
	return nil
}

func (d *BaiduNetdisk) getSliceSize(fileSize int64) int64 {
	if d.vipType == 0 {
		return DefaultSliceSize
	}
	maxSlice := DefaultSliceSize
	switch d.vipType {
	case 1:
		maxSlice = VipSliceSize
	case 2:
		maxSlice = SVipSliceSize
	}
	if fileSize > MaxSliceNum*maxSlice {
		internal.GetLogger().Warn("file too large for baidu upload", "size", fileSize)
	}
	return maxSlice
}

func (d *BaiduNetdisk) getUserInfo() (int, pan.DriverErrorInterface) {
	var resp UserInfoResp
	_, err := d.get("/xpan/nas", map[string]string{"method": "uinfo"}, &resp)
	if err != nil {
		return 0, err
	}
	return resp.VipType, nil
}
