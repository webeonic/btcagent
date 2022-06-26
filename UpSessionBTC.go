package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
)

type UpSessionBTC struct {
	id string // Connection identifier for printing logs

	manager *UpSessionManager
	config  *Config
	slot    int

	subAccount string
	poolIndex  int

	downSessions    map[uint16]*DownSessionBTC
	serverConn      net.Conn
	serverReader    *bufio.Reader
	readLoopRunning bool

	stat            AuthorizeStat
	sessionID       uint32
	versionMask     uint32
	extraNonce2Size int

	serverCapVersionRolling bool
	serverCapSubmitResponse bool

	eventLoopRunning bool
	eventChannel     chan interface{}

	lastJob           *StratumJobBTC
	rpcSetVersionMask []byte
	rpcSetDifficulty  []byte

	submitIDs   map[uint16]SubmitID
	submitIndex uint16

	// Used for statistics to disconnect the number of miners, and synchronize to UpsessionManager
	disconnectedMinerCounter int
}

func NewUpSessionBTC(manager *UpSessionManager, poolIndex int, slot int) (up *UpSessionBTC) {
	up = new(UpSessionBTC)
	up.manager = manager
	up.config = manager.config
	up.slot = slot
	up.subAccount = manager.subAccount
	up.poolIndex = poolIndex
	up.downSessions = make(map[uint16]*DownSessionBTC)
	up.stat = StatDisconnected
	up.eventChannel = make(chan interface{}, manager.config.Advanced.MessageQueueSize.PoolSession)
	up.submitIDs = make(map[uint16]SubmitID)

	if !up.config.MultiUserMode {
		up.subAccount = manager.config.Pools[poolIndex].SubAccount
	}

	return
}

func (up *UpSessionBTC) Stat() AuthorizeStat {
	return up.stat
}

func (up *UpSessionBTC) connect() {
	pool := up.config.Pools[up.poolIndex]
	url := fmt.Sprintf("%s:%d", pool.Host, pool.Port)

	if up.config.PoolUseTls {
		up.id = fmt.Sprintf("pool#%d <%s> [tls://%s] ", up.slot, up.subAccount, url)
	} else {
		up.id = fmt.Sprintf("pool#%d <%s> [%s] ", up.slot, up.subAccount, url)
	}

	// Try to connect to all proxies and find the fastest one
	counter := len(up.config.Proxy)
	for i := 0; i < counter; i++ {
		go up.tryConnect(pool.Host, url, up.config.Proxy[i])
	}
	if up.config.DirectConnectWithProxy {
		counter++
		go up.tryConnect(pool.Host, url, "")
	}

	// Receive connection event
	for i := 0; i < counter; i++ {
		event := <-up.eventChannel
		switch e := event.(type) {
		case EventUpSessionConnection:
			up.upSessionConnection(e)
			if up.stat == StatConnected {
				return
			}
		default:
			glog.Error(up.id, "unknown event: ", e)
		}
	}

	// No need to try directly
	if counter > 0 && !up.config.DirectConnectAfterProxy {
		return
	}

	// Try directly to
	go up.tryConnect(pool.Host, url, "")
	event := <-up.eventChannel
	switch e := event.(type) {
	case EventUpSessionConnection:
		up.upSessionConnection(e)
	default:
		glog.Error(up.id, "unknown event: ", e)
	}
}

func (up *UpSessionBTC) upSessionConnection(e EventUpSessionConnection) {
	if e.Error != nil {
		if len(e.ProxyURL) > 0 {
			glog.Warning(up.id, "proxy [", e.ProxyURL, "] failed: ", e.Error.Error())
		} else {
			glog.Warning(up.id, "direct connection failed: ", e.Error.Error())
		}

		if e.Conn != nil {
			e.Conn.Close()
		}
		return
	}

	up.serverConn = e.Conn
	up.serverReader = e.Reader
	up.stat = StatConnected
	up.id += fmt.Sprintf("(%s) ", up.serverConn.RemoteAddr().String())

	if len(e.ProxyURL) > 0 {
		glog.Info(up.id, "successfully connected with proxy [", e.ProxyURL, "]")
	} else {
		glog.Info(up.id, "successfully connected directly")
	}
}

