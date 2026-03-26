package pan_client

import (
	"context"
	"log/slog"
	"sync"

	"github.com/hefeiyu25/pan-client/internal"
	"github.com/hefeiyu25/pan-client/pan"
	"github.com/hefeiyu25/pan-client/pan/driver/aliyundrive"
	"github.com/hefeiyu25/pan-client/pan/driver/aliyundrive_open"
	"github.com/hefeiyu25/pan-client/pan/driver/baidu_netdisk"
	"github.com/hefeiyu25/pan-client/pan/driver/cloudreve"
	"github.com/hefeiyu25/pan-client/pan/driver/quark"
	"github.com/hefeiyu25/pan-client/pan/driver/thunder_browser"
)

// clientOptions holds optional settings for client creation.
type clientOptions struct {
	onChange       pan.OnChangeFunc
	ctx            context.Context
	downloadConfig pan.DownloadConfig
	proxyConfig    pan.ProxyConfig
}

// ClientOption configures a client.
type ClientOption func(*clientOptions)

// WithOnChange sets a callback that is invoked when driver properties change
// (e.g., token refresh, session update). The caller is responsible for persisting
// the updated properties however they see fit.
func WithOnChange(fn pan.OnChangeFunc) ClientOption {
	return func(o *clientOptions) {
		o.onChange = fn
	}
}

// WithContext sets a context for the client lifecycle.
func WithContext(ctx context.Context) ClientOption {
	return func(o *clientOptions) {
		o.ctx = ctx
	}
}

// WithDownloadTmpPath sets the temp directory for chunk downloads.
func WithDownloadTmpPath(p string) ClientOption {
	return func(o *clientOptions) {
		o.downloadConfig.TmpPath = p
	}
}

// WithDownloadMaxThread sets the max concurrent download goroutines.
func WithDownloadMaxThread(n int) ClientOption {
	return func(o *clientOptions) {
		o.downloadConfig.MaxThread = n
	}
}

// WithDownloadMaxRetry sets the max retry count per chunk.
func WithDownloadMaxRetry(n int) ClientOption {
	return func(o *clientOptions) {
		o.downloadConfig.MaxRetry = n
	}
}

// WithProxy sets the proxy URL for this driver instance.
// Supports http://host:port and socks5://host:port.
func WithProxy(proxyURL string) ClientOption {
	return func(o *clientOptions) {
		o.proxyConfig.ProxyURL = proxyURL
	}
}

func applyOpts(opts []ClientOption) (*clientOptions, context.CancelFunc) {
	o := &clientOptions{ctx: context.Background()}
	for _, opt := range opts {
		opt(o)
	}
	o.downloadConfig.ApplyDefaults()
	// derive a cancellable context so Close() can abort in-flight operations
	ctx, cancel := context.WithCancel(o.ctx)
	o.ctx = ctx
	return o, cancel
}

// NewQuarkClient creates a new Quark driver from the given properties.
func NewQuarkClient(props quark.QuarkProperties, opts ...ClientOption) (pan.Driver, error) {
	o, cancel := applyOpts(opts)
	d := &quark.Quark{
		PropertiesOperate: pan.PropertiesOperate[*quark.QuarkProperties]{
			Properties: &props,
			DriverType: pan.Quark,
			OnChange:   o.onChange,
		},
		CacheOperate:  pan.NewCacheOperate(),
		CommonOperate: pan.CommonOperate{},
	}
	d.BaseOperate = newBaseOperate(o, cancel)
	internal.SetDefaultByTag(d.Properties)
	id, err := d.Init()
	if err != nil {
		cancel()
		return nil, err
	}
	pan.StoreDriver(id, d)
	return d, nil
}

// NewThunderClient creates a new ThunderBrowser driver from the given properties.
func NewThunderClient(props thunder_browser.ThunderBrowserProperties, opts ...ClientOption) (pan.Driver, error) {
	o, cancel := applyOpts(opts)
	d := &thunder_browser.ThunderBrowser{
		PropertiesOperate: pan.PropertiesOperate[*thunder_browser.ThunderBrowserProperties]{
			Properties: &props,
			DriverType: pan.ThunderBrowser,
			OnChange:   o.onChange,
		},
		CacheOperate:  pan.NewCacheOperate(),
		CommonOperate: pan.CommonOperate{},
	}
	d.BaseOperate = newBaseOperate(o, cancel)
	internal.SetDefaultByTag(d.Properties)
	id, err := d.Init()
	if err != nil {
		cancel()
		return nil, err
	}
	pan.StoreDriver(id, d)
	return d, nil
}

