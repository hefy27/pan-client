package quark

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hefeiyu25/pan-client/internal"
	"github.com/hefeiyu25/pan-client/pan"
	"github.com/imroc/req/v3"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func funReturn(err error, response *req.Response, result Resp) (*Resp, pan.DriverErrorInterface) {
	if err != nil {
		return nil, pan.OnlyError(err)
	}
	if response.IsErrorState() && result.Code != 0 {
		return nil, pan.CodeMsg(result.Code, result.Msg)
	}
	return &result, pan.NoError()
}
func funReturnBySuccess[T any](err error, response *req.Response, errorResult Resp, successResult RespData[T]) (*RespData[T], pan.DriverErrorInterface) {
	if err != nil {
		return nil, pan.OnlyError(err)
	}
	if response.IsErrorState() {
		return nil, pan.CodeMsg(errorResult.Code, errorResult.Msg)
	}
	if successResult.Code != 0 {
		return nil, pan.CodeMsg(successResult.Code, successResult.Msg)
	}
	return &successResult, pan.NoError()
}

func funReturnBySuccessMeta[T any, R any](err error, response *req.Response, errorResult Resp, successResult RespDataWithMeta[T, R]) (*RespDataWithMeta[T, R], pan.DriverErrorInterface) {
	if err != nil {
		return nil, pan.OnlyError(err)
	}
	if response.IsErrorState() {
		return nil, pan.CodeMsg(errorResult.Code, errorResult.Msg)
	}
	if successResult.Code != 0 {
		return nil, pan.CodeMsg(successResult.Code, successResult.Msg)
	}
	return &successResult, pan.NoError()
}

func checkTaskSuccess(finish bool, successResult RespDataWithMeta[TaskDoing, TaskMeta], c *Quark) pan.DriverErrorInterface {
	isFinish := finish
	taskId := successResult.Data.TaskId
	for {
		if isFinish || taskId == "" {
			break
		}
		time.Sleep(time.Duration(successResult.Metadata.TqGap) * time.Millisecond)
		query, err := c.taskQuery(taskId)
		if err != nil {
			return err
		}
		// 暂时断定status==2是完成
		isFinish = query.Data.Status == 2
	}
	return nil
}

func (q *Quark) taskQuery(taskId string) (*RespDataWithMeta[Task, TaskMeta], pan.DriverErrorInterface) {
	r := q.sessionClient.R()
	var successResult RespDataWithMeta[Task, TaskMeta]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	r.SetQueryParamsAnyType(map[string]any{
		"task_id": taskId,
	})
	// task
	response, err := r.Get("/task")
	return funReturnBySuccessMeta(err, response, errorResult, successResult)
}

func (q *Quark) member() (*RespDataWithMeta[MemberData, MemberMeta], pan.DriverErrorInterface) {
	r := q.sessionClient.R()
	var successResult RespDataWithMeta[MemberData, MemberMeta]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	response, err := r.Get("/member")
	return funReturnBySuccessMeta(err, response, errorResult, successResult)
}

func (q *Quark) config() (*RespData[Config], pan.DriverErrorInterface) {
	r := q.sessionClient.R()
	var successResult RespData[Config]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	response, err := r.Get("/config")
	if err != nil {
		return nil, pan.OnlyError(err)
	}
	if response.IsErrorState() {
		return nil, pan.CodeMsg(errorResult.Code, errorResult.Msg)
	}
	if successResult.Code != 0 {
		return nil, pan.CodeMsg(successResult.Code, successResult.Msg)
	}
	// cookies 由 CookieJar 自动管理和回写，无需手动处理
	return &successResult, pan.NoError()
}

func (q *Quark) createDirectory(dirName, dstId string) (*RespData[Dir], pan.DriverErrorInterface) {
	r := q.sessionClient.R()
	var successResult RespData[Dir]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	r.SetBody(map[string]any{
		"dir_init_lock": false,
		"dir_path":      "",
		"file_name":     dirName,
		"pdir_fid":      dstId,
	})
	//directory
	response, err := r.Post("/file")
	return funReturnBySuccess(err, response, errorResult, successResult)
}

