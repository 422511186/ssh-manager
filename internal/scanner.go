package internal

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var sshdPattern = regexp.MustCompile(`sshd:\s+(\S+)`)

type Scanner struct{}

func NewScanner() *Scanner {
	return &Scanner{}
}

func (s *Scanner) ScanSessions() ([]SSHSession, error) {
	whoSessions, err := s.scanWho()
	if err != nil {
		return nil, fmt.Errorf("scan who failed: %w", err)
	}

	psMap, err := s.scanPs()
	if err != nil {
		return nil, fmt.Errorf("scan ps failed: %w", err)
	}

	portForwards := s.scanPortForwards()

	var sessions []SSHSession
	for _, who := range whoSessions {
		session := who

		if pid, ok := psMap[who.TTY]; ok {
			session.PID = pid
		}

		session.LoginTime = time.Now().Add(-session.IdleTime)

		if forwards, ok := portForwards[session.PID]; ok {
			session.Forwards = DeduplicateForwards(forwards)
			delete(portForwards, session.PID)
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (s *Scanner) ScanTunnels() ([]SSHTunnel, error) {
	portForwards := s.scanPortForwards()
	userPids := s.getUserPids()
	sshConnections := s.scanSSHConnections()
	lastActivity := s.getLastActivity()

	var tunnels []SSHTunnel

	for pid, forwards := range portForwards {
		tunnel := SSHTunnel{
			PID:      pid,
			Forwards: DeduplicateForwards(forwards),
		}

		if activity, ok := lastActivity[pid]; ok {
			tunnel.LastActivity = activity
		}

		for user, pids := range userPids {
			for _, userPid := range pids {
				if userPid == pid {
					tunnel.User = user
					break
				}
			}
		}

		for _, conn := range sshConnections {
			if conn.PID == pid || conn.PPID == pid {
				tunnel.IP = conn.RemoteIP
				break
			}
		}

		if tunnel.User != "" && len(tunnel.Forwards) > 0 {
			tunnels = append(tunnels, tunnel)
		}
	}

	return tunnels, nil
}

type sshConnection struct {
	PID      int
	PPID     int
	RemoteIP string
}

func (s *Scanner) scanSSHConnections() []sshConnection {
	var conns []sshConnection

	cmd := exec.Command("ss", "-tnp")
	output, err := cmd.Output()
	if err != nil {
		return conns
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, ":22") || !strings.Contains(line, "sshd") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		if fields[0] != "ESTAB" {
			continue
		}

		peerAddr := ""
		for i := 5; i < len(fields); i++ {
			field := fields[i]
			if strings.Contains(field, ":") && !strings.Contains(field, "users") && !strings.Contains(field, "pid=") {
				peerAddr = field
				if idx := strings.Index(peerAddr, "("); idx > 0 {
					peerAddr = peerAddr[:idx]
				}
				break
			}
		}

		if peerAddr == "" {
			continue
		}

		var peerIP string
		if idx := strings.LastIndex(peerAddr, ":"); idx > 0 {
			peerIP = peerAddr[:idx]
		}

		if strings.HasPrefix(peerIP, "::ffff:") {
			peerIP = peerIP[7:]
		}

		if peerIP == "" || peerIP == "*" {
			continue
		}

		var pid int
		for _, field := range fields[5:] {
			if strings.Contains(field, "pid=") {
				parts := strings.Split(field, ",")
				for _, p := range parts {
					if strings.HasPrefix(p, "pid=") {
						fmt.Sscanf(strings.TrimPrefix(p, "pid="), "%d", &pid)
					}
					if strings.HasPrefix(p, "fd=") {
						continue
					}
				}
			}
		}

		if pid > 0 {
			conns = append(conns, sshConnection{
				PID:      pid,
				PPID:     0,
				RemoteIP: peerIP,
			})
		}
	}

	cmd = exec.Command("ps", "ax", "-o", "pid=,ppid=,comm=")
	output, _ = cmd.Output()
	psScanner := bufio.NewScanner(strings.NewReader(string(output)))
	for psScanner.Scan() {
		psLine := psScanner.Text()
		fields := strings.Fields(psLine)
		if len(fields) < 3 {
			continue
		}

		pid, _ := strconv.Atoi(fields[0])
		ppid, _ := strconv.Atoi(fields[1])
		comm := fields[2]

		if strings.Contains(comm, "sshd") && ppid > 0 {
			for i := range conns {
				if conns[i].PID == ppid {
					conns[i].PPID = pid
				}
			}
		}
	}

	return conns
}

func (s *Scanner) getForwardUsers(forwards map[int][]PortForward) map[int]string {
	result := make(map[int]string)

	cmd := exec.Command("ps", "aux")
	output, _ := cmd.Output()

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "sshd") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		user := fields[0]
		pid, _ := strconv.Atoi(fields[1])

		if _, ok := forwards[pid]; ok {
			result[pid] = user
		}
	}

	return result
}

