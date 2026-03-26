# Pan Client

Go 语言多云盘统一客户端 SDK，支持夸克网盘、迅雷云盘、Cloudreve，提供一致的文件操作接口。

纯 SDK 设计，无全局配置文件，无内置文件日志，所有状态管理交由调用方处理。

## 安装

```bash
go get github.com/hefeiyu25/pan-client
```

要求 Go 1.23+

## 支持的云盘

| 服务 | 驱动 | 认证方式 |
|------|------|---------|
| 夸克网盘 | `quark` | cookies.txt (Netscape 格式) |
| 迅雷云盘 | `thunder_browser` | access_token / refresh_token / 用户名密码 |
| Cloudreve | `cloudreve` | session cookie |

## 快速开始

### 初始化

```go
package main

import (
    "log/slog"
    "os"

    pan "github.com/hefeiyu25/pan-client"
    "github.com/hefeiyu25/pan-client/pan/driver/quark"
)

func main() {
    defer pan.GracefulExit()

    // 可选：设置自定义 logger
    pan.Init(pan.WithLogger(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))))

    // 创建夸克客户端
    client, err := pan.NewQuarkClient(quark.QuarkProperties{
        CookieFile: "pan.quark.cn_cookies.txt",
    },
        pan.WithDownloadMaxThread(20),
        pan.WithDownloadTmpPath("./tmp"),
    )
    if err != nil {
        panic(err)
    }

    // 列出根目录
    list, _ := client.List(pan.ListReq{
        Dir: &pan.PanObj{Path: "/", Type: "dir"},
    })
    for _, item := range list {
        slog.Info("file", "name", item.Name, "type", item.Type)
    }
}
```

注意：上面 `pan.ListReq` 和 `pan.PanObj` 的完整路径是 `github.com/hefeiyu25/pan-client/pan`，为简洁起见示例中使用短名。

### 创建其他客户端

```go
// 迅雷云盘
client, err := pan.NewThunderClient(thunder_browser.ThunderBrowserProperties{
    Username: "user",
    Password: "pass",
},
    pan.WithOnChange(func(props pan.Properties) {
        // token 刷新后回调，调用方自行持久化
        data, _ := json.Marshal(props)
        os.WriteFile("thunder.json", data, 0644)
    }),
)

// Cloudreve
client, err := pan.NewCloudreveClient(cloudreve.CloudreveProperties{
    Url:     "https://pan.example.com",
    Session: "your_session",
})
```

## ClientOption

每个客户端创建时可传入以下选项：

| Option | 说明 | 默认值 |
|--------|------|--------|
| `WithOnChange(fn)` | 属性变更回调（token 刷新等），调用方负责持久化 | nil |
| `WithContext(ctx)` | 客户端生命周期 context | `context.Background()` |
| `WithDownloadTmpPath(path)` | 分块下载临时目录 | `./download_tmp` |
| `WithDownloadMaxThread(n)` | 最大下载并发数 | 50 |
| `WithDownloadMaxRetry(n)` | 分块下载最大重试次数 | 3 |

## GlobalOption

`Init()` 接受全局选项：

| Option | 说明 | 默认值 |
|--------|------|--------|
| `WithLogger(l)` | 设置 `*slog.Logger`，库内所有日志走此 logger | `slog.Default()` |

## 文件操作

### 列出目录

```go
list, err := client.List(pan.ListReq{
    Dir:    &pan.PanObj{Path: "/", Name: "documents"},
    Reload: true, // 强制刷新缓存
})
```

### 上传

```go
// 上传单文件
err = client.UploadFile(pan.UploadFileReq{
    LocalFile:  "./data/report.pdf",
    RemotePath: "/backup",
    Resumable:  true,
})

// 上传目录
err = client.UploadPath(pan.UploadPathReq{
    LocalPath:   "./data",
    RemotePath:  "/backup",
    Resumable:   true,
    Extensions:  []string{".pdf", ".doc"}, // 只上传指定类型
    IgnorePaths: []string{"temp"},
    SuccessDel:  false,
})
```

### 下载

