package main

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"unsafe"
)

var nativeEndian binary.ByteOrder

func init() {
	var one int16 = 1
	b := (*byte)(unsafe.Pointer(&one))
	if *b == 0 {
		nativeEndian = binary.BigEndian
	} else {
		nativeEndian = binary.LittleEndian
	}
}

type encoder struct {
	w io.Writer
}

func newEncoder(w io.Writer) *encoder {
	return &encoder{w: w}
}

func (e *encoder) Encode(v interface{}) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	msgLen := uint32(len(buf))
	if err := binary.Write(e.w, nativeEndian, &msgLen); err != nil {
		return err
	}
	if _, err := e.w.Write(buf); err != nil {
		return err
	}
	return nil
}

type decoder struct {
	r io.Reader
}

func newDecoder(r io.Reader) *decoder {
	return &decoder{r: r}
}

func (d *decoder) Decode(v interface{}) error {
	var msgLen uint32
	if err := binary.Read(d.r, nativeEndian, &msgLen); err != nil {
		return err
	}
	buf := make([]byte, msgLen)
	if _, err := io.ReadFull(d.r, buf); err != nil {
		return err
	}
	return json.Unmarshal(buf, v)
}
