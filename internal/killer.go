package internal

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type Killer struct{}

func NewKiller() *Killer {
	return &Killer{}
}

func (k *Killer) KillSession(session *SSHSession) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}

	pid := session.PIDSSH
	if pid == 0 {
		pid = session.PID
	}

	if pid == 0 {
		return fmt.Errorf("cannot find PID for session %s on %s", session.User, session.TTY)
	}

	cmd := exec.Command("kill", "-9", strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}

	return nil
}

func (k *Killer) KillByUser(username string) ([]SSHSession, error) {
	scanner := NewScanner()
	sessions, err := scanner.ScanSessions()
	if err != nil {
		return nil, err
	}

	var killed []SSHSession
	for _, session := range sessions {
		if session.User == username {
			if err := k.KillSession(&session); err == nil {
				killed = append(killed, session)
			}
		}
	}

	return killed, nil
}

func (k *Killer) KillByIP(ip string) ([]SSHSession, error) {
	scanner := NewScanner()
	sessions, err := scanner.ScanSessions()
	if err != nil {
		return nil, err
	}

	var killed []SSHSession
	for _, session := range sessions {
		if session.IP == ip {
			if err := k.KillSession(&session); err == nil {
				killed = append(killed, session)
			}
		}
	}

	return killed, nil
}

func (k *Killer) KillByTTY(tty string) error {
	pid := findPIDByTTY(tty)
	if pid == 0 {
		return fmt.Errorf("cannot find process for tty %s", tty)
	}

	cmd := exec.Command("kill", "-9", strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}

	return nil
}

func (k *Killer) KillByPID(pid int) error {
	cmd := exec.Command("kill", "-9", strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to kill process %d: %w", pid, err)
	}
	return nil
}

func findPIDByTTY(tty string) int {
	cmd := exec.Command("lsof", "-t", tty)
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, pidStr := range pids {
		pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
		if err == nil && pid > 0 {
			cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
			out, _ := cmd.Output()
			if strings.Contains(string(out), "sshd") {
				return pid
			}
		}
	}

	cmd = exec.Command("ps", "aux")
	output, err = cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "sshd:") && strings.Contains(line, tty) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				pid, err := strconv.Atoi(fields[1])
				if err == nil {
					return pid
				}
			}
		}
	}

	return 0
}

func (k *Killer) SendSignal(pid int, sig syscall.Signal) error {
	cmd := exec.Command("kill", "-"+strconv.Itoa(int(sig)), strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send signal to process %d: %w", pid, err)
	}
	return nil
}