```go
// 下载单文件
err = client.DownloadFile(pan.DownloadFileReq{
    RemoteFile:  fileObj,
    LocalPath:   "./downloads",
    Concurrency: 4,
    ChunkSize:   50 * 1024 * 1024,
    OverCover:   true,
})

// 下载目录
err = client.DownloadPath(pan.DownloadPathReq{
    RemotePath:  dirObj,
    LocalPath:   "./downloads",
    Concurrency: 4,
    Extensions:  []string{".mp4"},
    SkipFileErr: true,
})
```

### 其他操作

```go
// 创建目录
dir, err := client.Mkdir(pan.MkdirReq{NewPath: "/new_folder"})

// 重命名
err = client.ObjRename(pan.ObjRenameReq{Obj: fileObj, NewName: "new.pdf"})

// 批量重命名
err = client.BatchRename(pan.BatchRenameReq{
    Path: dirObj,
    Func: func(obj *pan.PanObj) string {
        return "prefix_" + obj.Name
    },
})

// 移动
err = client.Move(pan.MovieReq{Items: []*pan.PanObj{f1, f2}, TargetObj: targetDir})

// 删除
err = client.Delete(pan.DeleteReq{Items: []*pan.PanObj{f1, f2}})

// 磁盘信息
disk, err := client.Disk()

// 离线下载
task, err := client.OfflineDownload(pan.OfflineDownloadReq{
    Url:        "https://example.com/file.zip",
    RemotePath: "/downloads",
})
```

## 核心接口

```go
type Driver interface {
    Meta
    Operate
    Share
}

type Meta interface {
    GetId() string
    Init() (string, error)
    Close() error
    GetProperties() Properties
    Get(key string) (interface{}, bool)
    GetOrLoad(key string, loader func() (interface{}, error)) (interface{}, error)
    Set(key string, value interface{})
    SetWithTTL(key string, value interface{}, d time.Duration)
    Del(key string)
}

type Operate interface {
    Disk() (*DiskResp, error)
    List(req ListReq) ([]*PanObj, error)
    ObjRename(req ObjRenameReq) error
    BatchRename(req BatchRenameReq) error
    Mkdir(req MkdirReq) (*PanObj, error)
    Move(req MovieReq) error
    Delete(req DeleteReq) error
    UploadPath(req UploadPathReq) error
    UploadFile(req UploadFileReq) error
    DownloadPath(req DownloadPathReq) error
    DownloadFile(req DownloadFileReq) error
    OfflineDownload(req OfflineDownloadReq) (*Task, error)
    TaskList(req TaskListReq) ([]*Task, error)
    DirectLink(req DirectLinkReq) ([]*DirectLink, error)
}

type Share interface {
    ShareList(req ShareListReq) ([]*ShareData, error)
    NewShare(req NewShareReq) (*ShareData, error)
    DeleteShare(req DelShareReq) error
    ShareRestore(req ShareRestoreReq) error
}
```

## 设计原则

- **纯 SDK**：无配置文件，无文件日志，无全局状态，所有参数通过构造函数传入
- **状态回调**：token 刷新等状态变更通过 `OnChange` 回调通知调用方，持久化由调用方负责
- **标准日志**：使用 `log/slog`，调用方通过 `WithLogger` 注入自己的 Handler
- **Per-Client 配置**：下载并发、重试等参数每个客户端独立配置
- **Context 支持**：支持通过 `WithContext` 控制客户端生命周期

## 开发

### 添加新驱动

1. 在 `pan/driver/` 下创建新包
2. 实现 `Driver` 接口
3. 在 `init()` 中注册：

```go
func init() {
    pan.RegisterDriver(pan.YourType, func() pan.Driver {
        return &YourDriver{
            PropertiesOperate: pan.PropertiesOperate[*YourProperties]{
                DriverType: pan.YourType,
            },
            CacheOperate:  pan.NewCacheOperate(),
            CommonOperate: pan.CommonOperate{},
        }
    })
}
```

4. 在 `pan/driver/all.go` 中添加 import
5. 在 `enter.go` 中添加 `NewYourClient` 工厂函数

### 测试

```bash
go test -v -run TestListDir      # 夸克目录列表
go test -v -run TestDirectLink   # 直链获取
go test -v -run TestDownloadAndUpload  # 上传下载
```

## License

MIT