func (s *Scanner) getUserPids() map[string][]int {
	result := make(map[string][]int)

	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return result
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "sshd:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		user := fields[0]
		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		result[user] = append(result[user], pid)
	}

	return result
}

func (s *Scanner) findForwardsForUser(user string, forwards map[int][]PortForward, userPids map[string][]int) []PortForward {
	var allForwards []PortForward

	for _, pid := range userPids[user] {
		if fwd, ok := forwards[pid]; ok {
			allForwards = append(allForwards, fwd...)
		}
	}

	return DeduplicateForwards(allForwards)
}

func (s *Scanner) getLastActivity() map[int]time.Time {
	result := make(map[int]time.Time)

	cmd := exec.Command("ss", "-tnip")
	output, err := cmd.Output()
	if err != nil {
		return result
	}

	var currentPid int
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "users:") && strings.Contains(line, "sshd") {
			if strings.Contains(line, "pid=") {
				parts := strings.Split(line, "pid=")
				if len(parts) >= 2 {
					pidStr := strings.Trim(parts[1], "(), ")
					for i, c := range pidStr {
						if c < '0' || c > '9' {
							pidStr = pidStr[:i]
							break
						}
					}
					fmt.Sscanf(pidStr, "%d", &currentPid)
				}
			}
			continue
		}

		if strings.Contains(line, "lastrcv:") && currentPid > 0 {
			idx := strings.Index(line, "lastrcv:")
			remaining := line[idx+8:]
			var lastrcvMs int
			for i, c := range remaining {
				if c < '0' || c > '9' {
					fmt.Sscanf(remaining[:i], "%d", &lastrcvMs)
					break
				}
			}

			if lastrcvMs > 0 {
				result[currentPid] = time.Now().Add(-time.Duration(lastrcvMs) * time.Millisecond)
			}
		}
	}

	return result
}

func (s *Scanner) scanPortForwards() map[int][]PortForward {
	result := make(map[int][]PortForward)

	cmd := exec.Command("ss", "-tnp")
	output, err := cmd.Output()
	if err != nil {
		return result
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "sshd") {
			continue
		}

		if !strings.Contains(line, "ESTAB") {
			continue
		}

		var pid int
		if strings.Contains(line, "pid=") {
			parts := strings.Split(line, "pid=")
			if len(parts) >= 2 {
				pidStr := strings.Trim(parts[1], "(), ")
				for i, c := range pidStr {
					if c < '0' || c > '9' {
						pidStr = pidStr[:i]
						break
					}
				}
				fmt.Sscanf(pidStr, "%d", &pid)
			}
		}

		if pid == 0 {
			continue
		}

		parts := strings.Split(line, "users:")
		if len(parts) < 1 {
			continue
		}

		addrPart := strings.TrimSpace(parts[0])
		addrFields := strings.Fields(addrPart)

		if len(addrFields) < 3 {
			continue
		}

		localAddr := addrFields[len(addrFields)-2]
		peerAddr := addrFields[len(addrFields)-1]

		if strings.Contains(localAddr, ":22") {
			continue
		}

		var localIP string
		var localPort int
		if idx := strings.LastIndex(localAddr, ":"); idx > 0 {
			localIP = localAddr[:idx]
			fmt.Sscanf(localAddr[idx+1:], "%d", &localPort)
		}

		if localIP == "0.0.0.0" || localIP == "*" || localIP == "[::]" {
			localIP = "0.0.0.0"
		} else if localIP == "::ffff:127.0.0.1" || strings.HasPrefix(localIP, "::ffff:127.0.0.1") {
			localIP = "127.0.0.1"
		} else if strings.HasPrefix(localIP, "::ffff:") {
			localIP = localIP[7:]
		}

		var peerIP string
		var peerPort int
		if idx := strings.LastIndex(peerAddr, ":"); idx > 0 {
			peerIP = peerAddr[:idx]
			fmt.Sscanf(peerAddr[idx+1:], "%d", &peerPort)
		}

		if strings.HasPrefix(peerIP, "::ffff:") {
			peerIP = peerIP[7:]
		}

		pType := "L"
		remoteAddr := peerIP
		remotePort := peerPort

		if localIP == "127.0.0.1" && peerIP == "127.0.0.1" {
			pType = "L"
			remoteAddr = "127.0.0.1"
		} else if localPort > 1024 && peerPort == 0 {
			pType = "D"
			remoteAddr = "*"
			remotePort = 0
		}

		pf := PortForward{
			Type:       pType,
			LocalAddr:  localIP,
			LocalPort:  localPort,
			RemoteAddr: remoteAddr,
			RemotePort: remotePort,
		}

		result[pid] = append(result[pid], pf)
	}

	return result
}

