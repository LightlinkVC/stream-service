package dto

import "encoding/json"

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint            `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type OnCreateMediaElementResult struct {
	ElementID string `json:"value"`
	SessionID string `json:"sessionId"`
}

type OnSDPOfferProcessResult struct {
	RemoteSDPOffer string `json:"value"`
}

type OnSubscribeEventResult struct {
	Value     string `json:"value"`
	SessionID string `json:"sessionId"`
}

type OnGenerateSDPOfferResult struct {
	SDPOffer  string `json:"value"`
	SessionID string `json:"sessionId"`
}

type IceCandidate struct {
	Candidate     string `json:"candidate"`
	SdpMid        string `json:"sdpMid"`
	SdpMLineIndex uint   `json:"sdpMLineIndex"`
}
