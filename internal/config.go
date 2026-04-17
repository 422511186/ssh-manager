package internal

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	General  GeneralConfig  `yaml:"general"`
	Ban      BanConfig      `yaml:"ban"`
	Monitor  MonitorConfig   `yaml:"monitor"`
}

type GeneralConfig struct {
	CheckInterval int    `yaml:"check_interval"` // 秒
	ConfigPath    string `yaml:"config_path"`    // 规则配置文件路径
}

type BanConfig struct {
	Enabled     bool     `yaml:"enabled"`
	Users       []string `yaml:"users"`       // 禁止的用户
	IPs         []string `yaml:"ips"`         // 禁止的IP
	IPRanges    []string `yaml:"ip_ranges"`   // 禁止的IP段
	UseHostsDeny bool    `yaml:"use_hosts_deny"` // 使用 /etc/hosts.deny
	UseIptables bool     `yaml:"use_iptables"`   // 使用 iptables
}

type MonitorConfig struct {
	AutoKillIdle   bool   `yaml:"auto_kill_idle"`   // 自动断开空闲连接
	IdleThreshold  int    `yaml:"idle_threshold"`   // 空闲阈值（分钟）
	RefreshSeconds int    `yaml:"refresh_seconds"`  // 刷新间隔
}

var DefaultConfig = Config{
	General: GeneralConfig{
		CheckInterval: 5,
		ConfigPath:    "/etc/ssh-manager/ban_rules.yaml",
	},
	Ban: BanConfig{
		Enabled:      false,
		UseHostsDeny: true,
		UseIptables:  true,
		Users:        []string{},
		IPs:          []string{},
		IPRanges:     []string{},
	},
	Monitor: MonitorConfig{
		AutoKillIdle:   false,
		IdleThreshold:  30,
		RefreshSeconds: 5,
	},
}

var globalConfig *Config

func LoadConfig() (*Config, error) {
	if globalConfig != nil {
		return globalConfig, nil
	}

	configPaths := []string{
		filepath.Join(os.Getenv("HOME"), ".ssh-manager", "config.yaml"),
		"/etc/ssh-manager/config.yaml",
	}

	cfg := DefaultConfig

	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, &cfg); err == nil {
				globalConfig = &cfg
				return &cfg, nil
			}
		}
	}

	globalConfig = &cfg
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	homeDir := os.Getenv("HOME")
	configDir := filepath.Join(homeDir, ".ssh-manager")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "config.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

type BanRules struct {
	Users    []string    `yaml:"users"`
	IPs      []string    `yaml:"ips"`
	Ranges   []string    `yaml:"ip_ranges"`
	UpdatedAt time.Time  `yaml:"updated_at"`
}

func LoadBanRules(path string) (*BanRules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &BanRules{}, nil
		}
		return nil, err
	}

	var rules BanRules
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, err
	}

	return &rules, nil
}

func SaveBanRules(path string, rules *BanRules) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	rules.UpdatedAt = time.Now()
	data, err := yaml.Marshal(rules)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
