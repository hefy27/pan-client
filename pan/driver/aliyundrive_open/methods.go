package aliyundrive_open

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/hefeiyu25/pan-client/pan"
	"github.com/imroc/req/v3"
)

func (d *AliyundriveOpen) refreshToken() error {
	if d.Properties.ClientId == "" || d.Properties.ClientSecret == "" {
		return fmt.Errorf("empty ClientId or ClientSecret")
	}
	r := d.sessionClient.R()
	r.SetContentType("application/json")
	body := map[string]string{
		"client_id":     d.Properties.ClientId,
		"client_secret": d.Properties.ClientSecret,
		"grant_type":    "refresh_token",
		"refresh_token": d.Properties.RefreshToken,
	}
	var errResp ErrResp
	r.SetBody(body).SetErrorResult(&errResp)
	response, err := r.Post(API_URL + "/oauth/access_token")
	if err != nil {
		return err
	}
	respBody := response.Bytes()
	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.Unmarshal(respBody, &result)
	if errResp.Code != "" {
		return fmt.Errorf("refresh token error: %s - %s", errResp.Code, errResp.Message)
	}
	if result.RefreshToken == "" {
		return fmt.Errorf("empty refresh token returned")
	}
	d.Properties.AccessToken = result.AccessToken
	d.Properties.RefreshToken = result.RefreshToken
	d.NotifyChange()
	return nil
}

func (d *AliyundriveOpen) request(uri, method string, callback func(r *req.Request), resp interface{}) ([]byte, pan.DriverErrorInterface) {
	return d.requestWithRetry(uri, method, callback, resp, false)
}

func (d *AliyundriveOpen) requestWithRetry(uri, method string, callback func(r *req.Request), resp interface{}, isRetry bool) ([]byte, pan.DriverErrorInterface) {
	r := d.sessionClient.R()
	r.SetHeader("Authorization", "Bearer "+d.Properties.AccessToken)
	if method == http.MethodPost {
		r.SetContentType("application/json")
	}
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
		response, err = r.Get(API_URL + uri)
	case http.MethodPost:
		response, err = r.Post(API_URL + uri)
	case http.MethodPut:
		response, err = r.Put(API_URL + uri)
	default:
		response, err = r.Send(method, API_URL+uri)
	}
	if err != nil {
		return nil, pan.OnlyError(err)
	}
	body := response.Bytes()
	var errResp ErrResp
	_ = json.Unmarshal(body, &errResp)
	if errResp.Code != "" {
		if !isRetry && (errResp.Code == "AccessTokenInvalid" || errResp.Code == "AccessTokenExpired" || errResp.Code == "I400JD" || d.Properties.AccessToken == "") {
			if refreshErr := d.refreshToken(); refreshErr != nil {
				return nil, pan.OnlyError(refreshErr)
			}
			return d.requestWithRetry(uri, method, callback, resp, true)
		}
		return body, pan.OnlyMsg(fmt.Sprintf("%s: %s", errResp.Code, errResp.Message))
	}
	return body, nil
}

func (d *AliyundriveOpen) getFiles(fileId string) ([]AliyunFile, pan.DriverErrorInterface) {
	marker := "first"
	result := make([]AliyunFile, 0)
	for marker != "" {
		if marker == "first" {
			marker = ""
		}
		body := map[string]interface{}{
			"drive_id":        d.driveId,
			"limit":           200,
			"marker":          marker,
			"order_by":        d.Properties.OrderBy,
			"order_direction": d.Properties.OrderDirection,
			"parent_file_id":  fileId,
		}
		var resp FilesResp
		_, err := d.request("/adrive/v1.0/openFile/list", http.MethodPost, func(r *req.Request) {
			r.SetBody(body).SetSuccessResult(&resp)
		}, nil)
		if err != nil {
			return nil, err
		}
		marker = resp.NextMarker
		result = append(result, resp.Items...)
	}
	return result, nil
}

func (d *AliyundriveOpen) getDownloadUrl(fileId string) (string, pan.DriverErrorInterface) {
	body := map[string]interface{}{
		"drive_id":   d.driveId,
		"file_id":    fileId,
		"expire_sec": 14400,
	}
	res, err := d.request("/adrive/v1.0/openFile/getDownloadUrl", http.MethodPost, func(r *req.Request) {
		r.SetBody(body)
	}, nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Url string `json:"url"`
	}
	_ = json.Unmarshal(res, &result)
	if result.Url == "" {
		return "", pan.OnlyMsg("get download url failed")
	}
	return result.Url, nil
}

