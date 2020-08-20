package serf

import (
	"fmt"
	"net"
	"regexp"

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
	var InvalidNameRe = regexp.MustCompile(`[^A-Za-z0-9\\-]+`)

	if len(n.Name) > 128 {
		return fmt.Errorf("NodeName length is %v characters. Valid length is between "+
			"1 and 128 characters.", len(n.Name))
	}
	if InvalidNameRe.MatchString(n.Name) {
		return fmt.Errorf("Nodename contains invalid characters %v , Valid characters include "+
			"all alpha-numerics and dashes", n.Name)
	}

	if net.ParseIP(string(n.Addr)) == nil {
		return fmt.Errorf("Address is %v . Must be a valid representation of an IP address. ", n.Addr)
	}
	if len(n.Meta) > memberlist.MetaMaxSize {
		return fmt.Errorf("Encoded length of tags exceeds limit of %d bytes",
			memberlist.MetaMaxSize)
	}
	return nil
}
