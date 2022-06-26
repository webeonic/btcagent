package main

// AuthorizeStat 认证状态
type AuthorizeStat uint8

const (
	StatConnected AuthorizeStat = iota
	StatSubScribed
	StatAuthorized
	StatDisconnected
	StatExit
)

// Stratum protocol type
type StratumProtocol uint8

const (
	// unknown protocol
	ProtocolUnknown StratumProtocol = iota
	// ETHProxy protocol
	ProtocolETHProxy
	// NiceHash's EthereumStratum1.0.0 protocol
	ProtocolEthereumStratum
	// Legacy Stratum Protocol
	ProtocolLegacyStratum
)

const DownSessionChannelCache uint = 64
const UpSessionChannelCache uint = 512
const UpSessionManagerChannelCache uint = 64
const SessionManagerChannelCache uint = 64

const UpSessionDialTimeoutSeconds Seconds = 15
const UpSessionReadTimeoutSeconds Seconds = 60

//btccom-agent/2.0.0-mu
const UpSessionUserAgent = "oktapool-agent"

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