func (q *Quark) fileSort(parent string) ([]File, pan.DriverErrorInterface) {
	files := make([]File, 0)
	r := q.sessionClient.R()
	page := 1
	size := 100
	query := map[string]string{
		"pdir_fid":     parent,
		"_size":        strconv.Itoa(size),
		"_fetch_total": "1",
	}
	var successResult RespDataWithMeta[FileList, SortMeta]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	for {
		query["_page"] = strconv.Itoa(page)
		r.SetQueryParams(query)
		response, err := r.Get("/file/sort")
		if err != nil {
			return nil, pan.OnlyError(err)
		}
		if response.IsErrorState() {
			return nil, pan.CodeMsg(errorResult.Code, errorResult.Msg)
		}
		if successResult.Status >= 400 || successResult.Code != 0 {
			return nil, pan.CodeMsg(successResult.Code, successResult.Msg)
		}
		files = append(files, successResult.Data.List...)
		if page*size >= successResult.Metadata.Total {
			break
		}
		page++
	}

	return files, nil
}

func (q *Quark) objectDelete(objIds []string) pan.DriverErrorInterface {

	r := q.sessionClient.R()
	var successResult RespDataWithMeta[TaskDoing, TaskMeta]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	r.SetBody(map[string]any{
		"action_type":  2,
		"exclude_fids": []string{},
		"filelist":     objIds,
	})
	// object
	response, err := r.Post("/file/delete")
	result, e := funReturnBySuccessMeta(err, response, errorResult, successResult)
	if e != nil {
		return e
	}
	finish := result.Data.Finish
	return checkTaskSuccess(finish, successResult, q)
}

func (q *Quark) objectMove(objIds []string, dstId string) pan.DriverErrorInterface {
	r := q.sessionClient.R()
	var successResult RespDataWithMeta[TaskDoing, TaskMeta]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	r.SetBody(map[string]any{
		"action_type":  2,
		"exclude_fids": []string{},
		"filelist":     objIds,
		"to_pdir_fid":  dstId,
	})
	// object
	response, err := r.Post("/file/move")
	if err != nil {
		return pan.OnlyError(err)
	}
	if response.IsErrorState() {
		return pan.CodeMsg(errorResult.Code, errorResult.Msg)
	}
	if successResult.Status >= 400 || successResult.Code != 0 {
		return pan.CodeMsg(successResult.Code, successResult.Msg)
	}
	finish := successResult.Data.Finish
	return checkTaskSuccess(finish, successResult, q)
}

func (q *Quark) objectRename(objId, newName string) pan.DriverErrorInterface {
	r := q.sessionClient.R()
	var successResult RespDataWithMeta[TaskDoing, TaskMeta]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	r.SetBody(map[string]any{
		"fid":       objId,
		"file_name": newName,
	})
	// object
	response, err := r.Post("/file/rename")
	if err != nil {
		return pan.OnlyError(err)
	}
	if response.IsErrorState() {
		return pan.CodeMsg(errorResult.Code, errorResult.Msg)
	}
	if successResult.Status >= 400 || successResult.Code != 0 {
		return pan.CodeMsg(successResult.Code, successResult.Msg)
	}
	finish := successResult.Data.Finish
	return checkTaskSuccess(finish, successResult, q)
}

