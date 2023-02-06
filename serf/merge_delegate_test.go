// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package serf

import (
	"net"
	"strings"
	"testing"

	"github.com/hashicorp/memberlist"
)

func TestValidateMemberInfo(t *testing.T) {
	type testCase struct {
		name              string
		addr              net.IP
		meta              []byte
		validateNodeNames bool
		err               string
	}

	cases := map[string]testCase{
		"invalid-name-chars": {
			name:              "space not allowed",
			addr:              []byte{1, 2, 3, 4},
			validateNodeNames: true,
			err:               "Node name contains invalid characters",
		},
		"invalid-name-chars-not-validated": {
			name:              "space not allowed",
			addr:              []byte{1, 2, 3, 4},
			validateNodeNames: false,
		},
		"invalid-name-len": {
			name:              strings.Repeat("abcd", 33),
			addr:              []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			validateNodeNames: true,
			err:               "Node name is 132 characters.", // 33 * 4
		},
		"invalid-name-len-not-validated": {
			name:              strings.Repeat("abcd", 33),
			addr:              []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			validateNodeNames: false,
		},
		"invalid-ip": {
			name: "test",
			addr: []byte{1, 2}, // length has to be 4 or 16
			err:  "IP byte length is invalid",
		},
		"invalid-ip-2": {
			name: "test",
			addr: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17}, // length has to be 4 or 16
			err:  "IP byte length is invalid",
		},
		"meta-too-long": {
			name: "test",
			addr: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			meta: []byte(strings.Repeat("a", 513)),
			err:  "Encoded length of tags exceeds limit",
		},
		"ipv4-okay": {
			name: "test",
			addr: []byte{1, 1, 1, 1},
		},
		"ipv6-okay": {
			name: "test",
			addr: []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {

			delegate := mergeDelegate{
				serf: &Serf{
					config: &Config{
						ValidateNodeNames: tcase.validateNodeNames,
					},
				},
			}

			node := &memberlist.Node{
				Name: tcase.name,
				Addr: tcase.addr,
				Meta: tcase.meta,
			}

			err := delegate.validateMemberInfo(node)

			if tcase.err == "" {
				if err != nil {
					t.Fatalf("Encountered an unexpected error when validating member info: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("Did not encounter the expected error of %q", tcase.err)
				}
				if !strings.Contains(err.Error(), tcase.err) {
					t.Fatalf("Member info validation failed with a different error than expected. Expected: %q, Actual: %q", tcase.err, err.Error())
				}
			}
		})
	}
}
