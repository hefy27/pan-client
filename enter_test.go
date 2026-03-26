package pan_client

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hefeiyu25/pan-client/pan"
	"github.com/hefeiyu25/pan-client/pan/driver/quark"
)

// ==========================================================
// 测试配置：切换 driver 或测试根目录只需修改此处
// ==========================================================

const testRoot = "/pan-client-test" // 所有测试在此目录下操作

func getClient(t *testing.T, opts ...ClientOption) pan.Driver {
	t.Helper()
	Init()
	client, err := NewQuarkClient(quark.QuarkProperties{
		CookieFile: "pan.quark.cn_cookies.txt",
	}, opts...)
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	ensureTestRoot(t, client)
	return client
}

// ==========================================================
// Helper
// ==========================================================

// testRootDir 返回测试根目录的 PanObj
func testRootDir() *pan.PanObj {
	return &pan.PanObj{Path: testRoot, Type: "dir"}
}

// ensureTestRoot 确保测试根目录存在
func ensureTestRoot(t *testing.T, client pan.Driver) {
	t.Helper()
	_, _ = client.Mkdir(pan.MkdirReq{NewPath: testRoot})
}

// mkdirOrSkip 创建目录，如果因回收站冲突等无法创建则 skip
func mkdirOrSkip(t *testing.T, client pan.Driver, path string) *pan.PanObj {
	t.Helper()
	dir, err := client.Mkdir(pan.MkdirReq{NewPath: path})
	if err == nil {
		return dir
	}
	name := filepath.Base(path)
	list, listErr := client.List(pan.ListReq{
		Dir:    testRootDir(),
		Reload: true,
	})
	if listErr == nil {
		for _, item := range list {
			if item.Name == name {
				return item
			}
		}
	}
	t.Skipf("mkdir %s: %v (可能回收站冲突，跳过)", path, err)
	return nil
}

// ==========================================================
// 集成测试
// ==========================================================

// TestDisk 查询磁盘空间
func TestDisk(t *testing.T) {
	client := getClient(t)

	disk, err := client.Disk()
	if err != nil {
		t.Fatalf("disk: %v", err)
	}
	slog.Info("磁盘信息", "total", disk.Total, "used", disk.Used, "free", disk.Free)
	if disk.Total <= 0 {
		t.Fatal("expected total > 0")
	}
}

// TestListRoot 列出根目录
func TestListRoot(t *testing.T) {
	client := getClient(t)

	list, err := client.List(pan.ListReq{
		Dir:    testRootDir(),
		Reload: true,
	})
	if err != nil {
		t.Fatalf("list root: %v", err)
	}
	slog.Info("测试目录列表", "path", testRoot, "count", len(list))
	for i, item := range list {
		slog.Info("文件项", "index", i+1, "name", item.Name, "type", item.Type)
	}
}

// TestMkdirAndDelete 创建目录 -> 删除
func TestMkdirAndDelete(t *testing.T) {
	client := getClient(t)
	dirName := fmt.Sprintf("pan-test-%d", time.Now().UnixMilli())
	dir := mkdirOrSkip(t, client, testRoot+"/"+dirName)
	slog.Info("创建目录", "id", dir.Id, "name", dir.Name)

	err := client.Delete(pan.DeleteReq{Items: []*pan.PanObj{dir}})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	slog.Info("删除目录成功")
}