func (up *UpSessionBTC) tryConnect(poolHost, poolURL, proxyURL string) {
	timeout := up.config.Advanced.PoolConnectionDialTimeoutSeconds.Get()
	insecureSkipVerify := up.config.Advanced.TLSSkipCertificateVerify

	var err error
	var dialer Dialer
	var conn net.Conn
	var reader *bufio.Reader

	if len(proxyURL) > 0 {
		glog.Info(up.id, "connect to pool server with proxy [", proxyURL, "]...")
		dialer, err = GetProxyDialer(proxyURL, timeout, insecureSkipVerify)
	} else {
		glog.Info(up.id, "connect to pool server directly...")
		dialer = &net.Dialer{Timeout: timeout}
	}

	if err == nil {
		conn, err = dialer.Dial("tcp", poolURL)
		if err == nil {
			if up.config.PoolUseTls {
				conn = tls.Client(conn, &tls.Config{
					ServerName:         poolHost,
					InsecureSkipVerify: insecureSkipVerify,
				})
			}
			reader, err = up.testConnection(conn)
		}
	}

	up.SendEvent(EventUpSessionConnection{proxyURL, conn, reader, err})
}

func (up *UpSessionBTC) testConnection(conn net.Conn) (reader *bufio.Reader, err error) {
	ch := make(chan error, 1)
	reader = bufio.NewReader(conn)

	go func() {
		// capsRequest := up.getAgentGetCapsRequest("conn_test")
		// bytes, e := capsRequest.ToJSONBytesLine()
		// if e == nil {
		// 	if glog.V(10) {
		// 		glog.Info(up.id, "testConnection send: ", string(bytes))
		// 	}
		// 	conn.SetWriteDeadline(up.getIODeadLine())
		// 	_, e = conn.Write(bytes)
		// 	if e == nil {
		// 		conn.SetReadDeadline(up.getIODeadLine())
		// 		bytes, e = reader.ReadBytes('\n')
		// 		if glog.V(9) {
		// 			glog.Info(up.id, "testConnection recv: ", string(bytes))
		// 		}
		// 	}
		// }
		//
		ch <- nil
	}()

	select {
	case <-time.After(up.config.Advanced.PoolConnectionDialTimeoutSeconds.Get()):
		err = errors.New("connection timeout")
		conn.Close()
	case err = <-ch:
	}

	return
}

func (up *UpSessionBTC) writeJSONRequest(jsonData *JSONRPCRequest) (int, error) {
	bytes, err := jsonData.ToJSONBytesLine()
	if err != nil {
		return 0, err
	}
	if glog.V(10) {
		glog.Info(up.id, "writeJSONRequest: ", string(bytes))
	}
	return up.writeBytes(bytes)
}

func (up *UpSessionBTC) writeBytes(bytes []byte) (int, error) {
	glog.Info("writeBytes before DeadLine: ", string(bytes))
	up.setWriteDeadline()
	return up.serverConn.Write(bytes)
}

func (up *UpSessionBTC) getAgentGetCapsRequest(id string) (req JSONRPCRequest) {
	req.ID = id
	req.Method = "mining.capabilities"
	if up.config.SubmitResponseFromServer {
		req.SetParams(JSONRPCArray{CapVersionRolling, CapSubmitResponse})
	} else {
		req.SetParams(JSONRPCArray{CapVersionRolling})
	}
	return
}