func (s *Scanner) scanWho() ([]SSHSession, error) {
	cmd := exec.Command("who", "-u")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var sessions []SSHSession
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 7 {
			continue
		}

		user := parts[0]
		tty := parts[1]

		if !strings.HasPrefix(tty, "pts/") {
			continue
		}

		dateStr := parts[2] + " " + parts[3]
		loginTime, err := time.ParseInLocation("2006-01-02 15:04", dateStr, time.Local)
		if err != nil {
			loginTime = time.Now()
		}

		var ip string
		if len(parts) >= 7 {
			ip = strings.Trim(parts[6], "()")
			if net.ParseIP(ip) == nil {
				ip = ""
			}
		}

		idleStr := parts[4]
		idleTime := time.Duration(0)
		if idleStr != "." {
			if idleStr == "old" {
				idleTime = 0
			} else {
				idleParts := strings.Split(idleStr, ":")
				if len(idleParts) == 2 {
					minutes, _ := strconv.Atoi(idleParts[0])
					idleTime = time.Duration(minutes) * time.Minute
				}
			}
		} else {
			idleTime = time.Since(loginTime)
		}

		sessions = append(sessions, SSHSession{
			User:      user,
			TTY:       tty,
			IP:        ip,
			LoginTime: loginTime,
			IdleTime:  idleTime,
		})
	}

	return sessions, nil
}

func (s *Scanner) scanPs() (map[string]int, error) {
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	result := make(map[string]int)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "sshd:") || !strings.Contains(line, "@") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		tty := fields[6]
		if strings.HasPrefix(tty, "/dev/") {
			tty = strings.TrimPrefix(tty, "/dev/")
		}

		if strings.HasPrefix(tty, "pts/") {
			cmdParts := strings.Split(fields[10], ",")
			for _, part := range cmdParts {
				matches := sshdPattern.FindStringSubmatch(part)
				if len(matches) >= 2 && strings.Contains(matches[1], "@") {
					ptsPart := strings.Split(matches[1], "@")[1]
					result["pts/"+strings.TrimPrefix(ptsPart, "pts/")] = pid
					result["pts/"+ptsPart] = pid
				}
			}

			if strings.HasPrefix(fields[10], "pts/") {
				result[fields[10]] = pid
			}
		}
	}

	cmd2 := exec.Command("ps", "aux")
	output2, err := cmd2.Output()
	if err != nil {
		return result, nil
	}

	scanner2 := bufio.NewScanner(strings.NewReader(string(output2)))
	for scanner2.Scan() {
		line := scanner2.Text()
		if !strings.Contains(line, "@pts/") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		for _, f := range fields[10:] {
			if strings.Contains(f, "@pts/") {
				parts := strings.Split(f, "@")
				if len(parts) >= 2 {
					ptses := strings.Split(parts[1], ",")
					for _, pts := range ptses {
						pts = strings.TrimSpace(pts)
						if strings.HasPrefix(pts, "pts/") {
							result[pts] = pid
						}
					}
				}
			}
		}
	}

	return result, nil
}

func (s *Scanner) GetSSHDPID(tty string) int {
	cmd := exec.Command("lsof", "-t", "-i", "@:"+tty)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	pidStr := strings.TrimSpace(string(output))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0
	}

	return pid
}
