package main

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/golang/glog"
)

type DownSessionBTC struct {
	id string // Connection identifier for printing logs

	manager   *SessionManager // Session manager
	upSession EventInterface  // Server session

	sessionID       uint16        // Session ID
	clientConn      net.Conn      // TCP connection to the mine machine
	clientReader    *bufio.Reader // Read the content sent by the mine machine
	readLoopRunning bool          // Is the TCP read loop running
	stat            AuthorizeStat // Certification status

	clientAgent    string // Mining software name
	fullName       string // Complete miner name
	subAccountName string // Sub-account name
	workerName     string // Mining machine name
	versionMask    uint32 // Bitcoin version mask(Asicboost)

	eventLoopRunning bool             // Whether the message loop is running
	eventChannel     chan interface{} // Message channel

	versionRollingShareCounter uint64 // ASICBoost share Quantity
}

// NewDownSessionBTC Create a new STRATUM session
func NewDownSessionBTC(manager *SessionManager, clientConn net.Conn, sessionID uint16) (down *DownSessionBTC) {
	down = new(DownSessionBTC)
	down.manager = manager
	down.sessionID = sessionID
	down.clientConn = clientConn
	down.clientReader = bufio.NewReader(clientConn)
	down.stat = StatConnected
	down.eventChannel = make(chan interface{}, manager.config.Advanced.MessageQueueSize.MinerSession)

	down.id = fmt.Sprintf("miner#%d (%s) ", down.sessionID, down.clientConn.RemoteAddr())

	glog.Info(down.id, "miner connected")
	return
}

func (down *DownSessionBTC) SessionID() uint16 {
	return down.sessionID
}

func (down *DownSessionBTC) SubAccountName() string {
	return down.subAccountName
}

func (down *DownSessionBTC) Stat() AuthorizeStat {
	return down.stat
}

func (down *DownSessionBTC) Init() {
	go down.handleRequest()
	down.handleEvent()
}

func (down *DownSessionBTC) Run() {
	down.handleEvent()
}

func (down *DownSessionBTC) close() {
	if down.upSession != nil && down.stat != StatExit {
		go down.upSession.SendEvent(EventDownSessionBroken{down.sessionID})
	}

	down.eventLoopRunning = false
	down.stat = StatDisconnected
	down.clientConn.Close()

	// release down id
	down.manager.sessionIDManager.FreeSessionID(down.sessionID)
}

func (down *DownSessionBTC) writeJSONResponse(jsonData *JSONRPCResponse) (int, error) {
	bytes, err := jsonData.ToJSONBytesLine()
	if err != nil {
		return 0, err
	}
	if glog.V(12) {
		glog.Info(down.id, "writeJSONResponse: ", string(bytes))
	}
	return down.clientConn.Write(bytes)
}

func (down *DownSessionBTC) stratumHandleRequest(request *JSONRPCLineBTC, requestJSON []byte) (result interface{}, err *StratumError) {
	glog.Info("DownStratumHandleRequest. request: ", *request, ", requestJSON: ", string(requestJSON))
	switch request.Method {
	case "mining.subscribe":
		if down.stat != StatConnected {
			err = StratumErrDuplicateSubscribed
			return
		}
		result, err = down.parseSubscribeRequest(request)
		if err == nil {
			down.stat = StatSubScribed
		}
		return

	case "mining.authorize":
		if down.stat != StatSubScribed {
			err = StratumErrNeedSubscribed
			return
		}
		result, err = down.parseAuthorizeRequest(request)
		if err == nil {
			down.stat = StatAuthorized
			//Let the init () function returns
			down.eventLoopRunning = false

			down.id += fmt.Sprintf("<%s> ", down.fullName)

			glog.Info(down.id, "miner authorized")
		}
		return

	case "mining.configure":
		result, err = down.parseConfigureRequest(request)
		return

	case "mining.submit":
		result, err = down.parseMiningSubmit(request)
		if err != nil {
			glog.Warning(down.id, "stratum error: ", err, "; ", string(requestJSON))
		}
		return

	// ignore unimplemented methods
	case "mining.multi_version":
		fallthrough
	case "mining.suggest_difficulty":
		// If no response, the miner may wait indefinitely
		err = StratumErrIllegalParams
		return

	default:
		glog.Warning(down.id, "unknown request: ", string(requestJSON))

		// If no response, the miner may wait indefinitely
		err = StratumErrIllegalParams
		return
	}
}

