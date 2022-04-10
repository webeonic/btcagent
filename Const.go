package main

import (

)

// AuthorizeStat 认证状态
type AuthorizeStat uint8

const (
	StatConnected AuthorizeStat = iota
	StatSubScribed
	StatAuthorized
	StatDisconnected
	StatExit
)

// Stratum协议类型
type StratumProtocol uint8

const (
	// 未知协议
	ProtocolUnknown StratumProtocol = iota
	// ETHProxy 协议
	ProtocolETHProxy
	// NiceHash 的 EthereumStratum/1.0.0 协议
	ProtocolEthereumStratum
	// 传统 Stratum 协议
	ProtocolLegacyStratum
)


const DownSessionChannelCache uint = 64
const UpSessionChannelCache uint = 512
const UpSessionManagerChannelCache uint = 64
const SessionManagerChannelCache uint = 64

const UpSessionDialTimeoutSeconds Seconds = 15
const UpSessionReadTimeoutSeconds Seconds = 60

//btccom-agent/2.0.0-mu
const UpSessionUserAgent = ""

const DefaultWorkerName = "__default__"
const DefaultIpWorkerNameFormat = "{1}x{2}x{3}x{4}"

// UpSessionNumPerSubAccount Number of mine connections for each child account
const UpSessionNumPerSubAccount uint8 = 5

const (
	CapVersionRolling = "verrol" // ASICBoost version rolling
	CapSubmitResponse = "subres" // Send response of mining.submit
)

const DownSessionDisconnectWhenLostAsicboost = true
const UpSessionTLSInsecureSkipVerify = true

const FakeJobNotifyIntervalSeconds Seconds = 30

var FakeJobIDETHPrefixBin = []byte{
	0xfa, 0x6e, 0x07, 0x0b, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

