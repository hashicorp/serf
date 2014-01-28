package command

import (
	"flag"
	"fmt"
	"github.com/mitchellh/cli"
	"net"
	"regexp"
	"strings"
)

// MembersCommand is a Command implementation that queries a running
// Serf agent what members are part of the cluster currently.
type MembersCommand struct {
	Ui cli.Ui
}

// A container of member details. Maintaining a command-specific struct here
// makes sense so that the agent.Member struct can evolve without changing the
// keys in the output interface.
type Member struct {
	detail   bool
	XMLName  string            `json:"-"        xml:"member"`
	Name     string            `json:"name"     xml:"name,attr"`
	Addr     string            `json:"addr"     xml:"addr"`
	Port     uint16            `json:"port"     xml:"port"`
	Tags     TagContainer      `json:"-"        xml:"tags"`
	StrTags  map[string]string `json:"tags"     xml:"-"`
	Status   string            `json:"status"   xml:"status"`
	Proto    ProtoDetail       `json:"protocol" xml:"protocol"`
}

type ProtoDetail struct {
	Min uint8 `json:"min"     xml:"min"`
	Max uint8 `json:"max"     xml:"max"`
	Ver uint8 `json:"version" xml:"version"`
}

type Tag struct {
	XMLName string `xml:"tag"`
	Name    string `xml:"name,attr"`
	Value   string `xml:"val,attr"`
}

type TagContainer struct {
	Tags []Tag `xml:"tag"`
}

type MemberContainer struct{
	XMLName string   `json:"-"       xml:"members"`
	Members []Member `json:"members" xml:"members"`
}

func (c MemberContainer) String() string {
	var result string
	for _, member := range c.Members {
		// Format the tags as tag1=v1,tag2=v2,...
		var tagPairs []string
		for name, value := range member.StrTags {
			tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", name, value))
		}
		tags := strings.Join(tagPairs, ",")

		result += fmt.Sprintf("%s     %s    %s    %s",
			member.Name, member.Addr, member.Status, tags)
		if member.detail {
			result += fmt.Sprintf(
				"    Protocol Version: %d    Available Protocol Range: [%d, %d]",
				member.Proto.Ver, member.Proto.Min, member.Proto.Max)
		}
		result += "\n"
	}
	return result
}

func (c *MembersCommand) Help() string {
	helpText := `
Usage: serf members [options]

  Outputs the members of a running Serf agent.

Options:

  -format                   If provided, output is returned in the specified
                            format. Valid formats are 'json', 'xml', and
                            'text' (default)

  -detailed                 Additional information such as protocol verions
                            will be shown (only affects text output format).

  -role=<regexp>            If provided, output is filtered to only nodes matching
                            the regular expression for role

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.

  -status=<regexp>			If provided, output is filtered to only nodes matching
                            the regular expression for status
`
	return strings.TrimSpace(helpText)
}

func (c *MembersCommand) Run(args []string) int {
	var detailed bool
	var roleFilter, statusFilter, format string
	cmdFlags := flag.NewFlagSet("members", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.BoolVar(&detailed, "detailed", false, "detailed output")
	cmdFlags.StringVar(&roleFilter, "role", ".*", "role filter")
	cmdFlags.StringVar(&statusFilter, "status", ".*", "status filter")
	cmdFlags.StringVar(&format, "format", "text", "output format")
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	// Compile the regexp
	roleRe, err := regexp.Compile(roleFilter)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to compile role regexp: %v", err))
		return 1
	}
	statusRe, err := regexp.Compile(statusFilter)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Failed to compile status regexp: %v", err))
		return 1
	}

	client, err := RPCClient(*rpcAddr)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error connecting to Serf agent: %s", err))
		return 1
	}
	defer client.Close()

	members, err := client.Members()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving members: %s", err))
		return 1
	}

	result := MemberContainer{}

	for _, member := range members {
		// Skip the non-matching members
		if !roleRe.MatchString(member.Tags["role"]) || !statusRe.MatchString(member.Status) {
			continue
		}

		addr := net.TCPAddr{IP: member.Addr, Port: int(member.Port)}

		tags := TagContainer{}
		for name, value := range member.Tags {
			tags.Tags = append(tags.Tags, Tag{
				Name: name,
				Value: value,
			})
		}

		result.Members = append(result.Members, Member{
			detail: detailed,
			Name: member.Name,
			Addr: addr.String(),
			Port: member.Port,
			Tags: tags,
			StrTags: member.Tags,
			Status: member.Status,
			Proto: ProtoDetail{
				Min: member.DelegateMin,
				Max: member.DelegateMax,
				Ver: member.DelegateCur,
			},
		})
	}

	output, err := formatOutput(result, format)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Encoding error: %s", err))
		return 1
	}

	c.Ui.Output(string(output))
	return 0
}

func (c *MembersCommand) Synopsis() string {
	return "Lists the members of a Serf cluster"
}