func (down *DownSessionBTC) parseMiningSubmit(request *JSONRPCLineBTC) (result interface{}, err *StratumError) {
	glog.Info("(down *DownSessionBTC) parseMiningSubmit. request: ", request)
	if down.stat != StatAuthorized {
		err = StratumErrNeedAuthorized

		// there must be something wrong, send reconnect command
		down.sendReconnectRequest()
		return
	}

	if down.upSession == nil {
		err = StratumErrJobNotFound
		return
	}

	// params:
	// [0] Worker Name
	// [1] Job ID
	// [2] ExtraNonce2
	// [3] Time
	// [4] Nonce
	// [5] Version Mask

	if len(request.Params) < 5 {
		err = StratumErrTooFewParams
		return
	}

	var msg ExMessageSubmitShareBTC

	// [1] Job ID

	jobIDStr, ok := request.Params[1].(string)
	if !ok {
		err = StratumErrIllegalParams
		return
	}

	if IsFakeJobIDBTC(jobIDStr) {
		msg.IsFakeJob = true
	} else {
		msg.Base.JobID = jobIDStr
	}

	// [2] ExtraNonce2
	extraNonce2Hex, ok := request.Params[2].(string)
	if !ok {
		err = StratumErrIllegalParams
		return
	}
	//extraNonce, convErr := strconv.ParseUint(extraNonce2Hex, 16, 32)
	//if convErr != nil {
	//	err = StratumErrIllegalParams
	//	return
	//}
	//msg.Base.ExtraNonce2 = uint32(extraNonce)
	msg.Base.ExtraNonce2 = extraNonce2Hex

	// [3] Time
	timeHex, ok := request.Params[3].(string)
	if !ok {
		err = StratumErrIllegalParams
		return
	}
	// time, convErr := strconv.ParseUint(timeHex, 16, 32)
	// if convErr != nil {
	// 	err = StratumErrIllegalParams
	// 	return
	// }
	// msg.Time = uint32(time)
	msg.Time = timeHex

	// [4] Nonce
	nonceHex, ok := request.Params[4].(string)
	if !ok {
		err = StratumErrIllegalParams
		return
	}
	// nonce, convErr := strconv.ParseUint(nonceHex, 16, 32)
	// if convErr != nil {
	// 	err = StratumErrIllegalParams
	// 	return
	// }
	// msg.Base.Nonce = uint32(nonce)
	msg.Base.Nonce = nonceHex

	// [5] Version Mask
	hasVersionMask := false
	if len(request.Params) >= 6 {
		versionMaskHex, ok := request.Params[5].(string)
		if !ok {
			err = StratumErrIllegalParams
			return
		}
		versionMask, convErr := strconv.ParseUint(versionMaskHex, 16, 32)
		if convErr != nil {
			err = StratumErrIllegalParams
			return
		}
		msg.VersionMask = uint32(versionMask)
		hasVersionMask = true
	}

	// down id
	msg.Base.SessionID = down.sessionID

	go down.upSession.SendEvent(EventSubmitShareBTC{request.ID, &msg})

	// If AsicBoost is lost, send a reconnection request
	if down.manager.config.DisconnectWhenLostAsicboost {
		if hasVersionMask {
			down.versionRollingShareCounter++
		} else if down.versionRollingShareCounter > 100 {
			glog.Warning(down.id, "AsicBoost disabled mid-way after ", down.versionRollingShareCounter, " shares, send client.reconnect")

			// send reconnect request to miner
			down.sendReconnectRequest()
		}
	}
	return
}

