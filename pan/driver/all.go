package driver

import (
	_ "github.com/hefy27/pan-client/pan/driver/aliyundrive"
	_ "github.com/hefy27/pan-client/pan/driver/aliyundrive_open"
	_ "github.com/hefy27/pan-client/pan/driver/baidu_netdisk"
	_ "github.com/hefy27/pan-client/pan/driver/cloudreve"
	_ "github.com/hefy27/pan-client/pan/driver/quark"
	_ "github.com/hefy27/pan-client/pan/driver/thunder_browser"
)

// All do nothing,just for import
// same as _ import
func All() {
	//do nothing ,only import
}
