package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"omp-cli/internal/config"
	"omp-cli/internal/core"
	"omp-cli/internal/version"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"
)

type tickMsg time.Time
type queryResultMsg struct{ results []core.ServerInfo }

type SortMode int

const (
	SortPlayersDesc SortMode = iota
	SortPingAsc
	SortName
)

type TabMode int

const (
	TabGlobal TabMode = iota
	TabFavorites
)

type InputMode int

const (
	InputNone InputMode = iota
	InputSearch
	InputAddIP
	InputRcon
	InputServerPassword
	InputServerPasswordAndLaunch
	InputNickname
)

type model struct {
	cfg         *config.AppConfig
	apiServers  []core.OpenMpServer
	favsData    *config.FavoritesData
	viewList    []core.OpenMpServer
	liveData    map[string]core.ServerInfo
	pingHistory map[string][]float64

	activeTab   TabMode
	cursor      int
	scroll      int
	width       int
	height      int
	visibleRows int

	inputMode   InputMode
	textInput   textinput.Model
	sortMode    SortMode
	statusMsg   string
	selectedVer string
	isQuerying  bool

	launchCb func(string, uint16, string, string, string) error
}

func InitialModel(cfg *config.AppConfig, servers []core.OpenMpServer, launcher func(string, uint16, string, string, string) error) model {
	ti := textinput.New()
	ti.CharLimit = 50
	ti.Width = 40

	favs, _ := config.LoadFavorites()

	m := model{
		cfg:         cfg,
		apiServers:  servers,
		favsData:    favs,
		liveData:    make(map[string]core.ServerInfo),
		pingHistory: make(map[string][]float64),
		textInput:   ti,
		sortMode:    SortPlayersDesc,
		activeTab:   TabFavorites,
		selectedVer: cfg.DefaultVersion,
		isQuerying:  false,
		launchCb:    launcher,
	}
	m.applyFiltersAndSort()
	return m
}

