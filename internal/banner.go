package internal

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Banner struct {
	hostsDenyPath string
	configPath    string
}

func NewBanner() *Banner {
	return &Banner{
		hostsDenyPath: "/etc/hosts.deny",
		configPath:    "/etc/ssh-manager/ban_rules.yaml",
	}
}

func (b *Banner) SetConfigPath(path string) {
	b.configPath = path
}

func (b *Banner) BanIP(ip string) error {
	rules, err := b.LoadCurrentRules()
	if err != nil {
		rules = &BanRules{}
	}

	if !contains(rules.IPs, ip) {
		rules.IPs = append(rules.IPs, ip)
	}

	return b.ApplyRules(rules)
}

func (b *Banner) IsIPBanned(ip string, rules *BanRules) bool {
	if rules == nil {
		return false
	}

	for _, bannedIP := range rules.IPs {
		if bannedIP == ip {
			return true
		}
	}

	for _, cidr := range rules.Ranges {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipNet.Contains(net.ParseIP(ip)) {
			return true
		}
	}

	return false
}

func (b *Banner) IsUserBanned(user string, rules *BanRules) bool {
	if rules == nil {
		return false
	}

	for _, bannedUser := range rules.Users {
		if bannedUser == user {
			return true
		}
	}

	return false
}

func (b *Banner) ApplyRules(rules *BanRules) error {
	if err := os.MkdirAll(filepath.Dir(b.configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := SaveBanRules(b.configPath, rules); err != nil {
		return fmt.Errorf("failed to save rules: %w", err)
	}

	if err := b.updateHostsDeny(rules); err != nil {
		return fmt.Errorf("failed to update hosts.deny: %w", err)
	}

	if err := b.updateIptables(rules); err != nil {
		return fmt.Errorf("failed to update iptables: %w", err)
	}

	return nil
}

func (b *Banner) updateHostsDeny(rules *BanRules) error {
	data, err := os.ReadFile(b.hostsDenyPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	existingLines := strings.Split(string(data), "\n")
	var filteredLines []string

	for _, line := range existingLines {
		if strings.Contains(line, "sshd:") && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			if strings.Contains(line, "# SSH-MANAGER-AUTO") {
				continue
			}
		}
		filteredLines = append(filteredLines, line)
	}

	var newLines []string
	for _, ip := range rules.IPs {
		newLines = append(newLines, fmt.Sprintf("sshd: %s # SSH-MANAGER-AUTO", ip))
	}
	for _, cidr := range rules.Ranges {
		newLines = append(newLines, fmt.Sprintf("sshd: %s # SSH-MANAGER-AUTO", cidr))
	}

	if len(newLines) > 0 {
		filteredLines = append(filteredLines, "# SSH-MANAGER-AUTO")
		filteredLines = append(filteredLines, newLines...)
		filteredLines = append(filteredLines, "# END SSH-MANAGER-AUTO")
	}

	content := strings.Join(filteredLines, "\n")
	return os.WriteFile(b.hostsDenyPath, []byte(content), 0644)
}

func (b *Banner) updateIptables(rules *BanRules) error {
	bannerChain := "SSH-MANAGER-BAN"

	checkCmd := fmt.Sprintf("iptables -L %s -n 2>/dev/null", bannerChain)
	execCmd := func(cmd string) error {
		c := exec.Command("sh", "-c", cmd)
		return c.Run()
	}

	if err := execCmd(checkCmd); err != nil {
		execCmd(fmt.Sprintf("iptables -N %s", bannerChain))
		execCmd(fmt.Sprintf("iptables -I INPUT -p tcp --dport 22 -j %s", bannerChain))
	}

	execCmd(fmt.Sprintf("iptables -F %s", bannerChain))

	for _, ip := range rules.IPs {
		execCmd(fmt.Sprintf("iptables -A %s -s %s -j REJECT", bannerChain, ip))
	}
	for _, cidr := range rules.Ranges {
		execCmd(fmt.Sprintf("iptables -A %s -s %s -j REJECT", bannerChain, cidr))
	}

	return nil
}

func (b *Banner) RemoveAllRules() error {
	rules := &BanRules{
		Users:  []string{},
		IPs:    []string{},
		Ranges: []string{},
	}
	return b.ApplyRules(rules)
}

func (b *Banner) LoadCurrentRules() (*BanRules, error) {
	return LoadBanRules(b.configPath)
}

func (b *Banner) ListCurrentBans() (*BanRules, error) {
	rules, err := b.LoadCurrentRules()
	if err != nil {
		return nil, err
	}

	hostsDenyRules, _ := b.readHostsDeny()
	for _, rule := range hostsDenyRules {
		if !contains(rules.IPs, rule) && !contains(rules.Ranges, rule) {
			rules.IPs = append(rules.IPs, rule)
		}
	}

	return rules, nil
}

func (b *Banner) readHostsDeny() ([]string, error) {
	data, err := os.ReadFile(b.hostsDenyPath)
	if err != nil {
		return nil, err
	}

	var rules []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if strings.Contains(line, "sshd:") && !strings.Contains(line, "SSH-MANAGER-AUTO") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				ip := strings.TrimSpace(parts[1])
				ip = strings.Split(ip, " ")[0]
				if net.ParseIP(ip) != nil || strings.Contains(ip, "/") {
					rules = append(rules, ip)
				}
			}
		}
	}

	return rules, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