func (up *UpSessionBTC) sendInitRequest() (err error) {
	// send agent.get_capabilities first
	// capsRequest := up.getAgentGetCapsRequest("caps")
	// _, err = up.writeJSONRequest(&capsRequest)
	// if err != nil {
	// 	return
	// }

	// send configure request
	var request JSONRPCRequest
	request.ID = "conf"
	request.Method = "mining.configure"
	request.SetParams(JSONRPCArray{"version-rolling"}, JSONRPCObj{"version-rolling.mask": "ffffffff", "version-rolling.min-bit-count": 0})
	_, err = up.writeJSONRequest(&request)
	if err != nil {
		return
	}

	// send subscribe request
	request.ID = "sub"
	request.Method = "mining.subscribe"
	request.SetParams(UpSessionUserAgent)
	_, err = up.writeJSONRequest(&request)
	if err != nil {
		return
	}

	// send authorize request
	request.ID = "auth"
	request.Method = "mining.authorize"
	request.SetParams(up.subAccount, "123456")
	_, err = up.writeJSONRequest(&request)
	if err != nil {
		return
	}

	// send agent.get_capabilities again
	// fix subres (submit_response_from_server)
	// Subres negotiation must be sent after authentication, or sserver will not send the response.
	// capsRequest.ID = "caps_again"
	// _, err = up.writeJSONRequest(&capsRequest)
	return
}

func (up *UpSessionBTC) exit() {
	up.stat = StatExit
	up.close()
}

func (up *UpSessionBTC) close() {
	if up.stat == StatAuthorized {
		up.manager.SendEvent(EventUpSessionBroken{up.slot})
	}

	if up.config.AlwaysKeepDownconn {
		if up.lastJob != nil {
			up.manager.SendEvent(EventUpdateFakeJobBTC{up.lastJob})
		}
		for _, down := range up.downSessions {
			go up.manager.SendEvent(EventAddDownSession{down})
		}
	} else {
		for _, down := range up.downSessions {
			go down.SendEvent(EventExit{})
		}
	}

	up.eventLoopRunning = false
	up.stat = StatDisconnected
	up.serverConn.Close()
}

func (up *UpSessionBTC) Init() {
	up.connect()
	if up.stat != StatConnected {
		if len(up.config.Proxy) > 0 && (up.config.DirectConnectWithProxy || up.config.DirectConnectAfterProxy) {
			glog.Error(up.id, "all connections both proxy and direct failed")
		} else if len(up.config.Proxy) > 1 {
			glog.Error(up.id, "all proxy connections failed")
		}
		return
	}

	go up.handleResponse()

	err := up.sendInitRequest()
	if err != nil {
		glog.Error(up.id, "failed to send request to pool server: ", err.Error())
		up.close()
		return
	}

	up.handleEvent()
}

func (up *UpSessionBTC) handleSetVersionMask(rpcData *JSONRPCLineBTC, jsonBytes []byte) {
	up.rpcSetVersionMask = jsonBytes

	if len(rpcData.Params) > 0 {
		if up.serverCapVersionRolling {
			versionMaskHex, ok := rpcData.Params[0].(string)
			if !ok {
				glog.Error(up.id, "version mask is not a string: ", string(jsonBytes))
				return
			}
			versionMask, err := strconv.ParseUint(versionMaskHex, 16, 32)
			if err != nil {
				glog.Error(up.id, "version mask is not a hex: ", string(jsonBytes))
				return
			}
			up.versionMask = uint32(versionMask)

			if glog.V(1) {
				glog.Info(up.id, "AsicBoost via BTCAgent enabled, allowed version mask: ", versionMaskHex)
			}
		} else {
			// server doesn't support version rolling via BTCAgent
			up.versionMask = 0
			rpcData.Params[0] = "00000000"
		}
	}

	e := EventSendBytes{up.rpcSetVersionMask}
	for _, down := range up.downSessions {
		if down.versionMask != 0 {
			go down.SendEvent(e)
		}
	}
}

func (up *UpSessionBTC) handleSetDifficulty(rpcData *JSONRPCLineBTC, jsonBytes []byte) {
	if up.rpcSetDifficulty == nil {
		up.rpcSetDifficulty = jsonBytes

		e := EventSendBytes{up.rpcSetDifficulty}
		for _, down := range up.downSessions {
			go down.SendEvent(e)
		}
	}
}

