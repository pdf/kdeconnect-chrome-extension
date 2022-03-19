//go:build !windows
// +build !windows

package main

import "github.com/godbus/dbus/v5"

func DBusSessionBusForPlatform() (conn *dbus.Conn, err error) {
	return dbus.SessionBus()
}
