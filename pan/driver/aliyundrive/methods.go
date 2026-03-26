package aliyundrive

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/hefy27/pan-client/internal"
	"github.com/hefy27/pan-client/pan"
	"github.com/imroc/req/v3"
)

func (d *AliDrive) refreshToken() error {
	r := d.sessionClient.R()
	r.SetContentType("application/json")
	body := map[string]string{
		"refresh_token": d.Properties.RefreshToken,
		"grant_type":    "refresh_token",
	}
	var errResp RespErr
	var tokenResp TokenResp
	r.SetBody(body).SetSuccessResult(&tokenResp).SetErrorResult(&errResp)
	response, err := r.Post(AUTH_URL + "/v2/account/token")
	if err != nil {
		return err
	}
	if response.IsErrorState() || errResp.Code != "" {
		return fmt.Errorf("refresh token error: %s - %s", errResp.Code, errResp.Message)
	}
	if tokenResp.RefreshToken == "" {
		return fmt.Errorf("empty refresh token returned")
	}
	d.Properties.RefreshToken = tokenResp.RefreshToken
	d.Properties.AccessToken = tokenResp.AccessToken
	d.NotifyChange()
	return nil
}

func (d *AliDrive) request(url, method string, callback func(r *req.Request), resp interface{}) ([]byte, pan.DriverErrorInterface) {
	return d.requestInner(url, method, callback, resp, false)
}

func (d *AliDrive) requestInner(url, method string, callback func(r *req.Request), resp interface{}, isRetry bool) ([]byte, pan.DriverErrorInterface) {
	r := d.sessionClient.R()
	state := getOrCreateState(d.userId)
	r.SetHeaders(map[string]string{
		"Authorization": "Bearer\t" + d.Properties.AccessToken,
		"Content-Type":  "application/json",
		"Origin":        "https://www.alipan.com",
		"Referer":       "https://alipan.com/",
		"X-Signature":   state.signature,
		"X-Request-Id":  uuid.NewString(),
		"X-Canary":      "client=Android,app=adrive,version=v4.1.0",
		"X-Device-Id":   state.deviceID,
	})
	if callback != nil {
		callback(r)
	} else {
		r.SetBody("{}")
	}
	if resp != nil {
		r.SetSuccessResult(resp)
	}
	var response *req.Response
	var err error
	switch method {
	case http.MethodGet:
		response, err = r.Get(url)
	case http.MethodPost:
		response, err = r.Post(url)
	case http.MethodPut:
		response, err = r.Put(url)
	default:
		response, err = r.Send(method, url)
	}
	if err != nil {
		return nil, pan.OnlyError(err)
	}
	body := response.Bytes()
	var errResp RespErr
	_ = json.Unmarshal(body, &errResp)
	if errResp.Code != "" {
		if !isRetry {
			switch errResp.Code {
			case "AccessTokenInvalid":
				if refreshErr := d.refreshToken(); refreshErr != nil {
					return nil, pan.OnlyError(refreshErr)
				}
				return d.requestInner(url, method, callback, resp, true)
			case "DeviceSessionSignatureInvalid":
				if sessionErr := d.doCreateSession(); sessionErr != nil {
					return nil, pan.OnlyError(sessionErr)
				}
				return d.requestInner(url, method, callback, resp, true)
			}
		}
		return nil, pan.OnlyMsg(fmt.Sprintf("%s: %s", errResp.Code, errResp.Message))
	}
	return body, nil
}

func (d *AliDrive) doCreateSession() error {
	state := getOrCreateState(d.userId)
	signData(state, d.userId)
	r := d.sessionClient.R()
	r.SetHeaders(map[string]string{
		"Authorization": "Bearer\t" + d.Properties.AccessToken,
		"Content-Type":  "application/json",
	})
	r.SetBody(map[string]interface{}{
		"deviceName":   "samsung",
		"modelName":    "SM-G9810",
		"nonce":        0,
		"pubKey":       publicKeyToHex(&state.privateKey.PublicKey),
		"refreshToken": d.Properties.RefreshToken,
	})
	response, err := r.Post(API_URL + "/users/v1/users/device/create_session")
	if err != nil {
		return err
	}
	if response.IsErrorState() {
		return fmt.Errorf("create session failed: %s", response.String())
	}
	return nil
}

func (d *AliDrive) getFiles(fileId string) ([]AliyunFile, pan.DriverErrorInterface) {
	marker := "first"
	result := make([]AliyunFile, 0)
	for marker != "" {
		if marker == "first" {
			marker = ""
		}
		var resp FilesResp
		body := map[string]interface{}{
			"drive_id":                d.driveId,
			"fields":                  "*",
			"image_thumbnail_process": "image/resize,w_400/format,jpeg",
			"image_url_process":       "image/resize,w_1920/format,jpeg",
			"limit":                   200,
			"marker":                  marker,
			"order_by":                d.Properties.OrderBy,
			"order_direction":         d.Properties.OrderDirection,
			"parent_file_id":          fileId,
			"video_thumbnail_process": "video/snapshot,t_0,f_jpg,ar_auto,w_300",
			"url_expire_sec":          14400,
		}
		_, err := d.request(API_URL+"/v2/file/list", http.MethodPost, func(r *req.Request) {
			r.SetBody(body)
		}, &resp)
		if err != nil {
			return nil, err
		}
		marker = resp.NextMarker
		result = append(result, resp.Items...)
	}
	return result, nil
}

func (d *AliDrive) getDownloadUrl(fileId string) (string, pan.DriverErrorInterface) {
	body := map[string]interface{}{
		"drive_id":   d.driveId,
		"file_id":    fileId,
		"expire_sec": 14400,
	}
	res, err := d.request(API_URL+"/v2/file/get_download_url", http.MethodPost, func(r *req.Request) {
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

func (d *AliDrive) batch(srcId, dstId, batchUrl string) pan.DriverErrorInterface {
	body := map[string]interface{}{
		"requests": []map[string]interface{}{
			{
				"headers": map[string]string{"Content-Type": "application/json"},
				"method":  "POST",
				"id":      srcId,
				"body": map[string]string{
					"drive_id":          d.driveId,
					"file_id":           srcId,
					"to_drive_id":       d.driveId,
					"to_parent_file_id": dstId,
				},
				"url": batchUrl,
			},
		},
		"resource": "file",
	}
	res, err := d.request(API_URL+"/v3/batch", http.MethodPost, func(r *req.Request) {
		r.SetBody(body)
	}, nil)
	if err != nil {
		return err
	}
	var batchResp struct {
		Responses []struct {
			Status int `json:"status"`
		} `json:"responses"`
	}
	_ = json.Unmarshal(res, &batchResp)
	if len(batchResp.Responses) > 0 && (batchResp.Responses[0].Status >= 400 || batchResp.Responses[0].Status < 100) {
		return pan.OnlyMsg(fmt.Sprintf("batch operation failed: %s", string(res)))
	}
	return nil
}

func (d *AliDrive) startTokenRefreshCron() {
	go func() {
		ticker := time.NewTicker(2 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-d.Ctx.Done():
				return
			case <-ticker.C:
				if err := d.refreshToken(); err != nil {
					internal.GetLogger().Error("aliyundrive token refresh failed", "error", err)
				}
			}
		}
	}()
}