func (d *AliyundriveOpen) createFile(parentFileId, name string, size int64, partCount int, contentHash, preHash, proofCode string) (*CreateResp, []byte, pan.DriverErrorInterface) {
	partInfoList := make([]map[string]int, partCount)
	for i := 0; i < partCount; i++ {
		partInfoList[i] = map[string]int{"part_number": i + 1}
	}
	body := map[string]interface{}{
		"drive_id":        d.driveId,
		"parent_file_id":  parentFileId,
		"name":            name,
		"type":            "file",
		"check_name_mode": "ignore",
		"size":            size,
		"part_info_list":  partInfoList,
	}
	if preHash != "" {
		body["pre_hash"] = preHash
	}
	if contentHash != "" {
		body["content_hash_name"] = "sha1"
		body["content_hash"] = contentHash
		body["proof_version"] = "v1"
		body["proof_code"] = proofCode
	}
	if contentHash == "" && preHash == "" {
		body["content_hash_name"] = "none"
		body["proof_version"] = "v1"
	}
	var resp CreateResp
	raw, err := d.request("/adrive/v1.0/openFile/create", http.MethodPost, func(r *req.Request) {
		r.SetBody(body)
	}, &resp)
	return &resp, raw, err
}

func (d *AliyundriveOpen) uploadPart(uploadUrl string, reader io.Reader) error {
	httpReq, err := http.NewRequest(http.MethodPut, uploadUrl, reader)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("upload part failed with status %d", resp.StatusCode)
	}
	return nil
}

func (d *AliyundriveOpen) completeUpload(fileId, uploadId string) pan.DriverErrorInterface {
	body := map[string]string{
		"drive_id":  d.driveId,
		"file_id":   fileId,
		"upload_id": uploadId,
	}
	_, err := d.request("/adrive/v1.0/openFile/complete", http.MethodPost, func(r *req.Request) {
		r.SetBody(body)
	}, nil)
	return err
}

func (d *AliyundriveOpen) getUploadUrl(count int, fileId, uploadId string) ([]PartInfo, pan.DriverErrorInterface) {
	partInfoList := make([]map[string]int, count)
	for i := 0; i < count; i++ {
		partInfoList[i] = map[string]int{"part_number": i + 1}
	}
	var resp CreateResp
	_, err := d.request("/adrive/v1.0/openFile/getUploadUrl", http.MethodPost, func(r *req.Request) {
		r.SetBody(map[string]interface{}{
			"drive_id":       d.driveId,
			"file_id":        fileId,
			"part_info_list": partInfoList,
			"upload_id":      uploadId,
		})
	}, &resp)
	return resp.PartInfoList, err
}

func calPartSize(fileSize int64) int64 {
	partSize := DefaultPartSize
	if fileSize > partSize {
		if fileSize > 1024*1024*1024*1024 { // >1TB
			partSize = 5 * 1024 * 1024 * 1024 // 5GB
		} else if fileSize > 768*1024*1024*1024 { // >768GB
			partSize = 109951163
		} else if fileSize > 512*1024*1024*1024 { // >512GB
			partSize = 82463373
		} else if fileSize > 384*1024*1024*1024 { // >384GB
			partSize = 54975582
		} else if fileSize > 256*1024*1024*1024 { // >256GB
			partSize = 41231687
		} else if fileSize > 128*1024*1024*1024 { // >128GB
			partSize = 27487791
		}
	}
	return partSize
}

func calProofCode(accessToken string, fileSize int64, f io.ReadSeeker) (string, error) {
	if fileSize == 0 {
		return "", nil
	}
	md5Hash := md5.Sum([]byte(accessToken))
	md5Hex := hex.EncodeToString(md5Hash[:])
	tmpInt, err := strconv.ParseUint(md5Hex[:16], 16, 64)
	if err != nil {
		return "", err
	}
	start := int64(tmpInt % uint64(fileSize))
	end := start + 8
	if end > fileSize {
		end = fileSize
	}
	_, err = f.Seek(start, io.SeekStart)
	if err != nil {
		return "", err
	}
	buf := make([]byte, end-start)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf[:n]), nil
}

func calPartCount(fileSize, partSize int64) int {
	return int(math.Ceil(float64(fileSize) / float64(partSize)))
}
