//go:generate enumer -type=messageType -json

package main

type messageType int

const (
	typeDevices messageType = iota
	typeShare
	typeDeviceUpdate
	typeError
	typeVersion
)

type messageShare struct {
	Target string `json:"target"`
	URL    string `json:"url"`
}
