package main

import (
	"bytes"
	"encoding/binary"

	"github.com/golang/glog"
)

// ex-message的magic number
const ExMessageMagicNumber uint8 = 0x7F

// types
const (
	CMD_REGISTER_WORKER            uint8 = 0x01 // Agent -> Pool
	CMD_SUBMIT_SHARE               uint8 = 0x02 // Agent -> Pool,  mining.submit(...)
	CMD_SUBMIT_SHARE_WITH_TIME     uint8 = 0x03 // Agent -> Pool,  mining.submit(..., nTime)
	CMD_UNREGISTER_WORKER          uint8 = 0x04 // Agent -> Pool
	CMD_MINING_SET_DIFF            uint8 = 0x05 // Pool  -> Agent, mining.set_difficulty(diff)
	CMD_SUBMIT_RESPONSE            uint8 = 0x10 // Pool  -> Agent, response of the submit (optional)
	CMD_SUBMIT_SHARE_WITH_VER      uint8 = 0x12 // Agent -> Pool,  mining.submit(..., nVersionMask)
	CMD_SUBMIT_SHARE_WITH_TIME_VER uint8 = 0x13 // Agent -> Pool,  mining.submit(..., nTime, nVersionMask)
	CMD_SUBMIT_SHARE_WITH_MIX_HASH uint8 = 0x14 // Agent -> Pool, for ETH
	CMD_SET_EXTRA_NONCE            uint8 = 0x22 // Pool  -> Agent, pool nonce prefix allocation result (Ethereum)
)

type SerializableExMessage interface {
	Serialize() []byte
}

type UnserializableExMessage interface {
	Unserialize(data []byte) (err error)
}

type ExMessageHeader struct {
	MagicNumber uint8
	Type        uint8
	Size        uint16
}

type ExMessage struct {
	ExMessageHeader
	Body []byte
}

type ExMessageRegisterWorker struct {
	SessionID   uint16
	ClientAgent string
	WorkerName  string
}

func (msg *ExMessageRegisterWorker) Serialize() []byte {
	header := ExMessageHeader{
		ExMessageMagicNumber,
		CMD_REGISTER_WORKER,
		uint16(4 + 2 + len(msg.ClientAgent) + 1 + len(msg.WorkerName) + 1)}

	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, &header)
	binary.Write(buf, binary.LittleEndian, msg.SessionID)
	buf.WriteString(msg.ClientAgent)
	buf.WriteByte(0)
	buf.WriteString(msg.WorkerName)
	buf.WriteByte(0)

	glog.Info("ExMessageRegisterWorker buf:", buf.String(), "header:", header, "msg.Base:", msg)

	return buf.Bytes()
}

type ExMessageUnregisterWorker struct {
	SessionID uint16
}

func (msg *ExMessageUnregisterWorker) Serialize() []byte {
	header := ExMessageHeader{
		ExMessageMagicNumber,
		CMD_UNREGISTER_WORKER,
		uint16(4 + 2)}

	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, &header)
	binary.Write(buf, binary.LittleEndian, msg.SessionID)

	return buf.Bytes()
}

type ExMessageSubmitShareBTC struct {
	Base struct {
		JobID       string
		SessionID   uint16
		ExtraNonce2 string
		Nonce       string
	}

	Time        string
	VersionMask uint32

	IsFakeJob bool
}
