package main

import "errors"

// StratumError Stratum error
type StratumError struct {
	// 错误号
	ErrNo int
	// 错误信息
	ErrMsg string
}

// NewStratumError Create a new Stratum Error
func NewStratumError(errNo int, errMsg string) *StratumError {
	err := new(StratumError)
	err.ErrNo = errNo
	err.ErrMsg = errMsg

	return err
}

// Error Implements the Error() interface of Stratum Error so that it can be used as an error type
func (err *StratumError) Error() string {
	return err.ErrMsg
}

// ToJSONRPCArray Convert to JSONRPC Array
func (err *StratumError) ToJSONRPCArray(extData interface{}) interface{} {
	if err == nil {
		return nil
	}

	return JSONRPCArray{err.ErrNo, err.ErrMsg, extData}
}

var (
	// ErrBufIOReadTimeout Timeout while reading data from bufio.Reader
	ErrBufIOReadTimeout = errors.New("bufIO read timeout")
	// ErrSessionIDFull Session ID is full (all available values are assigned)
	ErrSessionIDFull = errors.New("session id is full")
	// ErrSessionIDOccupied The Session ID has been occupied (when restoring the Session ID)
	ErrSessionIDOccupied = errors.New("session id has been occupied")
	// ErrParseSubscribeResponseFailed Failed to parse subscription response
	ErrParseSubscribeResponseFailed = errors.New("parse subscribe response failed")
	// ErrSessionIDInconformity The returned session ID does not match the currently saved one
	ErrSessionIDInconformity = errors.New("session id inconformity")
	// ErrAuthorizeFailed Authentication failed
	ErrAuthorizeFailed = errors.New("authorize failed")
	// ErrTooMuchPendingAutoRegReq Too many pending auto-registration requests
	ErrTooMuchPendingAutoRegReq = errors.New("too much pending auto reg request")
)

var (
	// StratumErrJobNotFound task does not exist
	StratumErrJobNotFound = NewStratumError(21, "Job not found (=stale)")
	// StratumErrNeedAuthorized Authentication required
	StratumErrNeedAuthorized = NewStratumError(24, "Unauthorized worker")
	// StratumErrNeedSubscribed Subscription required
	StratumErrNeedSubscribed = NewStratumError(25, "Not subscribed")
	// StratumErrIllegalParams Illegal parameter
	StratumErrIllegalParams = NewStratumError(27, "Illegal params")
	// StratumErrTooFewParams too few parameters
	StratumErrTooFewParams = NewStratumError(27, "Too few params")
	// StratumErrDuplicateSubscribed Repeat subscription
	StratumErrDuplicateSubscribed = NewStratumError(102, "Duplicate Subscribed")
	// StratumErrWorkerNameMustBeString Miner name must be a string
	StratumErrWorkerNameMustBeString = NewStratumError(104, "Worker Name Must be a String")
	// StratumErrSubAccountNameEmpty Sub account name is empty
	StratumErrSubAccountNameEmpty = NewStratumError(105, "Sub-account Name Cannot be Empty")

	// StratumErrStratumServerNotFound The Stratum Server for the corresponding currency could not be found
	StratumErrStratumServerNotFound = NewStratumError(301, "Stratum Server Not Found")
	// StratumErrConnectStratumServerFailed The Stratum Server connection of the corresponding currency fails
	StratumErrConnectStratumServerFailed = NewStratumError(302, "Connect Stratum Server Failed")

	// StratumErrUnknownChainType Unknown blockchain type
	StratumErrUnknownChainType = NewStratumError(500, "Unknown Chain Type")
)

var (
	// ErrReadFailed IO read error
	ErrReadFailed = errors.New("read failed")
	// ErrWriteFailed IO write error
	ErrWriteFailed = errors.New("write failed")
	// ErrInvalidReader illegal reader
	ErrInvalidReader = errors.New("invalid reader")
	// ErrInvalidWritter Illegal Writer
	ErrInvalidWritter = errors.New("invalid writter")
	// ErrInvalidBuffer Illegal Buffer
	ErrInvalidBuffer = errors.New("invalid buffer")
	// ErrConnectionClosed connection closed
	ErrConnectionClosed = errors.New("connection closed")
)
