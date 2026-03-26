package pan

import (
	"sync"
)

var (
	driverMu             sync.RWMutex
	driverConstructorMap = map[DriverType]DriverConstructor{}
	idMap                = map[string]Driver{}
	defaultDriverMap     = map[DriverType]string{}
)

func RegisterDriver(driverType DriverType, driver DriverConstructor) {
	driverMu.Lock()
	defer driverMu.Unlock()
	driverConstructorMap[driverType] = driver
}

func GetDriverConstructor(driverType DriverType) (DriverConstructor, bool) {
	driverMu.RLock()
	defer driverMu.RUnlock()
	c, ok := driverConstructorMap[driverType]
	return c, ok
}

func StoreDriver(id string, driver Driver) {
	driverMu.Lock()
	defer driverMu.Unlock()
	idMap[id] = driver
}

func LoadDriver(id string) (Driver, bool) {
	driverMu.RLock()
	defer driverMu.RUnlock()
	d, ok := idMap[id]
	return d, ok
}

func SetDefaultDriver(driverType DriverType, id string) {
	driverMu.Lock()
	defer driverMu.Unlock()
	defaultDriverMap[driverType] = id
}

func GetDefaultDriverId(driverType DriverType) string {
	driverMu.RLock()
	defer driverMu.RUnlock()
	return defaultDriverMap[driverType]
}

func RemoveDriver(id string) {
	driverMu.Lock()
	defer driverMu.Unlock()
	if d, ok := idMap[id]; ok {
		_ = d.Close()
		delete(idMap, id)
	}
	for dt, did := range defaultDriverMap {
		if did == id {
			delete(defaultDriverMap, dt)
		}
	}
}
