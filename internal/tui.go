package internal

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type TUI struct {
	app              *tview.Application
	pages            *tview.Pages
	sessionList      *tview.Table
	monitorView      *tview.TextView
	scanner          *Scanner
	killer           *Killer
	banner           *Banner
	config           *MonitorConfig
	Sessions         []SSHSession
	refreshDone      chan bool
	shouldRefresh    chan bool
	selectedForKill  *SSHSession
}

func NewTUI() *TUI {
	return &TUI{
		scanner:       NewScanner(),
		killer:        NewKiller(),
		banner:         NewBanner(),
		refreshDone:    make(chan bool),
		shouldRefresh:  make(chan bool),
	}
}

func (t *TUI) SetConfig(cfg *MonitorConfig) {
	t.config = cfg
}

func (t *TUI) RunMonitor() error {
	t.app = tview.NewApplication()
	t.pages = tview.NewPages()

	t.sessionList = tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false)

	t.monitorView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)

	t.createMainView()
	t.createConfirmModal()

	if err := t.app.SetRoot(t.pages, true).EnableMouse(true).Run(); err != nil {
		return err
	}

	return nil
}

func (t *TUI) createMainView() {
	header := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText("[yellow]SSH Manager - Monitor Panel[white]\n" +
			"[gray]Press [white]q[gray] to quit | [white]r[gray] to refresh | [white]k[gray] to kill | [white]b[gray] to ban IP | [white]Tab[gray] to switch panel")

	t.monitorView.SetBorder(true).SetTitle("Status")

	footer := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText("[gray]Total: 0 | Idle: 0 | Auto-kill: OFF | Threshold: 30m")

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 3, 0, false).
		AddItem(t.sessionList, 0, 1, true).
		AddItem(t.monitorView, 8, 0, false).
		AddItem(footer, 3, 0, false)

	t.pages.AddPage("main", flex, true, true)

	t.updateTable()
	t.updateStatus()

	t.sessionList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q', 'Q':
				t.app.Stop()
			case 'r', 'R':
				t.updateTable()
				t.updateStatus()
			case 'k', 'K':
				t.showKillDialog()
			case 'b', 'B':
				t.showBanDialog()
			}
		}
		return event
	})

	go t.autoRefresh()
}

func (t *TUI) createConfirmModal() {
	modal := tview.NewModal().
		SetText("Are you sure you want to kill this session?").
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, label string) {
			if buttonIndex == 0 {
				t.confirmKill()
			}
			t.pages.SwitchToPage("main")
		})

	t.pages.AddPage("confirm", modal, true, false)
}

func (t *TUI) updateTable() {
	sessions, err := t.scanner.ScanSessions()
	if err != nil {
		t.monitorView.SetText(fmt.Sprintf("[red]Error: %v", err))
		return
	}

	t.Sessions = sessions
	sort.Sort(SSHSessionList(t.Sessions))

	t.sessionList.Clear()

	headers := []string{"#", "PID", "USER", "TTY", "IP", "LOGIN TIME", "IDLE"}
	for i, h := range headers {
		t.sessionList.SetCell(0, i, &tview.TableCell{
			Text:  h,
			Color: tcell.ColorYellow,
			Align: tview.AlignCenter,
		})
	}

	for idx, session := range sessions {
		row := idx + 1
		idleColor := tcell.ColorGreen
		if session.IdleTime > time.Duration(t.config.IdleThreshold)*time.Minute {
			idleColor = tcell.ColorRed
		}

		loginStr := session.LoginTime.Format("15:04:05")
		idleStr := session.GetDisplayIdle()

		cells := []*tview.TableCell{
			{Text: strconv.Itoa(row), Align: tview.AlignCenter},
			{Text: strconv.Itoa(session.PID), Align: tview.AlignCenter},
			{Text: session.User, Align: tview.AlignCenter},
			{Text: session.TTY, Align: tview.AlignCenter},
			{Text: session.IP, Align: tview.AlignCenter},
			{Text: loginStr, Align: tview.AlignCenter},
			{Text: idleStr, Color: idleColor, Align: tview.AlignCenter},
		}

		for col, cell := range cells {
			t.sessionList.SetCell(row, col, cell)
		}
	}
}