func (q *Quark) FileUploadPre(req FileUpPreReq) (*RespDataWithMeta[FileUpPre, FileUpPreMeta], error) {
	r := q.sessionClient.R()
	var successResult RespDataWithMeta[FileUpPre, FileUpPreMeta]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	now := time.Now()
	response, err := r.SetBody(map[string]any{
		"ccp_hash_update": true,
		"dir_name":        "",
		"file_name":       req.FileName,
		"format_type":     req.MimeType,
		"l_created_at":    now.UnixMilli(),
		"l_updated_at":    now.UnixMilli(),
		"pdir_fid":        req.ParentId,
		"size":            req.FileSize,
	}).Post("/file/upload/pre")
	if err != nil {
		return nil, err
	}
	if response.IsErrorState() {
		return nil, fmt.Errorf("code: %d, msg: %s", errorResult.Code, errorResult.Msg)
	}
	if successResult.Status >= 400 || successResult.Code != 0 {
		return nil, fmt.Errorf("code: %d, msg: %s", successResult.Code, successResult.Msg)
	}

	return &successResult, nil
}

func (q *Quark) FileUploadHash(req FileUpHashReq) (*RespData[FileUpHash], error) {
	r := q.sessionClient.R()
	var successResult RespData[FileUpHash]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	response, err := r.SetBody(req).Post("/file/update/hash")
	if err != nil {
		return nil, err
	}
	if response.IsErrorState() {
		return nil, fmt.Errorf("code: %d, msg: %s", errorResult.Code, errorResult.Msg)
	}
	if successResult.Status >= 400 || successResult.Code != 0 {
		return nil, fmt.Errorf("code: %d, msg: %s", successResult.Code, successResult.Msg)
	}

	return &successResult, nil
}

func (q *Quark) FileUpPart(req FileUpPartReq) (string, error) {
	timeStr := time.Now().UTC().Format(http.TimeFormat)
	data := map[string]any{
		"auth_info": req.AuthInfo,
		"auth_meta": fmt.Sprintf(`PUT

%s
%s
x-oss-date:%s
x-oss-user-agent:aliyun-sdk-js/6.6.1 Chrome 98.0.4758.80 on Windows 10 64-bit
/%s/%s?partNumber=%d&uploadId=%s`, req.MineType, timeStr, timeStr, req.Bucket, req.ObjKey, req.PartNumber, req.UploadId),
		"task_id": req.TaskId,
	}
	r := q.sessionClient.R()
	var resp RespData[FileUpAuth]
	r.SetSuccessResult(&resp)
	r.SetBody(data)
	_, err := r.Post("/file/upload/auth")
	if err != nil {
		return "", err
	}

	u := fmt.Sprintf("https://%s.%s/%s", req.Bucket, req.UploadUrl[7:], req.ObjKey)
	r = q.defaultClient.R()
	r.SetHeaders(map[string]string{
		"Authorization":    resp.Data.AuthKey,
		"Content-Type":     req.MineType,
		"Referer":          "https://pan.quark.cn/",
		"x-oss-date":       timeStr,
		"x-oss-user-agent": "aliyun-sdk-js/6.6.1 Chrome 98.0.4758.80 on Windows 10 64-bit",
	}).
		SetQueryParams(map[string]string{
			"partNumber": strconv.Itoa(req.PartNumber),
			"uploadId":   req.UploadId,
		}).SetBody(req.Reader)

	res, err := r.Put(u)
	if err != nil {
		return "", err
	}
	if res.StatusCode != 200 {
		return "", fmt.Errorf("up status: %d, error: %s", res.StatusCode, res.String())
	}
	return res.Header.Get("ETag"), nil
}

