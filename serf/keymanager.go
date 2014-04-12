package serf

import (
	"encoding/base64"
	"fmt"
)

// keyManager encapsulates all functionality within Serf for handling
// encryption keyring changes across a cluster.
type keyManager struct {
	*Serf
}

// ModifyKeyResponse is used to relay results of keyring modifications.
type ModifyKeyResponse struct {
	Messages   map[string]string // Map of node name to response message
	TotalNodes int               // Total nodes in the cluster
}

// ListKeysResponse is used to relay a query for a list of all keys in use
// on a Serf cluster.
type ListKeysResponse struct {
	Messages   map[string]string // Map of node name to response message
	TotalNodes int               // Total nodes in the cluster

	// Keys is a mapping of the base64-encoded value of the key bytes to the
	// number of nodes that have the key installed.
	Keys map[string]int
}

// KeyManager returns a keyManager for the current Serf instance
func (s *Serf) KeyManager() *keyManager {
	return &keyManager{s}
}

// InstallKey handles broadcasting a query to all members and gathering
// responses from each of them, returning a list of messages from each node
// and any applicable error conditions.
func (k *keyManager) InstallKey(key string) (*ModifyKeyResponse, error) {
	resp := &ModifyKeyResponse{Messages: make(map[string]string)}
	qName := internalQueryName(installKeyQuery)

	// Decode the new key into raw bytes
	rawKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}

	qParam := &QueryParam{}
	queryResp, err := k.Query(qName, rawKey, qParam)
	if err != nil {
		return nil, err
	}

	totalErrors := 0
	for r := range queryResp.respCh {
		var nodeResponse nodeKeyResponse

		resp.TotalNodes++

		// Decode the response
		if len(r.Payload) < 1 || messageType(r.Payload[0]) != messageKeyResponseType {
			resp.Messages[r.From] = fmt.Sprintf(
				"Invalid install-key response type: %v", r.Payload)
			totalErrors++
			continue
		}
		if err := decodeMessage(r.Payload[1:], &nodeResponse); err != nil {
			resp.Messages[r.From] = fmt.Sprintf(
				"Failed to decode install-key response: %v", r.Payload)
			totalErrors++
			continue
		}

		if !nodeResponse.Result {
			resp.Messages[r.From] = nodeResponse.Message
			totalErrors++
		}
	}

	totalMembers := k.memberlist.NumMembers()

	if totalErrors != 0 {
		return resp, fmt.Errorf("%d/%d nodes reported failure", totalErrors, totalMembers)
	}
	if resp.TotalNodes != totalMembers {
		return resp, fmt.Errorf("%d/%d nodes reported success", resp.TotalNodes, totalMembers)
	}

	return resp, nil
}

// UseKey handles broadcasting a primary key change to all members in the
// cluster, and gathering any response messages. If successful, there should
// be an empty ModifyKeyResponse returned.
func (k *keyManager) UseKey(key string) (*ModifyKeyResponse, error) {
	resp := &ModifyKeyResponse{Messages: make(map[string]string)}
	qName := internalQueryName(useKeyQuery)

	// Decode the new key into raw bytes
	rawKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}

	qParam := &QueryParam{}
	queryResp, err := k.Query(qName, rawKey, qParam)
	if err != nil {
		return nil, err
	}

	totalErrors := 0
	for r := range queryResp.respCh {
		var nodeResponse nodeKeyResponse

		resp.TotalNodes++

		// Decode the response
		if len(r.Payload) < 1 || messageType(r.Payload[0]) != messageKeyResponseType {
			resp.Messages[r.From] = fmt.Sprintf(
				"Invalid use-key response type: %v", r.Payload)
			totalErrors++
			continue
		}
		if err := decodeMessage(r.Payload[1:], &nodeResponse); err != nil {
			resp.Messages[r.From] = fmt.Sprintf(
				"Failed to decode use-key response: %v", r.Payload)
			totalErrors++
			continue
		}

		if !nodeResponse.Result {
			resp.Messages[r.From] = nodeResponse.Message
			totalErrors++
		}
	}

	totalMembers := k.memberlist.NumMembers()

	if totalErrors != 0 {
		return resp, fmt.Errorf("%d/%d nodes reported failure", totalErrors, totalMembers)
	}
	if resp.TotalNodes != totalMembers {
		return resp, fmt.Errorf("%d/%d nodes reported success", resp.TotalNodes, totalMembers)
	}

	return resp, nil
}

