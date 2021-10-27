package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	dest = `org.kde.kdeconnect`
	path = `/modules/kdeconnect`

	signalReachableStatusChanged = dest + `.device.reachableStatusChanged`
	signalStateChanged           = dest + `.device.stateChanged`
	signalTrustedChanged         = dest + `.device.trustedChanged`
	signalNameChanged            = dest + `.device.nameChanged`
	signalPluginsChanged         = dest + `.device.pluginsChanged`

	pluginShare = `kdeconnect_share`
)

type deviceList struct {
	devices map[string]*Device
	conn    *dbus.Conn
	sync.RWMutex
}

func (d *deviceList) get(id string) (*Device, bool) {
	d.RLock()
	defer d.RUnlock()
	dev, ok := d.devices[id]
	return dev, ok
}

func (d *deviceList) add(id string) error {
	if _, ok := d.get(id); ok {
		return fmt.Errorf("Device already exists: %s", id)
	}

	d.Lock()
	defer d.Unlock()
	dev, err := newDevice(id)
	if err != nil {
		return err
	}
	d.devices[id] = dev

	return nil
}

func (d *deviceList) delete(id string) {
	d.Lock()
	delete(d.devices, id)
	d.Unlock()
}

func (d *deviceList) all() map[string]*Device {
	d.RLock()
	defer d.RUnlock()
	return d.devices
}

func (d *deviceList) Close() error {
	err := d.conn.Close()
	for _, d := range d.devices {
		if e := d.Close(); err != nil {
			log(e)
			if err == nil {
				err = e
			}
		}
	}
	return err
}

// Device maps to the DBUS device interface
type Device struct {
	ID               string              `json:"id"`
	Type             string              `json:"type"`
	Name             string              `json:"name"`
	IconName         string              `json:"iconName"`
	StatusIconName   string              `json:"statusIconName"`
	IsReachable      bool                `json:"isReachable"`
	IsTrusted        bool                `json:"isTrusted"`
	SupportedPlugins map[string]struct{} `json:"supportedPlugins"`
	conn             *dbus.Conn
	obj              dbus.BusObject
	signal           chan *dbus.Signal
	sync.RWMutex
}

func (d *Device) watch() error {
	// kdeconnect < v1.2
	if err := d.addMatchSignal(`reachableStatusChanged`); err != nil {
		return err
	}
	// kdeconnect >= v1.2
	if err := d.addMatchSignal(`reachableChanged`); err != nil {
		return err
	}

	if err := d.addMatchSignal(`stateChanged`); err != nil {
		return err
	}
	if err := d.addMatchSignal(`trustedChanged`); err != nil {
		return err
	}
	if err := d.addMatchSignal(`nameChanged`); err != nil {
		return err
	}
	if err := d.addMatchSignal(`pluginsChanged`); err != nil {
		return err
	}

	d.conn.Signal(d.signal)
	go func() {
		for s := range d.signal {
			var err error
			switch s.Name {
			case signalReachableStatusChanged:
				if err = d.getIsReachable(); err != nil {
					log(err)
				}
			case signalPluginsChanged:
				if err = d.getSupportedPlugins(); err != nil {
					log(err)
				}
			case signalNameChanged:
				if err = d.getName(); err != nil {
					log(err)
				}
			case signalTrustedChanged:
				if err = d.getIsTrusted(); err != nil {
					log(err)
				}
			case signalStateChanged:
				if err = d.update(); err != nil {
					log(err)
				}
			default:
				if err = d.update(); err != nil {
					log(err)
				}
			}
			update := &message{
				Type: typeDeviceUpdate,
			}
			if update.Data, err = json.Marshal(d); err != nil {
				log(err)
				continue
			}
			messageQueue <- update
		}
	}()

	return nil
}

func (d *Device) addMatchSignal(member string) error {
	call := d.conn.BusObject().Call(
		`org.freedesktop.DBus.AddMatch`,
		0,
		fmt.Sprintf("type='signal',path='%s',interface='%s.device',member='%s'", d.obj.Path(), dest, member),
	)
	return call.Err
}

func (d *Device) getType() error {
	v, err := d.obj.GetProperty(dest + `.device.type`)
	if err != nil {
		return err
	}
	d.Type = strings.Trim(v.String(), `"`)

	return nil
}

func (d *Device) getName() error {
	v, err := d.obj.GetProperty(dest + `.device.name`)
	if err != nil {
		return err
	}
	d.Name = strings.Trim(v.String(), `"`)

	return nil
}