func (q *Quark) FileUpCommit(req FileUpCommitReq, md5s []string) error {
	timeStr := time.Now().UTC().Format(http.TimeFormat)
	bodyBuilder := strings.Builder{}
	bodyBuilder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<CompleteMultipartUpload>
`)
	for i, m := range md5s {
		bodyBuilder.WriteString(fmt.Sprintf(`<Part>
<PartNumber>%d</PartNumber>
<ETag>%s</ETag>
</Part>
`, i+1, m))
	}
	bodyBuilder.WriteString("</CompleteMultipartUpload>")
	body := bodyBuilder.String()
	m := md5.New()
	m.Write([]byte(body))
	contentMd5 := base64.StdEncoding.EncodeToString(m.Sum(nil))
	callbackBytes, err := json.Marshal(req.Callback)
	if err != nil {
		return err
	}
	callbackBase64 := base64.StdEncoding.EncodeToString(callbackBytes)
	data := map[string]any{
		"auth_info": req.AuthInfo,
		"auth_meta": fmt.Sprintf(`POST
%s
application/xml
%s
x-oss-callback:%s
x-oss-date:%s
x-oss-user-agent:aliyun-sdk-js/6.6.1 Chrome 98.0.4758.80 on Windows 10 64-bit
/%s/%s?uploadId=%s`,
			contentMd5, timeStr, callbackBase64, timeStr,
			req.Bucket, req.ObjKey, req.UploadId),
		"task_id": req.TaskId,
	}
	var resp RespData[FileUpAuth]
	r := q.sessionClient.R()
	r.SetSuccessResult(&resp)
	r.SetBody(data)
	_, err = r.Post("/file/upload/auth")
	if err != nil {
		return err
	}

	r = q.defaultClient.R()
	u := fmt.Sprintf("https://%s.%s/%s", req.Bucket, req.UploadUrl[7:], req.ObjKey)
	res, err := r.
		SetHeaders(map[string]string{
			"Authorization":    resp.Data.AuthKey,
			"Content-MD5":      contentMd5,
			"Content-Type":     "application/xml",
			"Referer":          "https://pan.quark.cn/",
			"x-oss-callback":   callbackBase64,
			"x-oss-date":       timeStr,
			"x-oss-user-agent": "aliyun-sdk-js/6.6.1 Chrome 98.0.4758.80 on Windows 10 64-bit",
		}).
		SetQueryParams(map[string]string{
			"uploadId": req.UploadId,
		}).SetBody(body).Post(u)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("up status: %d, error: %s", res.StatusCode, res.String())
	}
	return nil
}

func (q *Quark) FileUpFinish(req FileUpFinishReq) (*Resp, error) {
	r := q.sessionClient.R()
	var result Resp
	r.SetSuccessResult(&result)
	r.SetErrorResult(&result)
	response, err := r.SetBody(req).Post("/file/upload/finish")
	if err != nil {
		return nil, err
	}
	if response.IsErrorState() {
		return nil, fmt.Errorf("code: %d, msg: %s", result.Code, result.Msg)
	}
	if result.Status >= 400 || result.Code != 0 {
		return nil, fmt.Errorf("code: %d, msg: %s", result.Code, result.Msg)
	}
	return &result, nil
}

func (q *Quark) fileDownload(fileId string) (*RespData[[]DownloadData], pan.DriverErrorInterface) {
	r := q.sessionClient.R()
	data := map[string]any{
		"fids": []string{fileId},
	}
	var successResult RespData[[]DownloadData]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	response, err := r.SetBody(data).Post("/file/download")
	return funReturnBySuccess(err, response, errorResult, successResult)
}

func (q *Quark) share(req ShareReq) (string, pan.DriverErrorInterface) {
	shareId := ""
	if req.UrlType == 2 && req.Passcode == "" {
		req.Passcode = internal.GenRandomWord()
	}
	r := q.sessionClient.R()
	var successResult RespDataWithMeta[TaskDoing, TaskMeta]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	r.SetBody(req)
	// share
	response, err := r.Post("/share")
	result, e := funReturnBySuccessMeta(err, response, errorResult, successResult)
	if e != nil {
		return shareId, e
	}
	isFinish := false

	for {
		if isFinish {
			break
		}
		time.Sleep(time.Duration(result.Metadata.TqGap) * time.Millisecond)
		query, err := q.taskQuery(result.Data.TaskId)
		if err != nil {
			return shareId, err
		}
		// 暂时断定status==2是完成
		isFinish = query.Data.Status == 2
		shareId = query.Data.ShareId
	}

	return shareId, nil
}

func (q *Quark) sharePassword(shareId string) (*RespData[SharePasswordData], pan.DriverErrorInterface) {
	r := q.sessionClient.R()
	var successResult RespData[SharePasswordData]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	r.SetBody(map[string]interface{}{
		"share_id": shareId,
	})
	// password
	response, err := r.Post("/share/password")
	return funReturnBySuccess(err, response, errorResult, successResult)
}

func (q *Quark) shareList() ([]*ShareList, pan.DriverErrorInterface) {
	shareList := make([]*ShareList, 0)
	r := q.sessionClient.R()
	page := 1
	size := 100
	query := map[string]string{
		"_size":        strconv.Itoa(size),
		"_fetch_total": "1",
	}
	var successResult RespDataWithMeta[ShareDetail, SortMeta]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	for {
		query["_page"] = strconv.Itoa(page)
		r.SetQueryParams(query)
		response, err := r.Get("/share/mypage/detail")
		result, e := funReturnBySuccessMeta(err, response, errorResult, successResult)
		if e != nil {
			return nil, e
		}
		shareList = append(shareList, successResult.Data.List...)
		if page*size >= result.Metadata.Total {
			break
		}
		page++
	}

	return shareList, nil
}

func (q *Quark) shareDelete(shareIds []string) (*Resp, error) {
	r := q.sessionClient.R()
	var result Resp
	r.SetSuccessResult(&result)
	r.SetErrorResult(&result)
	r.SetBody(map[string]interface{}{
		"share_ids": shareIds,
	})
	// share/delete
	response, err := r.Post("/share/delete")
	return funReturn(err, response, result)
}

func (q *Quark) shareToken(shareTokenReq ShareTokenReq) (*RespData[ShareTokenResp], error) {
	r := q.sessionClient.R()
	var successResult RespData[ShareTokenResp]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	r.SetBody(shareTokenReq)
	response, err := r.Post("/share/sharepage/token")
	return funReturnBySuccess(err, response, errorResult, successResult)
}

func (q *Quark) shareDetail(shareDetailReq ShareDetailReq) (*ShareDetailResp, error) {
	r := q.sessionClient.R()
	page := 1
	size := 100
	query := map[string]string{
		"pwd_id":       shareDetailReq.PwdId,
		"stoken":       shareDetailReq.Stoken,
		"pdir_fid":     "0",
		"force":        "0",
		"_fetch_share": "1",
		"_size":        strconv.Itoa(size),
		"_fetch_total": "1",
	}
	var successResult RespDataWithMeta[ShareDetailResp, SortMeta]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	returnResult := &ShareDetailResp{
		IsOwner: 0,
		Share:   nil,
		List:    make([]*File, 0),
	}
	for {
		query["_page"] = strconv.Itoa(page)
		r.SetQueryParams(query)
		response, err := r.Get("/share/sharepage/detail")
		result, e := funReturnBySuccessMeta(err, response, errorResult, successResult)
		if e != nil {
			return nil, e
		}
		returnResult.Share = successResult.Data.Share
		returnResult.List = append(returnResult.List, successResult.Data.List...)
		if page*size >= result.Metadata.Total {
			break
		}
		page++
	}

	return returnResult, nil
}

func (q *Quark) shareRestore(restoreReq RestoreReq) pan.DriverErrorInterface {
	r := q.sessionClient.R()
	var successResult RespDataWithMeta[TaskDoing, TaskMeta]
	var errorResult Resp
	r.SetSuccessResult(&successResult)
	r.SetErrorResult(&errorResult)
	r.SetBody(restoreReq)
	response, err := r.Post("/share/sharepage/save")
	result, e := funReturnBySuccessMeta(err, response, errorResult, successResult)
	if e != nil {
		return e
	}
	return checkTaskSuccess(false, *result, q)
}