// NewCloudreveClient creates a new Cloudreve driver from the given properties.
func NewCloudreveClient(props cloudreve.CloudreveProperties, opts ...ClientOption) (pan.Driver, error) {
	o, cancel := applyOpts(opts)
	d := &cloudreve.Cloudreve{
		PropertiesOperate: pan.PropertiesOperate[*cloudreve.CloudreveProperties]{
			Properties: &props,
			DriverType: pan.Cloudreve,
			OnChange:   o.onChange,
		},
		CacheOperate:  pan.NewCacheOperate(),
		CommonOperate: pan.CommonOperate{},
	}
	d.BaseOperate = newBaseOperate(o, cancel)
	internal.SetDefaultByTag(d.Properties)
	id, err := d.Init()
	if err != nil {
		cancel()
		return nil, err
	}
	pan.StoreDriver(id, d)
	return d, nil
}

// NewAliyundriveOpenClient creates a new AliyundriveOpen driver from the given properties.
func NewAliyundriveOpenClient(props aliyundrive_open.AliyundriveOpenProperties, opts ...ClientOption) (pan.Driver, error) {
	o, cancel := applyOpts(opts)
	d := &aliyundrive_open.AliyundriveOpen{
		PropertiesOperate: pan.PropertiesOperate[*aliyundrive_open.AliyundriveOpenProperties]{
			Properties: &props,
			DriverType: pan.AliyundriveOpen,
			OnChange:   o.onChange,
		},
		CacheOperate:  pan.NewCacheOperate(),
		CommonOperate: pan.CommonOperate{},
	}
	d.BaseOperate = newBaseOperate(o, cancel)
	internal.SetDefaultByTag(d.Properties)
	id, err := d.Init()
	if err != nil {
		cancel()
		return nil, err
	}
	pan.StoreDriver(id, d)
	return d, nil
}

// NewAliyundriveClient creates a new AliDrive (unofficial API) driver from the given properties.
func NewAliyundriveClient(props aliyundrive.AliDriveProperties, opts ...ClientOption) (pan.Driver, error) {
	o, cancel := applyOpts(opts)
	d := &aliyundrive.AliDrive{
		PropertiesOperate: pan.PropertiesOperate[*aliyundrive.AliDriveProperties]{
			Properties: &props,
			DriverType: pan.Aliyundrive,
			OnChange:   o.onChange,
		},
		CacheOperate:  pan.NewCacheOperate(),
		CommonOperate: pan.CommonOperate{},
	}
	d.BaseOperate = newBaseOperate(o, cancel)
	internal.SetDefaultByTag(d.Properties)
	id, err := d.Init()
	if err != nil {
		cancel()
		return nil, err
	}
	pan.StoreDriver(id, d)
	return d, nil
}

// NewBaiduClient creates a new BaiduNetdisk driver from the given properties.
func NewBaiduClient(props baidu_netdisk.BaiduProperties, opts ...ClientOption) (pan.Driver, error) {
	o, cancel := applyOpts(opts)
	d := &baidu_netdisk.BaiduNetdisk{
		PropertiesOperate: pan.PropertiesOperate[*baidu_netdisk.BaiduProperties]{
			Properties: &props,
			DriverType: pan.BaiduNetdisk,
			OnChange:   o.onChange,
		},
		CacheOperate:  pan.NewCacheOperate(),
		CommonOperate: pan.CommonOperate{},
	}
	d.BaseOperate = newBaseOperate(o, cancel)
	internal.SetDefaultByTag(d.Properties)
	id, err := d.Init()
	if err != nil {
		cancel()
		return nil, err
	}
	pan.StoreDriver(id, d)
	return d, nil
}

func newBaseOperate(o *clientOptions, cancel context.CancelFunc) pan.BaseOperate {
	return pan.NewBaseOperate(o.downloadConfig, o.proxyConfig, o.ctx, cancel)
}

// RemoveDriver removes a cached driver by id.
func RemoveDriver(id string) {
	pan.RemoveDriver(id)
}

// --- Global initialization ---

var initOnce sync.Once

// Init initializes global settings. Safe to call multiple times; only the first call takes effect.
func Init(opts ...GlobalOption) {
	initOnce.Do(func() {
		cfg := defaultGlobalConfig()
		for _, opt := range opts {
			opt(cfg)
		}
		if cfg.logger != nil {
			internal.SetLogger(cfg.logger)
		}
	})
}

type globalConfig struct {
	logger *slog.Logger
}

func defaultGlobalConfig() *globalConfig {
	return &globalConfig{}
}

// GlobalOption configures global initialization.
type GlobalOption func(*globalConfig)

// WithLogger sets a custom slog.Logger for the library.
// If not set, slog.Default() is used.
func WithLogger(l *slog.Logger) GlobalOption {
	return func(g *globalConfig) {
		g.logger = l
	}
}

// GracefulExit cleanly shuts down background tasks.
func GracefulExit() {
	internal.Shutdown()
}
