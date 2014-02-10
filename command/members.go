package command

import (
	"flag"
	"fmt"
	"github.com/hashicorp/serf/command/agent"
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
	detail bool
	Name   string            `json:"name"`
	Addr   string            `json:"addr"`
	Port   uint16            `json:"port"`
	Tags   map[string]string `json:"tags"`
	Status string            `json:"status"`
	Proto  map[string]uint8  `json:"protocol"`
}

type MemberContainer struct {
	Members []Member `json:"members"`
}

func (c MemberContainer) String() string {
	var result string
	for _, member := range c.Members {
		// Format the tags as tag1=v1,tag2=v2,...
		var tagPairs []string
		for name, value := range member.Tags {
			tagPairs = append(tagPairs, fmt.Sprintf("%s=%s", name, value))
		}
		tags := strings.Join(tagPairs, ",")

		result += fmt.Sprintf("%s     %s    %s    %s",
			member.Name, member.Addr, member.Status, tags)
		if member.detail {
			result += fmt.Sprintf(
				"    Protocol Version: %d    Available Protocol Range: [%d, %d]",
				member.Proto["version"], member.Proto["min"], member.Proto["max"])
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

  -detailed                 Additional information such as protocol verions
                            will be shown (only affects text output format).

  -format                   If provided, output is returned in the specified
                            format. Valid formats are 'json', and 'text' (default)

  -role=<regexp>            If provided, output is filtered to only nodes matching
                            the regular expression for role
                            '-role' is deprecated in favor of '-tag role=foo'.

  -rpc-addr=127.0.0.1:7373  RPC address of the Serf agent.

  -status=<regexp>			If provided, output is filtered to only nodes matching
                            the regular expression for status

  -tag <key>=<regexp>       If provided, output is filtered to only nodes with the
                            tag <key> with value matching the regular expression.
                            tag can be specified multiple times to filter on
                            multiple keys.
`
	return strings.TrimSpace(helpText)
}

func (c *MembersCommand) Run(args []string) int {
	var detailed bool
	var roleFilter, statusFilter, format string
	var tags []string
	var tagRes map[string](*regexp.Regexp)
	cmdFlags := flag.NewFlagSet("members", flag.ContinueOnError)
	cmdFlags.Usage = func() { c.Ui.Output(c.Help()) }
	cmdFlags.BoolVar(&detailed, "detailed", false, "detailed output")
	cmdFlags.StringVar(&roleFilter, "role", ".*", "role filter")
	cmdFlags.StringVar(&statusFilter, "status", ".*", "status filter")
	cmdFlags.StringVar(&format, "format", "text", "output format")
	cmdFlags.Var((*agent.AppendSliceValue)(&tags), "tag", "tag filter")
	rpcAddr := RPCAddrFlag(cmdFlags)
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	// Deprecation warning for role
	if roleFilter != ".*" {
		c.Ui.Output("Deprecation warning: 'Role' has been replaced with 'Tags'")
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
	tagRes = make(map[string](*regexp.Regexp))
	for _, tag := range tags {
		parts := strings.SplitN(tag, "=", 2)
		if len(parts) != 2 {
			c.Ui.Error(fmt.Sprintf("Invalid tag '%s' provided", tag))
			return 1
		}
		tagRe, err := regexp.Compile(parts[1])
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Failed to compile status regexp: %v", err))
			return 1
		}
		tagRes[parts[0]] = tagRe
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
		allTagsMatch := true
		for tag, re := range tagRes {
			value, ok := member.Tags[tag]
			if !ok || !re.MatchString(value) {
				allTagsMatch = false
			}
		}

		// Skip the non-matching members
		if !allTagsMatch || !roleRe.MatchString(member.Tags["role"]) || !statusRe.MatchString(member.Status) {
			continue
		}

		addr := net.TCPAddr{IP: member.Addr, Port: int(member.Port)}

		result.Members = append(result.Members, Member{
			detail: detailed,
			Name:   member.Name,
			Addr:   addr.String(),
			Port:   member.Port,
			Tags:   member.Tags,
			Status: member.Status,
			Proto: map[string]uint8{
				"min":     member.DelegateMin,
				"max":     member.DelegateMax,
				"version": member.DelegateCur,
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
