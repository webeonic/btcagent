package main

// ex-message的magic number

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