func (up *UpSessionBTC) handleSubScribeResponse(rpcData *JSONRPCLineBTC, jsonBytes []byte) {
	result, ok := rpcData.Result.([]interface{})
	if !ok {
		glog.Error(up.id, "subscribe result is not an array: ", string(jsonBytes))
		up.close()
		return
	}
	if len(result) < 3 {
		glog.Error(up.id, "subscribe result missing items: ", string(jsonBytes))
		up.close()
		return
	}
	sessionIDHex, ok := result[1].(string)
	if !ok {
		glog.Error(up.id, "session id is not a string: ", string(jsonBytes))
		up.close()
		return
	}
	sessionID, err := strconv.ParseUint(sessionIDHex, 16, 32)
	if err != nil {
		glog.Error(up.id, "session id is not a hex: ", string(jsonBytes))
		up.close()
		return
	}
	up.sessionID = uint32(sessionID)

	extraNonce2SizeFloat, ok := result[2].(float64)
	if !ok {
		glog.Error(up.id, "extra nonce 2 size is not an integer: ", string(jsonBytes))
		up.close()
		return
	}
	up.extraNonce2Size = int(extraNonce2SizeFloat)
	if up.extraNonce2Size != 4 {
		glog.Error(up.id, "BTCAgent is not compatible with this server, extra nonce 2 should be 4 bytes but only ", up.extraNonce2Size, " bytes")
		up.close()
		return
	}
	up.stat = StatSubScribed
}

func (up *UpSessionBTC) handleConfigureResponse(rpcData *JSONRPCLineBTC, jsonBytes []byte) {
	// ignore
}

func (up *UpSessionBTC) handleGetCapsResponse(rpcData *JSONRPCLineBTC, jsonBytes []byte) {
	result, ok := rpcData.Result.(map[string]interface{})
	if !ok {
		glog.Error(up.id, "get server capabilities failed, result is not an object: ", string(jsonBytes))
	}
	caps, ok := result["capabilities"]
	if !ok {
		glog.Error(up.id, "get server capabilities failed, missing field capabilities: ", string(jsonBytes))
	}
	capsArr, ok := caps.([]interface{})
	if !ok {
		glog.Error(up.id, "get server capabilities failed, capabilities is not an array: ", string(jsonBytes))
	}
	for _, capability := range capsArr {
		switch capability {
		case CapVersionRolling:
			up.serverCapVersionRolling = true
		case CapSubmitResponse:
			up.serverCapSubmitResponse = true
		}
	}
	if !up.serverCapVersionRolling {
		glog.Warning(up.id, "[WARNING] pool server does not support ASICBoost")
	}
	if up.config.SubmitResponseFromServer {
		if up.serverCapSubmitResponse {
			if glog.V(1) {
				glog.Info(up.id, "pool server will send share response to BTCAgent")
			}
		} else {
			glog.Warning(up.id, "[WARNING] pool server does not support sendding share response to BTCAgent")
		}
	}
}

func (up *UpSessionBTC) handleAuthorizeResponse(rpcData *JSONRPCLineBTC, jsonBytes []byte) {
	result, ok := rpcData.Result.(bool)
	if !ok || !result {
		glog.Error(up.id, "authorize failed: ", rpcData.Error)
		up.close()
		return
	}
	glog.Info(up.id, "authorize success, session id: ", up.sessionID)
	up.stat = StatAuthorized
	//Let the init () function returns
	up.eventLoopRunning = false
}

func (up *UpSessionBTC) connBroken() {
	up.readLoopRunning = false
	up.SendEvent(EventConnBroken{})
}

func (up *UpSessionBTC) getIODeadLine() time.Time {
	var timeout Seconds
	if up.stat == StatAuthorized {
		timeout = up.config.Advanced.PoolConnectionReadTimeoutSeconds
	} else {
		timeout = up.config.Advanced.PoolConnectionDialTimeoutSeconds
	}
	return time.Now().Add(timeout.Get())
}

