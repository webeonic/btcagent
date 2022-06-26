package main

import (
	"encoding/json"
)

// JSONRPCRequest JSON RPC Request data structure
type JSONRPCRequest struct {
	ID     interface{}   `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

// JSONRPCResponse JSON RPC Response data structure
type JSONRPCResponse struct {
	ID     interface{} `json:"id"`
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"`
}

type JSONRPCLineBTC struct {
	ID     interface{}   `json:"id,omitempty"`
	Method string        `json:"method,omitempty"`
	Params []interface{} `json:"params,omitempty"`
	Result interface{}   `json:"result,omitempty"`
	Error  interface{}   `json:"error,omitempty"`
}

// JSONRPC2Error error object of json-rpc 2.0
type JSONRPC2Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSONRPC2Request request message of json-rpc 2.0
type JSONRPC2Request struct {
	ID      interface{}   `json:"id"`
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// JSONRPC2Response response message of json-rpc 2.0
type JSONRPC2Response struct {
	ID      interface{}    `json:"id"`
	JSONRPC string         `json:"jsonrpc"`
	Result  interface{}    `json:"result,omitempty"`
	Error   *JSONRPC2Error `json:"error,omitempty"`
}

// JSONRPCArray JSON RPC Array
type JSONRPCArray []interface{}

// JSONRPCObj JSON RPC Object
type JSONRPCObj map[string]interface{}

// NewJSONRPC2Error create json-rpc 2.0 error object from json-1.0 error object
func NewJSONRPC2Error(v1Err interface{}) (err *JSONRPC2Error) {
	if v1Err == nil {
		return nil
	}

	errArr, ok := v1Err.(JSONRPCArray)
	if !ok {
		return nil
	}

	err = new(JSONRPC2Error)
	if len(errArr) >= 1 {
		code, ok := errArr[0].(int)
		if ok {
			err.Code = code
		}
	}
	if len(errArr) >= 2 {
		message, ok := errArr[1].(string)
		if ok {
			err.Message = message
		}
	}
	if len(errArr) >= 3 {
		err.Data = errArr[2]
	}
	return
}

func NewJSONRPCLineBTC(rpcJSON []byte) (rpcData *JSONRPCLineBTC, err error) {
	rpcData = new(JSONRPCLineBTC)
	err = json.Unmarshal(rpcJSON, &rpcData)
	return
}

// NewJSONRPCRequest Parse the JSON RPC request string and create a JSONRPCRequest object
func NewJSONRPCRequest(rpcJSON []byte) (rpcData *JSONRPCRequest, err error) {
	rpcData = new(JSONRPCRequest)
	err = json.Unmarshal(rpcJSON, &rpcData)
	return
}

// AddParams 向 JSONRPCRequest 对象添加一个或多个参数
func (rpcData *JSONRPCRequest) AddParams(param ...interface{}) {
	rpcData.Params = append(rpcData.Params, param...)
}

//SetParams Set the parameters of the JSONRPCREQUEST object
// Parameter list passed to setParams will be copied in the order of jsonrpcRequest.Params
func (rpcData *JSONRPCRequest) SetParams(param ...interface{}) {
	rpcData.Params = param
}

// TojsonBytes converts the JSONRPCREQUEST object to a JSON byte sequence
func (rpcData *JSONRPCRequest) ToJSONBytes() ([]byte, error) {
	return json.Marshal(rpcData)
}

func (rpcData *JSONRPCRequest) ToJSONBytesLine() (bytes []byte, err error) {
	bytes, err = rpcData.ToJSONBytes()
	if err == nil {
		bytes = append(bytes, '\n')
	}
	return
}

func (rpcData *JSONRPCRequest) ToRPC2JSONBytes() ([]byte, error) {
	id := rpcData.ID
	if id == nil {
		id = 0
	}
	rpc2Data := JSONRPC2Request{id, "2.0", rpcData.Method, rpcData.Params}
	return json.Marshal(rpc2Data)
}

func (rpcData *JSONRPCRequest) ToRPC2JSONBytesLine() (bytes []byte, err error) {
	bytes, err = rpcData.ToRPC2JSONBytes()
	if err == nil {
		bytes = append(bytes, '\n')
	}
	return
}

func (rpcData *JSONRPCRequest) ToJSONBytesLineWithVersion(version int) (bytes []byte, err error) {
	if version == 2 {
		return rpcData.ToRPC2JSONBytesLine()
	}
	return rpcData.ToJSONBytesLine()
}

// NewJSONRPCResponse 解析 JSON RPC 响应字符串并创建 JSONRPCResponse 对象
func NewJSONRPCResponse(rpcJSON []byte) (rpcData *JSONRPCResponse, err error) {
	rpcData = new(JSONRPCResponse)
	err = json.Unmarshal(rpcJSON, &rpcData)
	return
}

// SetResult 设置 JSONRPCResponse 对象的返回结果
func (rpcData *JSONRPCResponse) SetResult(result interface{}) {
	rpcData.Result = result
}

// ToJSONBytes 将 JSONRPCResponse 对象转换为 JSON 字节序列
func (rpcData *JSONRPCResponse) ToJSONBytes() ([]byte, error) {
	return json.Marshal(rpcData)
}

func (rpcData *JSONRPCResponse) ToJSONBytesLine() (bytes []byte, err error) {
	bytes, err = rpcData.ToJSONBytes()
	if err == nil {
		bytes = append(bytes, '\n')
	}
	return
}

func (rpcData *JSONRPCResponse) ToRPC2JSONBytes() ([]byte, error) {
	id := rpcData.ID
	if id == nil {
		id = 0
	}
	rpc2Data := JSONRPC2Response{id, "2.0", rpcData.Result, NewJSONRPC2Error(rpcData.Error)}
	return json.Marshal(rpc2Data)
}

func (rpcData *JSONRPCResponse) ToRPC2JSONBytesLine() (bytes []byte, err error) {
	bytes, err = rpcData.ToRPC2JSONBytes()
	if err == nil {
		bytes = append(bytes, '\n')
	}
	return
}

func (rpcData *JSONRPCResponse) ToJSONBytesLineWithVersion(version int) (bytes []byte, err error) {
	if version == 2 {
		return rpcData.ToRPC2JSONBytesLine()
	}
	return rpcData.ToJSONBytesLine()
}