func (down *DownSessionBTC) sendReconnectRequest() {
	var reconnect JSONRPCRequest
	reconnect.Method = "client.reconnect"
	reconnect.Params = JSONRPCArray{}
	bytes, err := reconnect.ToJSONBytesLine()
	if err != nil {
		glog.Error(down.id, "failed to convert client.reconnect request to JSON: ", err.Error(), "; ", reconnect)
		return
	}
	go down.SendEvent(EventSendBytes{bytes})
}

func (down *DownSessionBTC) parseSubscribeRequest(request *JSONRPCLineBTC) (result interface{}, err *StratumError) {

	if len(request.Params) >= 1 {
		down.clientAgent, _ = request.Params[0].(string)
	}

	sessionIDString := Uint32ToHex(uint32(down.sessionID))

	result = JSONRPCArray{JSONRPCArray{JSONRPCArray{"mining.set_difficulty", sessionIDString}, JSONRPCArray{"mining.notify", sessionIDString}}, sessionIDString, 4}
	return
}

func (down *DownSessionBTC) parseAuthorizeRequest(request *JSONRPCLineBTC) (result interface{}, err *StratumError) {
	if len(request.Params) < 1 {
		err = StratumErrTooFewParams
		return
	}

	fullWorkerName, ok := request.Params[0].(string)

	if !ok {
		err = StratumErrWorkerNameMustBeString
		return
	}

	// Miner name
	down.fullName = FilterWorkerName(fullWorkerName)

	// Intercepted "." Before the child account name, "." And after the mine machine name
	pos := strings.IndexByte(down.fullName, '.')
	if pos >= 0 {
		down.subAccountName = down.fullName[:pos]
		down.workerName = down.fullName[pos+1:]
	} else {
		down.subAccountName = down.fullName
		down.workerName = ""
	}

	if len(down.manager.config.FixedWorkerName) > 0 {
		down.workerName = down.manager.config.FixedWorkerName
		down.fullName = down.subAccountName + "." + down.workerName
	} else if down.manager.config.UseIpAsWorkerName {
		down.workerName = IPAsWorkerName(down.manager.config.IpWorkerNameFormat, down.clientConn.RemoteAddr().String())
		down.fullName = down.subAccountName + "." + down.workerName
	}

	if down.manager.config.MultiUserMode {
		if len(down.subAccountName) < 1 {
			err = StratumErrSubAccountNameEmpty
			return
		}
	} else {
		down.subAccountName = ""
	}

	if down.workerName == "" {
		down.workerName = down.fullName
		if down.workerName == "" {
			down.workerName = DefaultWorkerName
			down.fullName = down.subAccountName + "." + down.workerName
		}
	}

	//Get successful mine machine name
	result = true
	err = nil
	return
}

func (down *DownSessionBTC) parseConfigureRequest(request *JSONRPCLineBTC) (result interface{}, err *StratumError) {
	// request:
	//		{"id":3,"method":"mining.configure","params":[["version-rolling"],{"version-rolling.mask":"1fffe000","version-rolling.min-bit-count":2}]}
	// response:
	//		{"id":3,"result":{"version-rolling":true,"version-rolling.mask":"1fffe000"},"error":null}
	//		{"id":null,"method":"mining.set_version_mask","params":["1fffe000"]}

	if len(request.Params) < 2 {
		err = StratumErrTooFewParams
		return
	}

	if options, ok := request.Params[1].(map[string]interface{}); ok {
		if obj, ok := options["version-rolling.mask"]; ok {
			if versionMaskStr, ok := obj.(string); ok {
				versionMask, err := strconv.ParseUint(versionMaskStr, 16, 32)
				if err == nil {
					down.versionMask = uint32(versionMask)
				}
			}
		}
	}

	if down.versionMask != 0 {
		// The response here is a false version mask.After connecting the server mining.set_version_mask
		// Update to a real version mask.
		result = JSONRPCObj{
			"version-rolling":      true,
			"version-rolling.mask": down.versionMaskStr()}
		return
	}

	// Unknown configuration content, no response
	return
}

