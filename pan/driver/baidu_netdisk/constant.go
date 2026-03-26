package baidu_netdisk

import "time"

const (
	cacheDirectoryPrefix = "directory_"
)

const (
	API_URL   = "https://pan.baidu.com/rest/2.0"
	TOKEN_URL = "https://openapi.baidu.com/oauth/2.0/token"
	QUOTA_URL = "https://pan.baidu.com/api/quota"
)

const (
	DefaultSliceSize int64 = 4 * 1024 * 1024  // 4MB non-VIP
	VipSliceSize     int64 = 16 * 1024 * 1024  // 16MB VIP
	SVipSliceSize    int64 = 32 * 1024 * 1024  // 32MB SVIP
	MaxSliceNum            = 2048
	SliceStep        int64 = 1 * 1024 * 1024   // 1MB step for low-bandwidth mode
	FirstSliceSize   int64 = 256 * 1024         // 256KB for slice-md5
)

const (
	DefaultUploadAPI       = "https://d.pcs.baidu.com"
	UploadSliceTimeout     = 60 * time.Second
	MaxUploadRetry         = 3
	UploadRetryWait        = time.Second
	UploadRetryMaxWait     = 5 * time.Second
	DefaultUploadThread    = 3
	MaxUploadThread        = 32
)