func (d *Device) getIconName() error {
	v, err := d.obj.GetProperty(dest + `.device.iconName`)
	if err != nil {
		return err
	}
	d.IconName = strings.Trim(v.String(), `"`)

	return nil
}

func (d *Device) getStatusIconName() error {
	v, err := d.obj.GetProperty(dest + `.device.statusIconName`)
	if err != nil {
		return err
	}
	d.StatusIconName = strings.Trim(v.String(), `"`)

	return nil
}

func (d *Device) getIsReachable() error {
	v, err := d.obj.GetProperty(dest + `.device.isReachable`)
	if err != nil {
		return err
	}
	d.IsReachable = v.Value().(bool)

	return nil
}

func (d *Device) getIsTrusted() error {
	v, err := d.obj.GetProperty(dest + `.device.isTrusted`)
	if err != nil {
		return err
	}
	d.IsTrusted = v.Value().(bool)

	return nil
}

func (d *Device) getSupportedPlugins() error {
	v, err := d.obj.GetProperty(dest + `.device.supportedPlugins`)
	if err != nil {
		return err
	}
	plugins := make(map[string]struct{})
	for _, plugin := range v.Value().([]string) {
		plugins[plugin] = struct{}{}
	}
	d.Lock()
	d.SupportedPlugins = plugins
	d.Unlock()

	return nil
}

func (d *Device) update() error {
	if err := d.getName(); err != nil {
		logBadProp(d.ID, `name`, err)
	}
	if err := d.getType(); err != nil {
		logBadProp(d.ID, `type`, err)
	}
	if err := d.getIconName(); err != nil {
		logBadProp(d.ID, `iconName`, err)
	}
	if err := d.getStatusIconName(); err != nil {
		logBadProp(d.ID, `statusIconName`, err)
	}
	if err := d.getIsTrusted(); err != nil {
		logBadProp(d.ID, `isTrusted`, err)
	}
	if err := d.getIsReachable(); err != nil {
		logBadProp(d.ID, `isReachable`, err)
	}
	if err := d.getSupportedPlugins(); err != nil {
		logBadProp(d.ID, `supportedPlugins`, err)
	}

	return nil
}

func (d *Device) share(url string) error {
	if err := d.supported(pluginShare); err != nil {
		return err
	}
	return d.conn.Object(dest, d.obj.Path()+`/share`).Call(`shareUrl`, 0, url).Err
}

func (d *Device) supported(plugin string) error {
	d.RLock()
	defer d.RUnlock()
	if _, ok := d.SupportedPlugins[plugin]; !ok {
		return fmt.Errorf("Device does not currently support %s", plugin)
	}
	if !d.IsReachable {
		return fmt.Errorf("Device is not reachable")
	}
	if !d.IsTrusted {
		return fmt.Errorf("Device is not trusted")
	}

	return nil
}

// Close cleans up device signals, and removes it from the global list
func (d *Device) Close() error {
	devices.delete(d.ID)
	d.conn.RemoveSignal(d.signal)
	return d.conn.Close()
}

func (d *deviceList) getDevices() error {
	var ids []string

	obj := d.conn.Object(dest, path)
	// Find known devices, include unreachable, but exclude unpaired
	if err := obj.Call(`devices`, 0, false, true).Store(&ids); err != nil {
		return err
	}

	for _, id := range ids {
		var err error
		if _, ok := d.get(id); ok {
			continue
		}
		err = d.add(id)
		if err != nil {
			log(err)
			continue
		}
	}

	return nil
}

func logBadProp(id, prop string, err error) {
	log(fmt.Errorf("Device %s missing property (%s): %v\n", id, prop, err))
}

func newDevice(id string) (*Device, error) {
	conn, err := dbus.SessionBus()
	obj := conn.Object(dest, dbus.ObjectPath(fmt.Sprintf("%s/devices/%s", path, id)))
	if err != nil {
		return nil, err
	}
	d := &Device{
		ID:     id,
		conn:   conn,
		obj:    obj,
		signal: make(chan *dbus.Signal, 10),
	}
	if err = d.update(); err != nil {
		return nil, err
	}

	if err = d.watch(); err != nil {
		return nil, err
	}

	return d, nil
}

func newDeviceList() (*deviceList, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}
	return &deviceList{
		devices: make(map[string]*Device),
		conn:    conn,
	}, nil
}