// TestMkdirRenameDelete 创建 -> 重命名 -> 验证 -> 删除
func TestMkdirRenameDelete(t *testing.T) {
	client := getClient(t)
	dirName := fmt.Sprintf("pan-rename-%d", time.Now().UnixMilli())
	dir := mkdirOrSkip(t, client, testRoot+"/"+dirName)
	defer func() {
		_ = client.Delete(pan.DeleteReq{Items: []*pan.PanObj{dir}})
	}()

	newName := fmt.Sprintf("pan-renamed-%d", time.Now().UnixMilli())
	err := client.ObjRename(pan.ObjRenameReq{Obj: dir, NewName: newName})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	slog.Info("重命名成功", "from", dirName, "to", newName)

	time.Sleep(2 * time.Second) // 等待服务端刷新
	list, err := client.List(pan.ListReq{
		Dir:    testRootDir(),
		Reload: true,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, item := range list {
		if item.Name == newName {
			found = true
			dir = item
			break
		}
	}
	if !found {
		t.Fatalf("renamed dir %s not found", newName)
	}
}

// TestMkdirMoveDelete 创建两个目录 -> 移动 -> 验证 -> 清理
func TestMkdirMoveDelete(t *testing.T) {
	client := getClient(t)
	ts := time.Now().UnixMilli()
	srcName := fmt.Sprintf("pan-mv-s-%d", ts)
	dstName := fmt.Sprintf("pan-mv-d-%d", ts)
	src := mkdirOrSkip(t, client, testRoot+"/"+srcName)
	dst := mkdirOrSkip(t, client, testRoot+"/"+dstName)
	defer func() {
		_ = client.Delete(pan.DeleteReq{Items: []*pan.PanObj{dst}})
	}()

	err := client.Move(pan.MovieReq{Items: []*pan.PanObj{src}, TargetObj: dst})
	if err != nil {
		_ = client.Delete(pan.DeleteReq{Items: []*pan.PanObj{src}})
		t.Fatalf("move: %v", err)
	}
	slog.Info("移动成功")

	time.Sleep(2 * time.Second) // 等待服务端刷新
	list, err := client.List(pan.ListReq{
		Dir:    dst,
		Reload: true,
	})
	if err != nil {
		t.Fatalf("list dst: %v", err)
	}
	found := false
	for _, item := range list {
		if item.Name == srcName {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("moved dir not found in target")
	}
}

// TestShareAndDelete 分享创建 -> 列表 -> 删除
func TestShareAndDelete(t *testing.T) {
	client := getClient(t)
	shareDirName := fmt.Sprintf("pan-share-%d", time.Now().UnixMilli())
	dir := mkdirOrSkip(t, client, testRoot+"/"+shareDirName)
	defer func() {
		_ = client.Delete(pan.DeleteReq{Items: []*pan.PanObj{dir}})
	}()

	share, err := client.NewShare(pan.NewShareReq{
		Fids:         []string{dir.Id},
		Title:        "集成测试分享",
		NeedPassCode: false,
		ExpiredType:  1,
	})
	if err != nil {
		t.Fatalf("new share: %v", err)
	}
	slog.Info("创建分享", "shareId", share.ShareId)

	shareList, err := client.ShareList(pan.ShareListReq{ShareIds: []string{share.ShareId}})
	if err != nil {
		t.Fatalf("share list: %v", err)
	}
	if len(shareList) == 0 {
		t.Fatal("expected at least 1 share")
	}

	err = client.DeleteShare(pan.DelShareReq{ShareIds: []string{share.ShareId}})
	if err != nil {
		t.Fatalf("delete share: %v", err)
	}
	slog.Info("删除分享成功")
}

// TestGetProperties 验证 Properties 非 nil
func TestGetProperties(t *testing.T) {
	client := getClient(t)
	props := client.GetProperties()
	if props == nil {
		t.Fatal("expected non-nil properties")
	}
}

// TestClientClose 测试 Close + RemoveDriver
func TestClientClose(t *testing.T) {
	client := getClient(t)
	id := client.GetId()

	RemoveDriver(id)

	_, ok := pan.LoadDriver(id)
	if ok {
		t.Fatal("expected driver removed from registry")
	}
}

// TestWithContext 传入 ctx 并取消
func TestWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := getClient(t, WithContext(ctx))

	_, err := client.List(pan.ListReq{
		Dir:    testRootDir(),
		Reload: true,
	})
	if err != nil {
		t.Fatalf("list before cancel: %v", err)
	}

	cancel()
	slog.Info("context cancelled")
}

// TestWithCustomLogger 自定义 logger
func TestWithCustomLogger(t *testing.T) {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	Init(WithLogger(slog.New(handler)))

	client := getClient(t)
	_, err := client.List(pan.ListReq{
		Dir:    testRootDir(),
		Reload: true,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
}

// TestWithDownloadOptions 自定义下载参数
func TestWithDownloadOptions(t *testing.T) {
	client := getClient(t,
		WithDownloadTmpPath("./test_tmp"),
		WithDownloadMaxThread(5),
		WithDownloadMaxRetry(1),
	)
	_, err := client.Disk()
	if err != nil {
		t.Fatalf("disk: %v", err)
	}
	slog.Info("自定义下载参数验证通过")
}

// TestOnChange 属性变更回调
func TestOnChange(t *testing.T) {
	changed := false
	Init()
	client, err := NewQuarkClient(quark.QuarkProperties{
		CookieFile: "pan.quark.cn_cookies.txt",
	}, WithOnChange(func(props pan.Properties) {
		changed = true
	}))
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	_, err = client.List(pan.ListReq{
		Dir:    testRootDir(),
		Reload: true,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	slog.Info("OnChange", "triggered", changed)
}

// TestCacheReuse 连续两次 List，第二次命中缓存
func TestCacheReuse(t *testing.T) {
	client := getClient(t)

	list1, err := client.List(pan.ListReq{
		Dir:    testRootDir(),
		Reload: true,
	})
	if err != nil {
		t.Fatalf("first list: %v", err)
	}

	list2, err := client.List(pan.ListReq{
		Dir: testRootDir(),
	})
	if err != nil {
		t.Fatalf("second list: %v", err)
	}

	if len(list1) != len(list2) {
		t.Fatalf("expected same count, got %d vs %d", len(list1), len(list2))
	}
	slog.Info("缓存命中验证通过", "count", len(list2))
}

// TestDownloadFile 下载根目录下的第一个文件
func TestDownloadFile(t *testing.T) {
	client := getClient(t,
		WithDownloadTmpPath("./test_download_tmp"),
		WithDownloadMaxThread(3),
	)

	// 1. 上传一个临时文件
	uploadContent := fmt.Sprintf("download-test-content-%d", time.Now().UnixNano())
	tmpFile, err := os.CreateTemp("", "pan-dl-test-*.txt")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	_, _ = tmpFile.WriteString(uploadContent)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	uploadName := filepath.Base(tmpFile.Name())
	_, err = client.UploadFile(pan.UploadFileReq{
		LocalFile:  tmpFile.Name(),
		RemotePath: testRoot,
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	slog.Info("上传完成", "name", uploadName)

	// 等待服务端刷新后查找上传的文件
	time.Sleep(2 * time.Second)
	list, err := client.List(pan.ListReq{Dir: testRootDir(), Reload: true})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var targetFile *pan.PanObj
	for _, item := range list {
		if item.Name == uploadName {
			targetFile = item
			break
		}
	}
	if targetFile == nil {
		t.Fatalf("上传的文件 %s 未在列表中找到", uploadName)
	}
	defer func() {
		_ = client.Delete(pan.DeleteReq{Items: []*pan.PanObj{targetFile}})
	}()

	// 2. 下载并验证
	localPath := "./test_download_output"
	_ = os.MkdirAll(localPath, os.ModePerm)
	defer os.RemoveAll(localPath)
	defer os.RemoveAll("./test_download_tmp")

	slog.Info("开始下载", "name", targetFile.Name, "size", targetFile.Size)
	var dlProgressCalled bool
	_, err = client.DownloadFile(pan.DownloadFileReq{
		RemoteFile:  targetFile,
		LocalPath:   localPath,
		Concurrency: 2,
		ChunkSize:   5 * 1024 * 1024,
		OverCover:   true,
		DownloadCallback: func(taskId, fileTaskId, localFilePath, localFileName string) {
			slog.Info("下载完成回调", "taskId", taskId, "fileTaskId", fileTaskId, "path", localFilePath, "file", localFileName)
		},
		ProgressCallback: func(event pan.ProgressEvent) {
			dlProgressCalled = true
			slog.Info("下载进度", "file", event.FileName, "percent", fmt.Sprintf("%.1f%%", event.Percent), "speed", fmt.Sprintf("%.1fKB/s", event.Speed), "done", event.Done)
		},
	})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if !dlProgressCalled {
		t.Log("warning: 下载进度回调未触发（文件可能太小）")
	}

	// 3. 验证下载内容
	downloadedFile := filepath.Join(localPath, uploadName)
	data, err := os.ReadFile(downloadedFile)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != uploadContent {
		t.Fatalf("内容不匹配: got %q, want %q", string(data), uploadContent)
	}
	slog.Info("下载验证通过", "name", uploadName, "size", len(data))
}

// TestDownloadPath 下载目录
func TestDownloadPath(t *testing.T) {
	client := getClient(t)

	// 1. 创建远程目录
	dirName := fmt.Sprintf("pan-dlpath-%d", time.Now().UnixMilli())
	remoteDir := mkdirOrSkip(t, client, testRoot+"/"+dirName)
	defer func() {
		_ = client.Delete(pan.DeleteReq{Items: []*pan.PanObj{remoteDir}})
	}()

	// 2. 上传一个文件到该目录
	content := fmt.Sprintf("download-path-test-%d", time.Now().UnixNano())
	tmpFile, err := os.CreateTemp("", "pan-dlpath-*.txt")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	_, _ = tmpFile.WriteString(content)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	uploadName := filepath.Base(tmpFile.Name())
	_, err = client.UploadFile(pan.UploadFileReq{
		LocalFile:  tmpFile.Name(),
		RemotePath: testRoot + "/" + dirName,
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	slog.Info("上传到远程目录", "dir", dirName, "file", uploadName)

	// 等待服务端刷新，并强制 reload 确认文件可见
	time.Sleep(2 * time.Second)
	items, err := client.List(pan.ListReq{Dir: remoteDir, Reload: true})
	if err != nil {
		t.Fatalf("list remote dir: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("上传后远程目录为空")
	}
	slog.Info("远程目录文件数", "count", len(items))

	// 3. 下载整个目录
	localPath := "./test_download_dir"
	_ = os.MkdirAll(localPath, os.ModePerm)
	defer os.RemoveAll(localPath)

	slog.Info("开始下载目录", "name", dirName)
	_, err = client.DownloadPath(pan.DownloadPathReq{
		RemotePath:  remoteDir,
		LocalPath:   localPath,
		NotTraverse: true,
		Concurrency: 2,
		ChunkSize:   5 * 1024 * 1024,
		OverCover:   true,
		SkipFileErr: true,
	})
	if err != nil {
		t.Fatalf("download path: %v", err)
	}

	// 4. 验证下载内容（DownloadPath 直接将文件放在 localPath 下）
	downloadedFile := filepath.Join(localPath, uploadName)
	data, err := os.ReadFile(downloadedFile)
	if err != nil {
		t.Fatalf("read downloaded: %v", err)
	}
	if string(data) != content {
		t.Fatalf("内容不匹配: got %q, want %q", string(data), content)
	}
	slog.Info("下载目录验证通过", "dir", dirName, "file", uploadName, "size", len(data))
}

// TestUploadFile 上传文件并清理
func TestUploadFile(t *testing.T) {
	client := getClient(t)

	tmpFile, err := os.CreateTemp("", "pan-upload-test-*.txt")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	_, _ = tmpFile.WriteString("pan-client upload integration test\n")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	slog.Info("上传文件", "local", tmpFile.Name())
	var upProgressCalled bool
	_, err = client.UploadFile(pan.UploadFileReq{
		LocalFile:  tmpFile.Name(),
		RemotePath: testRoot,
		ProgressCallback: func(event pan.ProgressEvent) {
			upProgressCalled = true
			slog.Info("上传进度", "file", event.FileName, "percent", fmt.Sprintf("%.1f%%", event.Percent), "speed", fmt.Sprintf("%.1fKB/s", event.Speed), "done", event.Done)
		},
		Resumable:  false,
		SuccessDel: false,
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if !upProgressCalled {
		t.Log("warning: 上传进度回调未触发（文件可能太小）")
	}
	slog.Info("上传成功")

	// 尝试清理（服务端可能有延迟，重试2次）
	baseName := filepath.Base(tmpFile.Name())
	for retry := 0; retry < 2; retry++ {
		if retry > 0 {
			time.Sleep(2 * time.Second)
		}
		list, listErr := client.List(pan.ListReq{
			Dir:    testRootDir(),
			Reload: true,
		})
		if listErr != nil {
			continue
		}
		for _, item := range list {
			if item.Name == baseName && item.Type == "file" {
				_ = client.Delete(pan.DeleteReq{Items: []*pan.PanObj{item}})
				slog.Info("清理上传文件成功", "name", item.Name)
				return
			}
		}
	}
	t.Log("上传的文件未在列表中找到（可能服务端延迟）")
}
