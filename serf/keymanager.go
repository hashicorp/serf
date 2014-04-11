package serf

import (
	"encoding/base64"
	"fmt"
)

// keyManager encapsulates all functionality within Serf for handling
// encryption keyring changes across a cluster.
type keyManager struct {
	serf *Serf
}

type InstallKeyRequest struct {
	Key string
}

type UseKeyRequest struct {
	Key string
}

type RemoveKeyRequest struct {
	Key string
}

type InstallKeyResponse struct {
	Messages map[string]string
}

type UseKeyResponse struct {
	Messages map[string]string
}

type RemoveKeyResponse struct {
	Messages map[string]string
}

type ListKeysResponse struct {
	Keys     []string
	Messages map[string]string
}

// KeyManager returns a keyManager for the current Serf instance
func (s *Serf) KeyManager() *keyManager {
	return &keyManager{serf: s}
}

// InstallKey handles broadcasting a query to all members and gathering
// responses from each of them, returning a list of messages from each node
// and any applicable error conditions.
func (k *keyManager) InstallKey(key string) (*InstallKeyResponse, error) {
	resp := &InstallKeyResponse{Messages: make(map[string]string)}
	qName := internalQueryName(installKeyQuery)

	// Decode the new key into raw bytes
	rawKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}

	qParam := &QueryParam{}
	queryResp, err := k.serf.Query(qName, rawKey, qParam)
	if err != nil {
		return nil, err
	}

	totalReplies := 0
	totalErrors := 0
	for r := range queryResp.respCh {
		var nodeResponse nodeKeyResponse

		totalReplies++

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

	totalMembers := k.serf.memberlist.NumMembers()

	if totalErrors != 0 {
		return resp, fmt.Errorf("%d/%d nodes reported failure", totalErrors, totalMembers)
	}
	if totalReplies != totalMembers {
		return resp, fmt.Errorf("%d/%d nodes reported success", totalReplies, totalMembers)
	}

	return resp, nil
}

// UseKey handles broadcasting a primary key change to all members in the
// cluster, and gathering any response messages. If successful, there should
// be an empty UseKeyResponse returned.
func (k *keyManager) UseKey(key string) (*UseKeyResponse, error) {
	resp := &UseKeyResponse{Messages: make(map[string]string)}
	qName := internalQueryName(useKeyQuery)

	// Decode the new key into raw bytes
	rawKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}

	qParam := &QueryParam{}
	queryResp, err := k.serf.Query(qName, rawKey, qParam)
	if err != nil {
		return nil, err
	}

	totalReplies := 0
	totalErrors := 0
	for r := range queryResp.respCh {
		var nodeResponse nodeKeyResponse

		totalReplies++

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

	totalMembers := k.serf.memberlist.NumMembers()

	if totalErrors != 0 {
		return resp, fmt.Errorf("%d/%d nodes reported failure", totalErrors, totalMembers)
	}
	if totalReplies != totalMembers {
		return resp, fmt.Errorf("%d/%d nodes reported success", totalReplies, totalMembers)
	}

	return resp, nil
}

// RemoveKey handles broadcasting a key to the cluster for removal. Each member
// will receive this event, and if they have the key in their keyring, remove
// it. If any errors are encountered, RemoveKey will collect and relay them.
func (k *keyManager) RemoveKey(key string) (*RemoveKeyResponse, error) {
	resp := &RemoveKeyResponse{Messages: make(map[string]string)}
	qName := internalQueryName(removeKeyQuery)

	// Decode the new key into raw bytes
	rawKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}

	qParam := &QueryParam{}
	queryResp, err := k.serf.Query(qName, rawKey, qParam)
	if err != nil {
		return nil, err
	}

	totalReplies := 0
	totalErrors := 0
	for r := range queryResp.respCh {
		var nodeResponse nodeKeyResponse

		totalReplies++

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

	totalMembers := k.serf.memberlist.NumMembers()

	if totalErrors != 0 {
		return resp, fmt.Errorf("%d/%d nodes reported failure", totalErrors, totalMembers)
	}
	if totalReplies != totalMembers {
		return resp, fmt.Errorf("%d/%d nodes reported success", totalReplies, totalMembers)
	}

	return resp, nil
}

// ListKeys is used to collect installed keys from members in a Serf cluster
// and return an aggregated list of all installed keys. This is useful to
// operators to ensure that there are no lingering keys installed on any agents.
// Since having multiple keys installed can cause performance penalties in some
// cases, it's important to verify this information and remove unneeded keys.
func (k *keyManager) ListKeys() (*ListKeysResponse, error) {
	resp := &ListKeysResponse{Messages: make(map[string]string)}
	qName := internalQueryName(listKeysQuery)

	qParam := &QueryParam{}
	queryResp, err := k.serf.Query(qName, nil, qParam)
	if err != nil {
		return nil, err
	}

	totalReplies := 0
	totalErrors := 0
	for r := range queryResp.respCh {
		var nodeResponse nodeKeyResponse

		totalReplies++

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
			resp.Keys = condAppend(resp.Keys, key)
		}
	}

	totalMembers := k.serf.memberlist.NumMembers()

	if totalErrors != 0 {
		return resp, fmt.Errorf("%d/%d nodes reported failure", totalErrors, totalMembers)
	}
	if totalReplies != totalMembers {
		return resp, fmt.Errorf("%d/%d nodes reported success", totalReplies, totalMembers)
	}

	return resp, nil
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
