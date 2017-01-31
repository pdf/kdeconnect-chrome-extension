package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/godbus/dbus"
)

const (
	version = `0.0.1`
)

var (
	messageQueue = make(chan *message, 10)
	devices      = newDeviceList()
	installFlag  bool
)

type message struct {
	ID   string          `json:"id,omitempty"`
	Type messageType     `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

func writePump(ch <-chan *message) {
	var (
		enc = newEncoder(os.Stdout)
		err error
	)

	for msg := range ch {
		switch msg.Type {
		case typeDevices:
			out := &message{
				ID:   msg.ID,
				Type: typeDevices,
			}
			if out.Data, err = json.Marshal(devices.all()); err != nil {
				log(err)
				continue
			}
			if err = enc.Encode(out); err != nil {
				log(err)
				continue
			}
		case typeShare:
			share := &messageShare{}
			if err = json.Unmarshal(msg.Data, &share); err != nil {
				log(err)
				continue
			}
			if dev, ok := devices.get(share.Target); ok {
				if err = dev.share(share.URL); err != nil {
					log(err)
					continue
				}
			}
		case typeDeviceUpdate:
			if err = enc.Encode(msg); err != nil {
				log(err)
				continue
			}
		case typeVersion:
			out := &message{
				Type: typeVersion,
			}
			if out.Data, err = json.Marshal(version); err != nil {
				log(err)
				continue
			}
			if err = enc.Encode(out); err != nil {
				log(err)
				continue
			}
		case typeError:
			log(fmt.Errorf("typeError: %+v", msg))
		default:
			log(fmt.Errorf("Unhandled message type: %+v", msg.Type))
		}
	}
}

func readPump(ch chan<- *message) {
	defer close(ch)
	dec := newDecoder(os.Stdin)
	for {
		msg := &message{}
		if err := dec.Decode(msg); err != nil {
			if err == io.EOF {
				return
			}
			log(err)
			continue
		}
		ch <- msg
	}
}

func log(err error) {
	if _, e := fmt.Fprintln(os.Stderr, err); e != nil {
		panic(e)
	}
}

func init() {
	flag.BoolVar(&installFlag, `install`, false, `Perform installation`)
	flag.Parse()
}

func main() {
	if installFlag {
		if err := install(); err != nil {
			panic(err)
		}
		os.Exit(0)
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}

	if err = getDevices(conn); err != nil {
		log(err)
	}

	go writePump(messageQueue)
	readPump(messageQueue)

	shutdown()
}

func shutdown() {
	if err := devices.Close(); err != nil {
		panic(err)
	}
}
