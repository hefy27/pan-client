package thunder_browser

const (
	cacheDirectoryPrefix = "directory_"
)

const (
	API_URL             = "https://x-api-pan.xunlei.com/drive/v1"
	XLUSER_API_BASE_URL = "https://xluser-ssl.xunlei.com"
	XLUSER_API_URL      = XLUSER_API_BASE_URL + "/v1"
)

const (
	SignProvider = "access_end_point_token"
	APPID        = "22062"
	APPKey       = "a5d7416858147a4ab99573872ffccef8"
)

const (
	ClientID          = "ZUBzD9J_XPXfn7f7"
	ClientSecret      = "yESVmHecEe6F0aou69vl-g"
	ClientVersion     = "1.40.0.7208"
	PackageName       = "com.xunlei.browser"
	HeaderUserAgent   = "User-Agent"
	DownloadUserAgent = "AndroidDownloadManager/13 (Linux; U; Android 13; M2004J7AC Build/SP1A.210812.016)"
	SdkVersion        = "509300"
)

var Algorithms = []string{
	"Cw4kArmKJ/aOiFTxnQ0ES+D4mbbrIUsFn",
	"HIGg0Qfbpm5ThZ/RJfjoao4YwgT9/M",
	"u/PUD",
	"OlAm8tPkOF1qO5bXxRN2iFttuDldrg",
	"FFIiM6sFhWhU7tIMVUKOF7CUv/KzgwwV8FE",
	"yN",
	"4m5mglrIHksI6wYdq",
	"LXEfS7",
	"T+p+C+F2yjgsUtiXWU/cMNYEtJI4pq7GofW",
	"14BrGIEMXkbvFvZ49nDUfVCRcHYFOJ1BP1Y",
	"kWIH3Row",
	"RAmRTKNCjucPWC",
}

const (
	ThunderDriveSpace                 = ""
	ThunderDriveSafeSpace             = "SPACE_SAFE"
	ThunderBrowserDriveSpace          = "SPACE_BROWSER"
	ThunderBrowserDriveSafeSpace      = "SPACE_BROWSER_SAFE"
	ThunderDriveFolderType            = "DEFAULT_ROOT"
	ThunderBrowserDriveSafeFolderType = "BROWSER_SAFE"
)

const (
	FOLDER    = "drive#folder"
	FILE      = "drive#file"
	RESUMABLE = "drive#resumable"
	FILELIST  = "drive#fileList"
	TASK      = "drive#task"
)

const (
	UploadTypeUnknown   = "UPLOAD_TYPE_UNKNOWN"
	UploadTypeForm      = "UPLOAD_TYPE_FORM"
	UploadTypeResumable = "UPLOAD_TYPE_RESUMABLE"
	UploadTypeUrl       = "UPLOAD_TYPE_URL"
)

const (
	PhaseTypeError    = "PHASE_TYPE_ERROR"
	PhaseTypeRunning  = "PHASE_TYPE_RUNNING"
	PhaseTypePending  = "PHASE_TYPE_PENDING"
	PhaseTypeComplete = "PHASE_TYPE_COMPLETE"
)

const (
	TaskTypeOffline       = "offline"
	TaskTypeMove          = "move"
	TaskTypeUpload        = "upload"
	TaskTypeEventDeletion = "event-deletion"
	TaskTypeDeleteFile    = "deletefile"
)

const (
	QuotaCreateOfflineTaskLimit = "CREATE_OFFLINE_TASK_LIMIT"
)