func (up *UpSessionBTC) setReadDeadline() {
	up.serverConn.SetReadDeadline(up.getIODeadLine())
}

func (up *UpSessionBTC) setWriteDeadline() {
	up.serverConn.SetWriteDeadline(up.getIODeadLine())
}

func (up *UpSessionBTC) handleResponse() {
	up.readLoopRunning = true
	for up.readLoopRunning {
		up.setReadDeadline()
		up.readLine()
	}
}

func (up *UpSessionBTC) readLine() {
	jsonBytes, err := up.serverReader.ReadBytes('\n')
	if err != nil {
		glog.Error(up.id, "failed to read JSON line from pool server: ", err.Error())
		up.connBroken()
		return
	}
	if glog.V(9) {
		glog.Info(up.id, "readLine: ", string(jsonBytes))
	}

	jsonBytes = []byte(strings.Replace(string(jsonBytes), ":auth", ":\"auth\"", 1))
	rpcData, err := NewJSONRPCLineBTC(jsonBytes)

	// ignore the json decode error
	if err != nil {
		glog.Error(up.id, "failed to decode JSON line from pool server: ", err.Error(), "; ", string(jsonBytes))
		return
	}

	up.SendEvent(EventRecvJSONRPCBTC{rpcData, jsonBytes})
}

func (up *UpSessionBTC) Run() {
	up.handleEvent()
}

func (up *UpSessionBTC) SendEvent(event interface{}) {
	up.eventChannel <- event
}

func (up *UpSessionBTC) addDownSession(e EventAddDownSession) {
	down := e.Session.(*DownSessionBTC)
	up.downSessions[down.sessionID] = down

	if up.rpcSetVersionMask != nil && down.versionMask != 0 {
		down.SendEvent(EventSendBytes{up.rpcSetVersionMask})
	}

	if up.rpcSetDifficulty != nil {
		down.SendEvent(EventSendBytes{up.rpcSetDifficulty})
	}

	if up.lastJob != nil {
		bytes, err := up.lastJob.ToNotifyLine(true)
		if err == nil {
			down.SendEvent(EventSendBytes{bytes})
		} else {
			glog.Warning(up.id, "failed to convert job to JSON: ", err.Error(), "; ", up.lastJob)
		}
	}
}

func (up *UpSessionBTC) handleMiningNotify(rpcData *JSONRPCLineBTC, jsonBytes []byte) {
	job, err := NewStratumJobBTC(rpcData, up.sessionID)
	if err != nil {
		glog.Warning(up.id, err.Error(), ": ", string(jsonBytes))
		return
	}

	bytes, err := job.ToNotifyLine(false)
	if err != nil {
		glog.Warning(up.id, "failed to convert job to JSON: ", err.Error(), "; ", string(jsonBytes))
		return
	}

	for _, down := range up.downSessions {
		go down.SendEvent(EventSendBytes{bytes})
	}

	up.lastJob = job
}

func (up *UpSessionBTC) recvJSONRPC(e EventRecvJSONRPCBTC) {
	rpcData := e.RPCData
	jsonBytes := e.JSONBytes

	glog.Info("UpSessionRecvJSONRPC. RPCData: ", *e.RPCData, ", JSONBytes: ", string(jsonBytes))

	if len(rpcData.Method) > 0 {
		glog.Info("rpcData.Method: ", rpcData.Method)
		switch rpcData.Method {
		case "mining.set_version_mask":
			up.handleSetVersionMask(rpcData, jsonBytes)
		case "mining.set_difficulty":
			up.handleSetDifficulty(rpcData, jsonBytes)
		case "mining.notify":
			up.handleMiningNotify(rpcData, jsonBytes)
		default:
			glog.Info(up.id, "[TODO] pool request: ", rpcData)
		}
		return
	}

	switch rpcData.ID {
	case "caps":
		up.handleGetCapsResponse(rpcData, jsonBytes)
	case "conf":
		up.handleConfigureResponse(rpcData, jsonBytes)
	case "sub":
		up.handleSubScribeResponse(rpcData, jsonBytes)
	case "auth":
		up.handleAuthorizeResponse(rpcData, jsonBytes)
	case "caps_again":
		// ignore
	case "conn_test":
		// ignore
	default:
		glog.Info(up.id, "[TODO] pool response: ", rpcData)
	}
}

