package serf

import (
	"encoding/base64"
	"fmt"
)

// keyRequest is used to encapsulate a modification request to the keyring
type keyRequest struct {
	serf *Serf

	// query is the name of the internal query to perform
	query string

	// key is the base64-encoded value of the key being operated on.
	key string
}

// KeyResponse is used to deliver the results of a key query
type KeyResponse struct {
	// Messages is a mapping of node name to message. Messages can be any
	// message that the node needs to relay back to indicate its result.
	Messages map[string]string

	Keys []string

	// Err contains any error thrown while constructing or executing the query.
	// If this value is nil, and Errors is greater than zero at completion,
	// then this will be automatically set to an error summary message.
	Err error
}

// newKeyResponse creates a new key response and allocates substructures
func newKeyResponse() *KeyResponse {
	return &KeyResponse{
		Messages: make(map[string]string),
		Err:      nil,
	}
}

// condAppend will append a new string to a list of strings only if it does
// not already exist in the list.
func condAppend(strList []string, newStr string) []string {
	for _, str := range strList {
		if str == newStr {
			return strList
		}
	}
	return append(strList, newStr)
}

// Process will examine the query type and invoke the proper method to handle
// running it and gathering responses.
func (kr *keyRequest) Process() *KeyResponse {
	switch kr.query {
	case listKeysQuery:
		return kr.processList()
	case installKeyQuery, useKeyQuery, removeKeyQuery:
		return kr.processModification()
	}
	return nil
}

// processModification is used to perform an internal query accross all members
// in a cluster and modify keyring data. This function manages distributing key
// commands to our members and aggregating their responses into a KeyResponse.
func (kr *keyRequest) processModification() *KeyResponse {
	resp := newKeyResponse()
	qName := internalQueryName(kr.query)

	// Decode the new key into raw bytes before storing it away. This ensures
	// that later on when we go to copy the new key into the memberlist config
	// there will be no decoding errors, which would cause cluster segmentation.
	rawKey, err := base64.StdEncoding.DecodeString(kr.key)
	if err != nil {
		resp.Err = err
		return resp
	}

	qParam := &QueryParam{}
	queryResp, err := kr.serf.Query(qName, rawKey, qParam)
	if err != nil {
		resp.Err = err
		return resp
	}

	totalReplies := 0
	totalErrors := 0
	for r := range queryResp.respCh {
		var nodeResponse nodeKeyResponse

		totalReplies++

		// Decode the response
		if len(r.Payload) < 1 || messageType(r.Payload[0]) != messageKeyResponseType {
			resp.Messages[r.From] = fmt.Sprintf(
				"Invalid response type: %v", r.Payload)
			totalErrors++
			continue
		}
		if err := decodeMessage(r.Payload[1:], &nodeResponse); err != nil {
			resp.Messages[r.From] = fmt.Sprintf(
				"Failed to decode response: %v", r.Payload)
			totalErrors++
			continue
		}

		if !nodeResponse.Result {
			resp.Messages[r.From] = nodeResponse.Message
			totalErrors++
		}
	}

	totalMembers := kr.serf.memberlist.NumMembers()

	if totalErrors != 0 {
		resp.Err = fmt.Errorf("%d/%d nodes reported failure", totalErrors, totalMembers)
		goto END
	}
	if totalReplies != totalMembers {
		resp.Err = fmt.Errorf("%d/%d nodes reported success", totalReplies, totalMembers)
		goto END
	}

END:
	return resp
}

// processList is used to collect installed keys from members in a Serf cluster
// and return an aggregated list of all installed keys. This is useful to
// operators to ensure that there are no lingering keys installed on any agents.
// Since having multiple keys installed can cause performance penalties in some
// cases, it's important to verify this information and remove unneeded keys.
func (kr *keyRequest) processList() *KeyResponse {
	resp := newKeyResponse()
	qName := internalQueryName(kr.query)

	qParam := &QueryParam{}
	queryResp, err := kr.serf.Query(qName, nil, qParam)
	if err != nil {
		resp.Err = err
		return resp
	}

	totalReplies := 0
	totalErrors := 0
	for r := range queryResp.respCh {
		var nodeResponse nodeKeyResponse

		totalReplies++

		// Decode the response
		if len(r.Payload) < 1 || messageType(r.Payload[0]) != messageKeyResponseType {
			resp.Messages[r.From] = fmt.Sprintf(
				"Invalid response type: %v", r.Payload)
			totalErrors++
			continue
		}
		if err := decodeMessage(r.Payload[1:], &nodeResponse); err != nil {
			resp.Messages[r.From] = fmt.Sprintf(
				"Failed to decode response: %v", r.Payload)
			totalErrors++
			continue
		}

		if !nodeResponse.Result {
			resp.Messages[r.From] = nodeResponse.Message
			totalErrors++
			continue
		}

		for _, key := range nodeResponse.Keys {
			resp.Keys = condAppend(resp.Keys, key)
		}
	}

	totalMembers := kr.serf.memberlist.NumMembers()

	if totalErrors != 0 {
		resp.Err = fmt.Errorf("%d/%d nodes reported failure", totalErrors, totalMembers)
		goto END
	}
	if totalReplies != totalMembers {
		resp.Err = fmt.Errorf("%d/%d nodes reported success", totalReplies, totalMembers)
		goto END
	}

END:
	return resp
}
