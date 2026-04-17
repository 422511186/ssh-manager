package internal

import (
	"fmt"
	"time"
)

type PortForward struct {
	Type       string // L: 本地转发, R: 远程转发, D: 动态转发
	LocalAddr  string
	LocalPort  int
	RemoteAddr string
	RemotePort int
}

type SSHSession struct {
	PID        int           // sshd 进程 PID
	PIDSSH     int           // sshd 子进程 PID (用于断开)
	User       string        // 用户名
	TTY        string        // 终端设备
	IP         string        // 远程 IP
	LoginTime  time.Time     // 登录时间
	IdleTime   time.Duration // 空闲时间
	IsIdle     bool          // 是否空闲
	Forwards   []PortForward // 端口转发列表
}

type SSHTunnel struct {
	PID           int            // sshd 进程 PID
	User          string         // 用户名
	IP            string         // 远程 IP
	LastActivity  time.Time      // 最后活动时间
	Forwards      []PortForward  // 端口转发列表
}

func DeduplicateForwards(forwards []PortForward) []PortForward {
	seen := make(map[string]bool)
	var result []PortForward

	for _, f := range forwards {
		key := fmt.Sprintf("%s:%s:%d", f.Type, f.RemoteAddr, f.RemotePort)
		if !seen[key] {
			seen[key] = true
			f.LocalPort = 0
			result = append(result, f)
		}
	}

	return result
}

func (pf PortForward) String() string {
	switch pf.Type {
	case "L":
		if pf.LocalPort == 0 {
			return fmt.Sprintf("L:* -> %s:%d", pf.RemoteAddr, pf.RemotePort)
		}
		return fmt.Sprintf("L:%s:%d -> %s:%d", pf.LocalAddr, pf.LocalPort, pf.RemoteAddr, pf.RemotePort)
	case "R":
		return fmt.Sprintf("R:%s:%d -> %s:%d", pf.RemoteAddr, pf.RemotePort, pf.LocalAddr, pf.LocalPort)
	case "D":
		return fmt.Sprintf("D:%s:%d (SOCKS)", pf.LocalAddr, pf.LocalPort)
	default:
		return fmt.Sprintf("%s:%d -> %s:%d", pf.LocalAddr, pf.LocalPort, pf.RemoteAddr, pf.RemotePort)
	}
}

type SSHSessionList []SSHSession

func (s SSHSessionList) Len() int           { return len(s) }
func (s SSHSessionList) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s SSHSessionList) Less(i, j int) bool {
	return s[i].LoginTime.Before(s[j].LoginTime)
}

func (s *SSHSession) GetDisplayIdle() string {
	if s.IdleTime < time.Minute {
		return "<1m"
	}
	minutes := int(s.IdleTime.Minutes())
	if minutes < 60 {
		return formatDuration(s.IdleTime)
	}
	hours := minutes / 60
	minutes = minutes % 60
	return formatDuration(time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute)
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return formatPad(hours) + "h" + formatPad(minutes) + "m"
	}
	if minutes > 0 {
		return formatPad(minutes) + "m" + formatPad(seconds) + "s"
	}
	return formatPad(seconds) + "s"
}

func formatPad(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