func (up *UpSessionBTC) handleSubmitShare(e EventSubmitShareBTC) {
	if e.Message.IsFakeJob {
		up.sendSubmitResponse(e.Message.Base.SessionID, e.ID, STATUS_ACCEPT)
		return
	}
	glog.Info("handleSubmitShare workerFullName: ", up.downSessions[e.Message.Base.SessionID].fullName)
	glog.Info("handleSubmitShare. ID: ", e.ID, " message: ", *e.Message)

	// test request with writeJSONRequest (not writeExMessage)

	var request JSONRPCRequest
	request.ID = e.ID
	request.Method = "mining.submit"
	request.SetParams(
		up.downSessions[e.Message.Base.SessionID].fullName,
		e.Message.Base.JobID,
		e.Message.Base.ExtraNonce2,
		e.Message.Time,
		e.Message.Base.Nonce)

	glog.Info("handleSubmitShare request: ", request)
	_, err := up.writeJSONRequest(&request)

	if err != nil {
		glog.Error(up.id, "failed to submit share: ", err.Error())
		up.close()
		return
	}

	if up.config.SubmitResponseFromServer && up.serverCapSubmitResponse {
		up.submitIDs[up.submitIndex] = SubmitID{e.ID, e.Message.Base.SessionID}
		up.submitIndex++
	} else {
		up.sendSubmitResponse(e.Message.Base.SessionID, e.ID, STATUS_ACCEPT)
	}

}

func (up *UpSessionBTC) sendSubmitResponse(sessionID uint16, id interface{}, status StratumStatus) {
	down, ok := up.downSessions[sessionID]
	if !ok {
		// The client has been disconnected, ignored
		if glog.V(3) {
			glog.Info(up.id, "cannot find down session: ", sessionID)
		}
		return
	}
	go down.SendEvent(EventSubmitResponse{id, status})
}

func (up *UpSessionBTC) downSessionBroken(e EventDownSessionBroken) {
	delete(up.downSessions, e.SessionID)
	//up.unregisterWorker(e.SessionID)

	if up.disconnectedMinerCounter == 0 {
		go func() {
			time.Sleep(1 * time.Second)
			up.SendEvent(EventSendUpdateMinerNum{})
		}()
	}
	up.disconnectedMinerCounter++
}

func (up *UpSessionBTC) sendUpdateMinerNum() {
	go up.manager.SendEvent(EventUpdateMinerNum{up.slot, up.disconnectedMinerCounter})
	up.disconnectedMinerCounter = 0
}

func (up *UpSessionBTC) outdatedUpSessionConnection(e EventUpSessionConnection) {
	// up.Connect () method has its own event loop to receive connections.
	// So the connection to arrive here is extra, you can close directly.
	if e.Conn != nil {
		e.Conn.Close()
	}
}

func (up *UpSessionBTC) handleEvent() {
	up.eventLoopRunning = true
	for up.eventLoopRunning {
		event := <-up.eventChannel

		switch e := event.(type) {
		case EventAddDownSession:
			up.addDownSession(e)
		case EventSubmitShareBTC:
			up.handleSubmitShare(e)
		case EventDownSessionBroken:
			up.downSessionBroken(e)
		case EventSendUpdateMinerNum:
			up.sendUpdateMinerNum()
		case EventRecvJSONRPCBTC:
			up.recvJSONRPC(e)
		case EventConnBroken:
			up.close()
		case EventUpSessionConnection:
			up.outdatedUpSessionConnection(e)
		case EventExit:
			up.exit()
		default:
			glog.Error(up.id, "unknown event: ", e)
		}
	}
}