func (down *DownSessionBTC) versionMaskStr() string {
	return fmt.Sprintf("%08x", down.versionMask)
}

func (down *DownSessionBTC) setUpSession(e EventSetUpSession) {
	down.upSession = e.Session
	down.upSession.SendEvent(EventAddDownSession{down})
}

func (down *DownSessionBTC) handleRequest() {
	down.readLoopRunning = true

	for down.readLoopRunning {
		jsonBytes, err := down.clientReader.ReadBytes('\n')
		if err != nil {
			glog.Error(down.id, "failed to read request from miner: ", err.Error())
			down.connBroken()
			return
		}
		if glog.V(11) {
			glog.Info(down.id, "handleRequest: ", string(jsonBytes))
		}

		rpcData, err := NewJSONRPCLineBTC(jsonBytes)

		// ignore the json decode error
		if err != nil {
			glog.Warning(down.id, "failed to decode JSON from miner: ", err.Error(), "; ", string(jsonBytes))
		}

		down.SendEvent(EventRecvJSONRPCBTC{rpcData, jsonBytes})
	}
}

func (down *DownSessionBTC) recvJSONRPC(e EventRecvJSONRPCBTC) {
	// stat will be changed in stratumHandleRequest
	result, stratumErr := down.stratumHandleRequest(e.RPCData, e.JSONBytes)

	// Both are empty, there is no response you want to return.
	if result != nil || stratumErr != nil {
		var response JSONRPCResponse
		response.ID = e.RPCData.ID
		response.Result = result
		response.Error = stratumErr.ToJSONRPCArray(nil)

		glog.Info(fmt.Sprintf("recvJSONRPC response: %v", response))
		_, err := down.writeJSONResponse(&response)

		if err != nil {
			glog.Error(down.id, "failed to send response to miner: ", err.Error())
			down.close()
			return
		}
	}
}

func (down *DownSessionBTC) SendEvent(event interface{}) {
	down.eventChannel <- event
}

func (down *DownSessionBTC) connBroken() {
	down.readLoopRunning = false
	down.SendEvent(EventConnBroken{})
}

func (down *DownSessionBTC) sendBytes(e EventSendBytes) {
	if glog.V(12) {
		glog.Info(down.id, "sendBytes: down.clientConn address:", down.clientConn.RemoteAddr().String(), " e.Content: ", string(e.Content))
	}
	_, err := down.clientConn.Write(e.Content)
	if err != nil {
		glog.Error(down.id, "failed to send notify to miner: ", err.Error())
		down.close()
	}
}

func (down *DownSessionBTC) submitResponse(e EventSubmitResponse) {
	var response JSONRPCResponse
	response.ID = e.ID
	if e.Status.IsAccepted() {
		response.Result = true
	} else {
		response.Error = e.Status.ToJSONRPCArray(nil)
	}

	_, err := down.writeJSONResponse(&response)
	if err != nil {
		glog.Error(down.id, "failed to send share response to miner: ", err.Error())
		down.close()
	}
}

func (down *DownSessionBTC) exit() {
	down.stat = StatExit
	down.close()
}

func (down *DownSessionBTC) poolNotReady() {
	glog.Warning(down.id, "pool connection not ready")
	down.exit()
}

func (down *DownSessionBTC) handleEvent() {
	down.eventLoopRunning = true
	for down.eventLoopRunning {
		event := <-down.eventChannel

		switch e := event.(type) {
		case EventSetUpSession:
			down.setUpSession(e)
		case EventRecvJSONRPCBTC:
			down.recvJSONRPC(e)
		case EventSendBytes:
			down.sendBytes(e)
		case EventSubmitResponse:
			down.submitResponse(e)
		case EventConnBroken:
			down.close()
		case EventExit:
			down.exit()
		case EventPoolNotReady:
			down.poolNotReady()
		default:
			glog.Error(down.id, "unknown event: ", e)
		}
	}
}
