package serf

import (
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/memberlist"
)

type MergeDelegate interface {
	NotifyMerge([]*Member) error
}

type mergeDelegate struct {
	serf *Serf
}

func (m *mergeDelegate) NotifyMerge(nodes []*memberlist.Node) error {
	members := make([]*Member, len(nodes))
	for idx, n := range nodes {
		var err error
		members[idx], err = m.nodeToMember(n)
		if err != nil {
			return err
		}
	}
	return m.serf.config.Merge.NotifyMerge(members)
}

func (m *mergeDelegate) NotifyAlive(peer *memberlist.Node) error {
	member, err := m.nodeToMember(peer)
	if err != nil {
		return err
	}
	return m.serf.config.Merge.NotifyMerge([]*Member{member})
}

func (m *mergeDelegate) nodeToMember(n *memberlist.Node) (*Member, error) {
	status := StatusNone
	if n.State == memberlist.StateLeft {
		status = StatusLeft
	}
	if err := m.validiateMemberInfo(n); err != nil {
		return nil, err
	}
	return &Member{
		Name:        n.Name,
		Addr:        net.IP(n.Addr),
		Port:        n.Port,
		Tags:        m.serf.decodeTags(n.Meta),
		Status:      status,
		ProtocolMin: n.PMin,
		ProtocolMax: n.PMax,
		ProtocolCur: n.PCur,
		DelegateMin: n.DMin,
		DelegateMax: n.DMax,
		DelegateCur: n.DCur,
	}, nil
}

// validateMemberInfo checks that the data we are sending is valid
func (m *mergeDelegate) validiateMemberInfo(n *memberlist.Node) error {
	if err := m.serf.ValidateNodeNames(); err != nil {
		return err
	}

	host, port, err := net.SplitHostPort(string(n.Addr))
	if err != nil {
		return err
	}

	ip := net.ParseIP(host)
	if ip == nil || (ip.To4() == nil && ip.To16() == nil) {
		return fmt.Errorf("%v is not a valid IPv4 or IPv6 address\n", ip)
	}

	p, err := strconv.Atoi(port)
	if err != nil {
		return err
	}
	if p < 0 || p > 65535 {
		return fmt.Errorf("invalid port %v , port must be a valid number from 0-65535", p)
	}

	if len(n.Meta) > memberlist.MetaMaxSize {
		return fmt.Errorf("Encoded length of tags exceeds limit of %d bytes",
			memberlist.MetaMaxSize)
	}
	return nil
}
