package baidu_netdisk

type TokenErrResp struct {
	ErrorDescription string `json:"error_description"`
	Error            string `json:"error"`
}

type TokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type BaiduFile struct {
	FsId           int64  `json:"fs_id"`
	Path           string `json:"path"`
	ServerFilename string `json:"server_filename"`
	Size           int64  `json:"size"`
	Isdir          int    `json:"isdir"`
	Category       int    `json:"category"`
	Md5            string `json:"md5"`
	ServerCtime    int64  `json:"server_ctime"`
	ServerMtime    int64  `json:"server_mtime"`
	LocalCtime     int64  `json:"local_ctime"`
	LocalMtime     int64  `json:"local_mtime"`
	Ctime          int64  `json:"ctime"`
	Mtime          int64  `json:"mtime"`
	Thumbs         struct {
		Url3 string `json:"url3"`
	} `json:"thumbs"`
}

type ListResp struct {
	Errno int         `json:"errno"`
	List  []BaiduFile `json:"list"`
}

type DownloadResp struct {
	Errno int `json:"errno"`
	List  []struct {
		Dlink string `json:"dlink"`
	} `json:"list"`
}

type PrecreateResp struct {
	Errno      int   `json:"errno"`
	ReturnType int   `json:"return_type"`
	Path       string `json:"path"`
	Uploadid   string `json:"uploadid"`
	BlockList  []int  `json:"block_list"`
	File       BaiduFile `json:"info"`
}

type QuotaResp struct {
	Errno int   `json:"errno"`
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
}

type UploadServerResp struct {
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
	Servers   []struct {
		Server string `json:"server"`
	} `json:"servers"`
	BakServers []struct {
		Server string `json:"server"`
	} `json:"bak_servers"`
}

type UserInfoResp struct {
	Errno   int `json:"errno"`
	VipType int `json:"vip_type"`
}
