package pbr

import (
	"bufio"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type CommandExecutor interface {
	Execute(name string, args ...string) ([]byte, error)
}

type DefaultExecutor struct{}

func (e *DefaultExecutor) Execute(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

type Policy struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	SrcAddr   string `json:"src_addr"`
	Interface string `json:"interface"`
	Enabled   bool   `json:"enabled"`
}

func ParseUCIOutput(output string) ([]Policy, error) {
	policies := make(map[int]*Policy)

	re := regexp.MustCompile(`pbr\.@policy\[(\d+)\]\.(\w+)='([^']*)'`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		index, _ := strconv.Atoi(matches[1])
		field := matches[2]
		value := matches[3]

		if _, exists := policies[index]; !exists {
			policies[index] = &Policy{Index: index, Enabled: true}
		}

		switch field {
		case "name":
			policies[index].Name = value
		case "src_addr":
			policies[index].SrcAddr = value
		case "interface":
			policies[index].Interface = value
		case "enabled":
			policies[index].Enabled = value == "1"
		}
	}

	maxIndex := -1
	for idx := range policies {
		if idx > maxIndex {
			maxIndex = idx
		}
	}

	result := make([]Policy, 0, len(policies))
	for i := 0; i <= maxIndex; i++ {
		if p, exists := policies[i]; exists {
			result = append(result, *p)
		}
	}

	return result, scanner.Err()
}

func FindPolicyIndex(policies []Policy, ip, iface string) int {
	for _, p := range policies {
		if p.SrcAddr == ip && p.Interface == iface {
			return p.Index
		}
	}
	return -1
}
