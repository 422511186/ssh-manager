package internal

import (
	"fmt"
	"os/exec"
	"strings"
)

type IPTables struct {
	chainName string
}

func NewIPTables() *IPTables {
	return &IPTables{
		chainName: "SSH-MANAGER-BAN",
	}
}

func (i *IPTables) EnsureChain() error {
	cmd := exec.Command("iptables", "-L", i.chainName, "-n")
	if err := cmd.Run(); err != nil {
		if err := i.CreateChain(); err != nil {
			return err
		}
	}
	return nil
}

func (i *IPTables) CreateChain() error {
	cmds := []string{
		fmt.Sprintf("iptables -N %s", i.chainName),
		fmt.Sprintf("iptables -I INPUT -p tcp --dport 22 -j %s", i.chainName),
	}

	for _, cmd := range cmds {
		c := exec.Command("sh", "-c", cmd)
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to execute: %s: %w", cmd, err)
		}
	}

	return nil
}

func (i *IPTables) Flush() error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("iptables -F %s", i.chainName))
	return cmd.Run()
}

func (i *IPTables) BanIP(ip string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("iptables -A %s -s %s -j REJECT", i.chainName, ip))
	return cmd.Run()
}

func (i *IPTables) BanCIDR(cidr string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("iptables -A %s -s %s -j REJECT", i.chainName, cidr))
	return cmd.Run()
}

func (i *IPTables) UnbanIP(ip string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("iptables -D %s -s %s -j REJECT", i.chainName, ip))
	return cmd.Run()
}

func (i *IPTables) UnbanCIDR(cidr string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("iptables -D %s -s %s -j REJECT", i.chainName, cidr))
	return cmd.Run()
}

func (i *IPTables) ListRules() ([]string, error) {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("iptables -L %s -n --line-numbers", i.chainName))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var rules []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "REJECT") {
			rules = append(rules, line)
		}
	}

	return rules, nil
}

func (i *IPTables) DeleteChain() error {
	cmds := []string{
		fmt.Sprintf("iptables -D INPUT -p tcp --dport 22 -j %s", i.chainName),
		fmt.Sprintf("iptables -F %s", i.chainName),
		fmt.Sprintf("iptables -X %s", i.chainName),
	}

	for _, cmd := range cmds {
		c := exec.Command("sh", "-c", cmd)
		c.Run()
	}

	return nil
}

func (i *IPTables) IsIPBlocked(ip string) (bool, error) {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("iptables -L %s -n | grep %s", i.chainName, ip))
	err := cmd.Run()
	if err != nil {
		return false, nil
	}
	return true, nil
}