func (m *model) applyFiltersAndSort() {
	query := strings.ToLower(m.textInput.Value())
	var filtered []core.OpenMpServer

	if m.activeTab == TabGlobal {
		for _, s := range m.apiServers {
			if m.inputMode != InputSearch || query == "" ||
				strings.Contains(strings.ToLower(s.Hn), query) ||
				strings.Contains(strings.ToLower(s.Gm), query) {
				filtered = append(filtered, s)
			}
		}
	} else {
		for target := range m.favsData.Servers {
			var base core.OpenMpServer
			found := false
			for _, s := range m.apiServers {
				if s.IP == target {
					base = s
					found = true
					break
				}
			}
			if !found {
				base = core.OpenMpServer{IP: target, Hn: target}
			}

			if live, ok := m.liveData[target]; ok && live.Error == "" {
				base.Hn = live.Hostname
				base.Gm = live.Gamemode
				base.Pa = live.Password
			}

			if m.inputMode != InputSearch || query == "" || strings.Contains(strings.ToLower(base.Hn), query) {
				filtered = append(filtered, base)
			}
		}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		s1, s2 := filtered[i], filtered[j]
		live1, ok1 := m.liveData[s1.IP]
		live2, ok2 := m.liveData[s2.IP]

		switch m.sortMode {
		case SortPingAsc:
			p1, p2 := uint32(9999), uint32(9999)
			if ok1 && live1.Error == "" {
				p1 = live1.PingMs
			}
			if ok2 && live2.Error == "" {
				p2 = live2.PingMs
			}
			return p1 < p2
		case SortName:
			return strings.ToLower(s1.Hn) < strings.ToLower(s2.Hn)
		default:
			p1, p2 := s1.Pc, s2.Pc
			if ok1 && live1.Error == "" {
				p1 = live1.Players
			}
			if ok2 && live2.Error == "" {
				p2 = live2.Players
			}
			return p1 > p2
		}
	})

	m.viewList = filtered
	if m.cursor >= len(m.viewList) {
		m.cursor = 0
	}
	if len(m.viewList) == 0 {
		m.cursor = 0
		m.scroll = 0
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*1, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func queryVisibleServersCmd(targets []string) tea.Cmd {
	return func() tea.Msg {
		res, _ := core.QueryBatch(targets)
		return queryResultMsg{results: res}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.visibleRows = m.height - 9
		if m.visibleRows < 5 {
			m.visibleRows = 5
		}

	case tea.KeyMsg:
		if m.inputMode != InputNone {
			switch msg.String() {
			case "esc":
				m.inputMode = InputNone
				m.textInput.EchoMode = textinput.EchoNormal
				m.textInput.Blur()
				m.textInput.Reset()
				m.applyFiltersAndSort()
			case "enter":
				val := m.textInput.Value()
				if m.inputMode == InputAddIP && val != "" {
					if !strings.Contains(val, ":") {
						val += ":7777"
					}
					m.favsData.Servers[val] = config.FavoriteServer{}
					config.SaveFavorites(m.favsData)
					m.statusMsg = "Added " + val + " to Favorites."
					m.activeTab = TabFavorites

				} else if m.inputMode == InputServerPassword || m.inputMode == InputServerPasswordAndLaunch {
					if len(m.viewList) > 0 {
						target := m.viewList[m.cursor].IP
						enc, _ := config.EncryptAES(val)
						fav := m.favsData.Servers[target]
						fav.ServerPassword = enc
						m.favsData.Servers[target] = fav
						config.SaveFavorites(m.favsData)
						m.statusMsg = "Server password securely saved."

						if m.inputMode == InputServerPasswordAndLaunch {
							parts := strings.Split(target, ":")
							port, _ := strconv.Atoi(parts[1])

							m.statusMsg = "Launching game on " + target + " [" + m.selectedVer + "]..."
							err := m.launchCb(parts[0], uint16(port), m.cfg.DefaultName, m.selectedVer, val)
							if err != nil {
								m.statusMsg = "Launch failed: " + err.Error()
							} else {
								m.statusMsg = "Game launched successfully! (" + target + ")"
							}
						}
					}

				} else if m.inputMode == InputRcon && val != "" && len(m.viewList) > 0 {
					target := m.viewList[m.cursor].IP

					enc, _ := config.EncryptAES(val)
					fav := m.favsData.Servers[target]
					fav.RconPassword = enc
					m.favsData.Servers[target] = fav
					config.SaveFavorites(m.favsData)
					m.statusMsg = "RCON Password securely encrypted and saved."

				} else if m.inputMode == InputNickname && val != "" {
					m.cfg.DefaultName = val
					config.Save(m.cfg)
					m.statusMsg = "Nickname changed to: " + val
				}

				m.inputMode = InputNone
				m.textInput.EchoMode = textinput.EchoNormal
				m.textInput.Blur()
				m.textInput.Reset()
				m.applyFiltersAndSort()
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				if m.inputMode == InputSearch {
					m.applyFiltersAndSort()
				}
				return m, cmd
			}
			return m, nil
		}

		m.statusMsg = ""
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			if m.cursor < m.scroll {
				m.scroll = m.cursor
			}
		case "down", "j":
			if m.cursor < len(m.viewList)-1 {
				m.cursor++
			}
			if m.cursor >= m.scroll+m.visibleRows {
				m.scroll = m.cursor - m.visibleRows + 1
			}
		case "tab", "right", "left":
			if m.activeTab == TabGlobal {
				m.activeTab = TabFavorites
			} else {
				m.activeTab = TabGlobal
			}
			m.textInput.Reset()
			m.applyFiltersAndSort()
		case "f":
			m.inputMode = InputSearch
			m.textInput.Placeholder = "Search..."
			m.textInput.Focus()
			return m, textinput.Blink
		case "a":
			m.inputMode = InputAddIP
			m.textInput.Placeholder = "Enter IP:PORT manually..."
			m.textInput.Focus()
			return m, textinput.Blink
		case "d", "delete":
			if len(m.viewList) > 0 {
				target := m.viewList[m.cursor].IP
				if _, exists := m.favsData.Servers[target]; exists {
					delete(m.favsData.Servers, target)
					config.SaveFavorites(m.favsData)
					m.statusMsg = "Removed from Favorites."
					m.applyFiltersAndSort()
				} else {
					m.statusMsg = "Server is not in favorites."
				}
			}
		case "v":
			versions := version.AvailableVersions
			found := false
			for i, v := range versions {
				if v == m.selectedVer {
					next := (i + 1) % len(versions)
					m.selectedVer = versions[next]
					found = true
					break
				}
			}
			if !found && len(versions) > 0 {
				m.selectedVer = versions[0]
			}
			m.statusMsg = "Switched version to " + m.selectedVer

			m.cfg.DefaultVersion = m.selectedVer
			config.Save(m.cfg)
		case "p":
			if m.activeTab == TabFavorites && len(m.viewList) > 0 {
				m.inputMode = InputServerPassword
				m.textInput.Placeholder = "Enter server password..."
				m.textInput.EchoMode = textinput.EchoPassword
				m.textInput.EchoCharacter = '*'
				m.textInput.Focus()
				return m, textinput.Blink
			} else {
				m.statusMsg = "Passwords can only be saved on favorited servers."
			}
		case "r":
			if m.activeTab == TabFavorites && len(m.viewList) > 0 {
				m.inputMode = InputRcon
				m.textInput.Placeholder = "Enter RCON password..."
				m.textInput.Focus()
				return m, textinput.Blink
			} else {
				m.statusMsg = "RCON can only be set on favorited servers."
			}
		case "n":
			m.inputMode = InputNickname
			m.textInput.Placeholder = "Enter new nickname..."
			m.textInput.SetValue(m.cfg.DefaultName)
			m.textInput.Focus()
			return m, textinput.Blink
		case "b":
			if len(m.viewList) > 0 {
				target := m.viewList[m.cursor].IP
				if _, exists := m.favsData.Servers[target]; exists {
					delete(m.favsData.Servers, target)
					m.statusMsg = "Removed from Favorites."
				} else {
					m.favsData.Servers[target] = config.FavoriteServer{}
					m.statusMsg = "Added to Favorites."
				}
				config.SaveFavorites(m.favsData)
				m.applyFiltersAndSort()
			}
		case "i":
			count := config.ImportSAMPFavorites(m.favsData)
			config.SaveFavorites(m.favsData)
			m.statusMsg = fmt.Sprintf("Imported %d servers from USERDATA.DAT", count)
			m.applyFiltersAndSort()
		case "s":
			m.sortMode = (m.sortMode + 1) % 3
			m.applyFiltersAndSort()
		case "enter":
			if len(m.viewList) > 0 {
				target := m.viewList[m.cursor].IP
				parts := strings.Split(target, ":")
				port, _ := strconv.Atoi(parts[1])

				isLocked := m.viewList[m.cursor].Pa
				if live, ok := m.liveData[target]; ok && live.Error == "" {
					isLocked = live.Password // Fallback to live data lock status
				}

				password := ""
				if fav, exists := m.favsData.Servers[target]; exists && fav.ServerPassword != "" {
					decrypted, err := config.DecryptAES(fav.ServerPassword)
					if err == nil {
						password = decrypted
					}
				}

				if isLocked && password == "" {
					m.inputMode = InputServerPasswordAndLaunch
					m.textInput.Placeholder = "Locked server. Enter password..."
					m.textInput.EchoMode = textinput.EchoPassword
					m.textInput.EchoCharacter = '*'
					m.textInput.Focus()
					return m, textinput.Blink
				}

				m.statusMsg = "Launching game on " + target + " [" + m.selectedVer + "]..."

				err := m.launchCb(parts[0], uint16(port), m.cfg.DefaultName, m.selectedVer, password)
				if err != nil {
					m.statusMsg = "Launch failed: " + err.Error()
				} else {
					m.statusMsg = "Game launched successfully! (" + target + ")"
				}

				return m, nil
			}
		}

	case queryResultMsg:
		m.isQuerying = false // Unlock state to allow next batch query
		for _, res := range msg.results {
			if res.Target == "" {
				continue
			}
			m.liveData[res.Target] = res
			if res.Error == "" {
				m.pingHistory[res.Target] = append(m.pingHistory[res.Target], float64(res.PingMs))
				if len(m.pingHistory[res.Target]) > 20 {
					m.pingHistory[res.Target] = m.pingHistory[res.Target][1:]
				}
			}
		}
		if m.activeTab == TabFavorites {
			m.applyFiltersAndSort()
		}

	case tickMsg:
		cmds = append(cmds, tickCmd())
		if !m.isQuerying {
			var targets []string
			for i := m.scroll; i < m.scroll+m.visibleRows && i < len(m.viewList); i++ {
				targets = append(targets, m.viewList[i].IP)
			}
			if len(targets) > 0 {
				m.isQuerying = true
				cmds = append(cmds, queryVisibleServersCmd(targets))
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	leftWidth := m.width/2 - 2
	rightWidth := m.width/2 - 2

	listStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("63")).Width(leftWidth).Height(m.height-7).Padding(0, 1)
	graphStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("212")).Width(rightWidth).Height(m.height-7).Padding(0, 1)
	tabStyle := lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle := tabStyle.Background(lipgloss.Color("63")).Foreground(lipgloss.Color("255")).Bold(true)
	inactiveTabStyle := tabStyle.Foreground(lipgloss.Color("241"))

	favTab := inactiveTabStyle.Render("Favorites")
	globTab := inactiveTabStyle.Render("Global Servers")
	if m.activeTab == TabGlobal {
		globTab = activeTabStyle.Render("Global Servers")
	} else {
		favTab = activeTabStyle.Render("Favorites")
	}
	tabs := lipgloss.JoinHorizontal(lipgloss.Top, favTab, "  ", globTab)
	sortStr := "Players"
	if m.sortMode == SortPingAsc {
		sortStr = "Ping"
	} else if m.sortMode == SortName {
		sortStr = "Name"
	}

	infoBar := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		fmt.Sprintf("Nick: %s  |  Sort: %s", m.cfg.DefaultName, sortStr),
	)

	spaces := m.width - lipgloss.Width(tabs) - lipgloss.Width(infoBar) - 4
	if spaces < 2 {
		spaces = 2
	}
	topBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs, strings.Repeat(" ", spaces), infoBar)

	headerContent := " "
	if m.inputMode != InputNone {
		headerContent = " > " + m.textInput.View()
	} else if m.statusMsg != "" {
		headerContent = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(" ! " + m.statusMsg)
	}

	header := lipgloss.JoinVertical(lipgloss.Left, topBar, headerContent)

	var rows []string
	if len(m.viewList) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No servers found."))
	}

	end := m.scroll + m.visibleRows
	if end > len(m.viewList) {
		end = len(m.viewList)
	}

	for i := m.scroll; i < end; i++ {
		s := m.viewList[i]
		cursor := "  "

		status := fmt.Sprintf("(%d/%d)", s.Pc, s.Pm)
		hostname := s.Hn
		isLocked := s.Pa

		if live, ok := m.liveData[s.IP]; ok {
			if live.Error != "" {
				status = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("[OFFLINE]")
			} else {
				status = fmt.Sprintf("(%d/%d) %dms", live.Players, live.MaxPlayers, live.PingMs)
				isLocked = live.Password
			}
		}

		favIcon := " "
		if _, exists := m.favsData.Servers[s.IP]; exists {
			favIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("★")
		}

		lockIcon := ""
		if isLocked {
			lockIcon = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("🔒 ")
		}

		maxHnLen := leftWidth - 30
		if maxHnLen > 0 && len(hostname) > maxHnLen {
			hostname = hostname[:maxHnLen] + ".."
		}

		line := fmt.Sprintf("%s %s%s %s", favIcon, lockIcon, hostname, status)
		if m.cursor == i {
			cursor = "> "
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true).Render(line)
		}
		rows = append(rows, cursor+line)
	}

	details := "No server selected"
	graph := ""

	if len(m.viewList) > 0 {
		selected := m.viewList[m.cursor]
		live, hasLive := m.liveData[selected.IP]

		gm, lang := selected.Gm, selected.La
		if hasLive && live.Error == "" {
			gm, lang = live.Gamemode, live.Language
		}

		favStatus := ""
		if fav, exists := m.favsData.Servers[selected.IP]; exists {
			favStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("\n[Bookmarked]")
			if fav.ServerPassword != "" {
				favStatus += lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(" (Pass Saved)")
			}
			if fav.RconPassword != "" {
				favStatus += lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(" (RCON Saved)")
			}
		}

		headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
		details = fmt.Sprintf(
			"%s\nIP: %s\nMode: %s\nLanguage: %s%s\n",
			headerStyle.Render(selected.Hn), selected.IP, gm, lang, favStatus,
		)

		if hasLive && live.Error != "" {
			details += lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("\nStatus: OFFLINE\n" + live.Error)
		} else {
			pingStr := "Waiting..."
			if hasLive {
				pingStr = fmt.Sprintf("%dms", live.PingMs)
			}
			details += fmt.Sprintf("\nPing: %s\n\n", pingStr)

			history := m.pingHistory[selected.IP]
			if len(history) >= 2 {
				graphHeight := m.height - 21
				if graphHeight > 2 {
					graph = asciigraph.Plot(history, asciigraph.Height(graphHeight), asciigraph.Width(rightWidth-10), asciigraph.Caption("Live Ping Fluctuation (ms)"))
				}
			} else {
				graph = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Awaiting data for graph...")
			}
		}
	}

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(
		fmt.Sprintf(" Tab: Switch • S: Sort • F: Search • A: Add IP • B: Bookmark • D: Del • P: Pass • R: RCON • V: Ver (%s)", m.selectedVer),
	)

	panels := lipgloss.JoinHorizontal(lipgloss.Top,
		listStyle.Render(strings.Join(rows, "\n")),
		graphStyle.Render(details+graph),
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, panels, footer)
}