// RemoveKey handles broadcasting a key to the cluster for removal. Each member
// will receive this event, and if they have the key in their keyring, remove
// it. If any errors are encountered, RemoveKey will collect and relay them.
func (k *keyManager) RemoveKey(key string) (*ModifyKeyResponse, error) {
	resp := &ModifyKeyResponse{Messages: make(map[string]string)}
	qName := internalQueryName(removeKeyQuery)

	// Decode the new key into raw bytes
	rawKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}

	qParam := &QueryParam{}
	queryResp, err := k.Query(qName, rawKey, qParam)
	if err != nil {
		return nil, err
	}

	totalErrors := 0
	for r := range queryResp.respCh {
		var nodeResponse nodeKeyResponse

		resp.TotalNodes++

		// Decode the response
		if len(r.Payload) < 1 || messageType(r.Payload[0]) != messageKeyResponseType {
			resp.Messages[r.From] = fmt.Sprintf(
				"Invalid remove-key response type: %v", r.Payload)
			totalErrors++
			continue
		}
		if err := decodeMessage(r.Payload[1:], &nodeResponse); err != nil {
			resp.Messages[r.From] = fmt.Sprintf(
				"Failed to decode remove-key response: %v", r.Payload)
			totalErrors++
			continue
		}

		if !nodeResponse.Result {
			resp.Messages[r.From] = nodeResponse.Message
			totalErrors++
		}
	}

	totalMembers := k.memberlist.NumMembers()

	if totalErrors != 0 {
		return resp, fmt.Errorf("%d/%d nodes reported failure", totalErrors, totalMembers)
	}
	if resp.TotalNodes != totalMembers {
		return resp, fmt.Errorf("%d/%d nodes reported success", resp.TotalNodes, totalMembers)
	}

	return resp, nil
}

// ListKeys is used to collect installed keys from members in a Serf cluster
// and return an aggregated list of all installed keys. This is useful to
// operators to ensure that there are no lingering keys installed on any agents.
// Since having multiple keys installed can cause performance penalties in some
// cases, it's important to verify this information and remove unneeded keys.
func (k *keyManager) ListKeys() (*ListKeysResponse, error) {
	resp := &ListKeysResponse{
		Messages: make(map[string]string),
		Keys:     make(map[string]int),
	}
	qName := internalQueryName(listKeysQuery)

	qParam := &QueryParam{}
	queryResp, err := k.Query(qName, nil, qParam)
	if err != nil {
		return nil, err
	}

	totalErrors := 0
	for r := range queryResp.respCh {
		var nodeResponse nodeKeyResponse

		resp.TotalNodes++

		// Decode the response
		if len(r.Payload) < 1 || messageType(r.Payload[0]) != messageKeyResponseType {
			resp.Messages[r.From] = fmt.Sprintf(
				"Invalid list-keys response type: %v", r.Payload)
			totalErrors++
			continue
		}
		if err := decodeMessage(r.Payload[1:], &nodeResponse); err != nil {
			resp.Messages[r.From] = fmt.Sprintf(
				"Failed to decode list-keys response: %v", r.Payload)
			totalErrors++
			continue
		}

		if !nodeResponse.Result {
			resp.Messages[r.From] = nodeResponse.Message
			totalErrors++
			continue
		}

		for _, key := range nodeResponse.Keys {
			if _, ok := resp.Keys[key]; !ok {
				resp.Keys[key] = 1
			} else {
				resp.Keys[key]++
			}
		}
	}

	totalMembers := k.memberlist.NumMembers()

	if totalErrors != 0 {
		return resp, fmt.Errorf("%d/%d nodes reported failure", totalErrors, totalMembers)
	}
	if resp.TotalNodes != totalMembers {
		return resp, fmt.Errorf("%d/%d nodes reported success", resp.TotalNodes, totalMembers)
	}

	return resp, nil
}