func (t *TUI) updateStatus() {
	total := len(t.Sessions)
	idleCount := 0
	for _, s := range t.Sessions {
		if s.IdleTime > time.Duration(t.config.IdleThreshold)*time.Minute {
			idleCount++
		}
	}

	autoKillStr := "[red]OFF"
	if t.config.AutoKillIdle {
		autoKillStr = "[green]ON"
	}

	status := fmt.Sprintf("[yellow]Total: [white]%d[yellow] | Idle: [white]%d[yellow] | Auto-kill: %s[yellow] | Threshold: [white]%dm",
		total, idleCount, autoKillStr, t.config.IdleThreshold)

	t.monitorView.SetText(status)

	if t.config.AutoKillIdle && idleCount > 0 {
		t.autoKillIdle()
	}
}

func (t *TUI) showKillDialog() {
	row, _ := t.sessionList.GetSelection()
	if row < 1 || row > len(t.Sessions) {
		t.monitorView.SetText("[red]No session selected")
		return
	}

	session := t.Sessions[row-1]
	t.monitorView.SetText(fmt.Sprintf("[yellow]Selected: [white]%s[yellow] from [white]%s[yellow] (PID: %d)",
		session.User, session.IP, session.PID))

	t.selectedForKill = &session
	t.pages.SwitchToPage("confirm")
}

func (t *TUI) showBanDialog() {
	row, _ := t.sessionList.GetSelection()
	if row < 1 || row > len(t.Sessions) {
		t.monitorView.SetText("[red]No session selected")
		return
	}

	session := t.Sessions[row-1]
	t.monitorView.SetText(fmt.Sprintf("[yellow]Ban IP: [white]%s", session.IP))

	if err := t.banner.BanIP(session.IP); err != nil {
		t.monitorView.SetText(fmt.Sprintf("[red]Failed to ban IP: %v", err))
	} else {
		t.monitorView.SetText(fmt.Sprintf("[green]IP %s has been banned", session.IP))
	}
}

func (t *TUI) autoKillIdle() {
	for _, session := range t.Sessions {
		if session.IdleTime > time.Duration(t.config.IdleThreshold)*time.Minute {
			if err := t.killer.KillSession(&session); err == nil {
				t.monitorView.SetText(fmt.Sprintf("[yellow]Auto-killed idle session: [white]%s[yellow] from [white]%s",
					session.User, session.IP))
			}
		}
	}
}

func (t *TUI) autoRefresh() {
	ticker := time.NewTicker(time.Duration(t.config.RefreshSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.app.QueueUpdateDraw(func() {
				t.updateTable()
				t.updateStatus()
			})
		case <-t.shouldRefresh:
			return
		}
	}
}

func (t *TUI) Stop() {
	t.shouldRefresh <- true
	if t.app != nil {
		t.app.Stop()
	}
}

func (t *TUI) RunList() error {
	t.app = tview.NewApplication()
	t.pages = tview.NewPages()

	t.sessionList = tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false)

	t.createListView()

	if err := t.app.SetRoot(t.pages, true).EnableMouse(true).Run(); err != nil {
		return err
	}

	return nil
}

func (t *TUI) createListView() {
	header := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText("[yellow]SSH Manager - Session List[white]\n" +
			"[gray]Press [white]q[gray] to quit | [white]r[gray] to refresh | [white]k[gray] to kill | [white]b[gray] to ban IP")

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 3, 0, false).
		AddItem(t.sessionList, 0, 1, true)

	t.pages.AddPage("main", flex, true, true)

	t.updateTable()
	t.updateStatus()

	t.sessionList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q', 'Q':
				t.app.Stop()
			case 'r', 'R':
				t.updateTable()
				t.updateStatus()
			case 'k', 'K':
				t.showKillDialog()
			case 'b', 'B':
				t.showBanDialog()
			}
		}
		return event
	})
}

func (t *TUI) confirmKill() {
	if t.selectedForKill == nil {
		return
	}

	if err := t.killer.KillSession(t.selectedForKill); err != nil {
		t.monitorView.SetText(fmt.Sprintf("[red]Failed to kill session: %v", err))
	} else {
		t.monitorView.SetText(fmt.Sprintf("[green]Session killed: %s from %s",
			t.selectedForKill.User, t.selectedForKill.IP))
	}

	t.updateTable()
}

func (t *TUI) PrintList() {
	sessions, err := t.scanner.ScanSessions()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	t.Sessions = sessions
	t.updateTable()
}
