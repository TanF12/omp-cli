package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"omp-cli/internal/config"
	"omp-cli/internal/core"
	"omp-cli/internal/i18n"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"
)

type tickMsg time.Time
type queryResultMsg struct {
	results []core.ServerInfo
	err     error
}

type clientsResultMsg struct {
	target  string
	clients []core.ServerClient
	err     error
}

type serverRulesMsg struct {
	target string
	rules  map[string]string
}

type cachedPlayers struct {
	clients   []core.ServerClient
	fetchedAt time.Time
}

type SortMode int

const (
	SortPlayersDesc SortMode = iota
	SortPingAsc
	SortName
)

type TabMode int

const (
	TabGlobal TabMode = iota
	TabFavourites
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

var (
	ColorBg        = lipgloss.Color("#0f172a")
	ColorBorder    = lipgloss.Color("#475569")
	ColorBorderAct = lipgloss.Color("#6366f1")
	ColorAccent    = lipgloss.Color("#818cf8")
	ColorMuted     = lipgloss.Color("#64748b")
	ColorText      = lipgloss.Color("#f1f5f9")
	ColorGreen     = lipgloss.Color("#10b981")
	ColorRed       = lipgloss.Color("#f87171")
	ColorAmber     = lipgloss.Color("#fbbf24")
	ColorCyan      = lipgloss.Color("#2dd4bf")
)

type model struct {
	cfg         *config.AppConfig
	apiServers  []core.OpenMpServer
	favsData    *config.FavouritesData
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

	showPlayers     bool
	currentClients  []core.ServerClient
	fetchingClients bool
	playersCache    map[string]cachedPlayers

	launchCb func(string, uint16, string, string, string) error
}

func InitialModel(cfg *config.AppConfig, servers []core.OpenMpServer, launcher func(string, uint16, string, string, string) error) model {
	ti := textinput.New()
	ti.CharLimit = 50
	ti.Width = 40
	favs, _ := config.LoadFavourites()

	m := model{
		cfg:          cfg,
		apiServers:   servers,
		favsData:     favs,
		liveData:     make(map[string]core.ServerInfo),
		pingHistory:  make(map[string][]float64),
		playersCache: make(map[string]cachedPlayers),
		textInput:    ti,
		sortMode:     SortPlayersDesc,
		activeTab:    TabFavourites,
		selectedVer:  cfg.DefaultVersion,
		isQuerying:   false,
		launchCb:     launcher,
	}

	if len(m.apiServers) > 0 {
		m.activeTab = TabGlobal
	}

	m.applyFiltersAndSort()
	return m
}

func matchKey(input string, bindString string) bool {
	for _, bind := range strings.Split(bindString, ",") {
		if strings.TrimSpace(bind) == input {
			return true
		}
	}
	return false
}

func formatFirstKey(bindString string) string {
	parts := strings.Split(bindString, ",")
	if len(parts) > 0 {
		return strings.ToUpper(strings.TrimSpace(parts[0]))
	}
	return ""
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
		m.cursor, m.scroll = 0, 0
	}
	if len(m.viewList) == 0 {
		m.cursor, m.scroll = 0, 0
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
		res, err := core.QueryBatch(targets)
		return queryResultMsg{results: res, err: err}
	}
}

func fetchClientsCmd(target, ip string, port uint16) tea.Cmd {
	return func() tea.Msg {
		clients, err := core.QueryClients(ip, port)
		return clientsResultMsg{target: target, clients: clients, err: err}
	}
}

func fetchRulesCmd(target string, ip string, port uint16) tea.Cmd {
	return func() tea.Msg {
		info, err := core.QueryServer(ip, port)
		if err == nil && info.Rules != nil {
			return serverRulesMsg{target: target, rules: info.Rules}
		}
		return serverRulesMsg{target: target, rules: make(map[string]string)}
	}
}

func (m model) launchCurrent() (tea.Model, tea.Cmd) {
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
			password = config.DecryptPassword(fav.ServerPassword)
		}

		if isLocked && password == "" {
			m.inputMode = InputServerPasswordAndLaunch
			m.textInput.Placeholder = "Locked server. Enter password..."
			m.textInput.EchoMode = textinput.EchoPassword
			m.textInput.EchoCharacter = '*'
			m.textInput.Focus()
			return m, textinput.Blink
		}

		m.statusMsg = "Launching game on " + target + "..."
		if err := m.launchCb(parts[0], uint16(port), m.cfg.DefaultName, m.selectedVer, password); err != nil {
			m.statusMsg = "Launch failed: " + err.Error()
		} else {
			m.statusMsg = i18n.T("success") + " (" + target + ")"
		}
	}
	return m, nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.visibleRows = m.height - 9
		if m.visibleRows < 5 {
			m.visibleRows = 5
		}

	case tea.KeyMsg:
		if m.inputMode != InputNone {
			switch {
			case matchKey(msg.String(), "esc"):
				m.inputMode = InputNone
				m.textInput.EchoMode = textinput.EchoNormal
				m.textInput.Blur()
				m.textInput.Reset()
				m.applyFiltersAndSort()

			case m.inputMode == InputSearch && matchKey(msg.String(), m.cfg.Keybinds.Up):
				m.showPlayers = false
				if m.cursor > 0 {
					m.cursor--
				}
				if m.cursor < m.scroll {
					m.scroll = m.cursor
				}

			case m.inputMode == InputSearch && matchKey(msg.String(), m.cfg.Keybinds.Down):
				m.showPlayers = false
				if m.cursor < len(m.viewList)-1 {
					m.cursor++
				}
				if m.cursor >= m.scroll+m.visibleRows {
					m.scroll = m.cursor - m.visibleRows + 1
				}

			case matchKey(msg.String(), m.cfg.Keybinds.Enter):
				val := strings.TrimSpace(m.textInput.Value())
				target := ""
				if len(m.viewList) > 0 {
					target = m.viewList[m.cursor].IP
				}

				if m.inputMode == InputSearch {
					m.inputMode = InputNone
					m.textInput.Blur()
					return m.launchCurrent()
				}

				if val != "" || m.inputMode == InputAddIP {
					switch m.inputMode {
					case InputAddIP:
						if val != "" {
							if !strings.Contains(val, ":") {
								val += ":7777"
							}
							m.favsData.Servers[val] = config.Favouriteserver{}
							config.SaveFavourites(m.favsData)
							m.statusMsg, m.activeTab = "Added "+val+" to Favourites.", TabFavourites
						}
					case InputServerPassword, InputServerPasswordAndLaunch:
						enc := config.EncryptPassword(val, m.cfg.Security.EncryptPasswords)

						if m.inputMode == InputServerPassword {
							fav := m.favsData.Servers[target]
							fav.ServerPassword = enc
							m.favsData.Servers[target] = fav
							config.SaveFavourites(m.favsData)
							m.statusMsg = "Server password securely saved."
						}

						if m.inputMode == InputServerPasswordAndLaunch {
							if m.cfg.Security.SavePasswords {
								fav := m.favsData.Servers[target]
								fav.ServerPassword = enc
								m.favsData.Servers[target] = fav
								config.SaveFavourites(m.favsData)
							}

							parts := strings.Split(target, ":")
							port, _ := strconv.Atoi(parts[1])
							m.statusMsg = "Launching game on " + target + "..."
							if err := m.launchCb(parts[0], uint16(port), m.cfg.DefaultName, m.selectedVer, val); err != nil {
								m.statusMsg = "Launch failed: " + err.Error()
							} else {
								m.statusMsg = i18n.T("success") + " (" + target + ")"
							}
						}
					case InputRcon:
						enc := config.EncryptPassword(val, m.cfg.Security.EncryptPasswords)
						fav := m.favsData.Servers[target]
						fav.RconPassword = enc
						m.favsData.Servers[target] = fav
						config.SaveFavourites(m.favsData)
						m.statusMsg = "RCON Password saved."
					case InputNickname:
						m.cfg.DefaultName = val
						config.Save(m.cfg)
						m.statusMsg = "Nickname changed to: " + val
					}
				}
				m.inputMode = InputNone
				m.textInput.EchoMode = textinput.EchoNormal
				m.textInput.Blur()
				m.textInput.Reset()
				m.applyFiltersAndSort()

			default:
				m.textInput, cmd = m.textInput.Update(msg)
				if m.inputMode == InputSearch {
					m.cursor, m.scroll = 0, 0
					m.applyFiltersAndSort()
				}
				return m, cmd
			}
			return m, nil
		}

		m.statusMsg = ""
		switch {
		case matchKey(msg.String(), m.cfg.Keybinds.Quit):
			return m, tea.Quit
		case matchKey(msg.String(), m.cfg.Keybinds.Up):
			m.showPlayers = false
			if m.cursor > 0 {
				m.cursor--
			}
			if m.cursor < m.scroll {
				m.scroll = m.cursor
			}
		case matchKey(msg.String(), m.cfg.Keybinds.Down):
			m.showPlayers = false
			if m.cursor < len(m.viewList)-1 {
				m.cursor++
			}
			if m.cursor >= m.scroll+m.visibleRows {
				m.scroll = m.cursor - m.visibleRows + 1
			}
		case matchKey(msg.String(), m.cfg.Keybinds.SwitchTab):
			m.showPlayers = false
			if m.activeTab == TabGlobal {
				m.activeTab = TabFavourites
			} else if len(m.apiServers) > 0 {
				m.activeTab = TabGlobal
			}
			m.textInput.Reset()
			m.cursor, m.scroll = 0, 0
			m.applyFiltersAndSort()
		case matchKey(msg.String(), m.cfg.Keybinds.TogglePlayers):
			if len(m.viewList) > 0 {
				m.showPlayers = !m.showPlayers
				if m.showPlayers {
					target := m.viewList[m.cursor].IP
					if cache, ok := m.playersCache[target]; ok && time.Since(cache.fetchedAt) < 15*time.Second {
						m.currentClients = cache.clients
						m.fetchingClients = false
						m.statusMsg = "Loaded players from cache."
						return m, nil
					}
					m.fetchingClients = true
					parts := strings.Split(target, ":")
					port, _ := strconv.Atoi(parts[1])
					m.statusMsg = "Fetching player list..."
					return m, tea.Batch(textinput.Blink, fetchClientsCmd(target, parts[0], uint16(port)))
				}
			}
		case matchKey(msg.String(), m.cfg.Keybinds.Search):
			m.inputMode = InputSearch
			m.textInput.Placeholder = "Search..."
			m.textInput.Focus()
			return m, textinput.Blink
		case matchKey(msg.String(), m.cfg.Keybinds.AddIP):
			m.inputMode = InputAddIP
			m.textInput.Placeholder = "Enter IP:PORT manually..."
			m.textInput.Focus()
			return m, textinput.Blink
		case matchKey(msg.String(), m.cfg.Keybinds.Delete):
			if len(m.viewList) > 0 {
				target := m.viewList[m.cursor].IP
				if _, exists := m.favsData.Servers[target]; exists {
					delete(m.favsData.Servers, target)
					config.SaveFavourites(m.favsData)
					m.statusMsg = "Removed from Favourites."
					m.applyFiltersAndSort()
				} else {
					m.statusMsg = "Server is not in favourites."
				}
			}
		case matchKey(msg.String(), m.cfg.Keybinds.SwitchVersion):
			versions := m.cfg.GetAvailableVersions()
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
		case matchKey(msg.String(), m.cfg.Keybinds.SetPassword):
			if !m.cfg.Security.SavePasswords {
				m.statusMsg = "Saving passwords is disabled in config."
				return m, nil
			}
			if m.activeTab == TabFavourites && len(m.viewList) > 0 {
				m.inputMode = InputServerPassword
				m.textInput.Placeholder = "Enter server password..."
				m.textInput.EchoMode = textinput.EchoPassword
				m.textInput.EchoCharacter = '*'
				m.textInput.Focus()
				return m, textinput.Blink
			} else {
				m.statusMsg = "Passwords can only be saved on Favourited servers."
			}
		case matchKey(msg.String(), m.cfg.Keybinds.SetRcon):
			if !m.cfg.Security.SavePasswords {
				m.statusMsg = "Saving passwords is disabled in config."
				return m, nil
			}
			if m.activeTab == TabFavourites && len(m.viewList) > 0 {
				m.inputMode = InputRcon
				m.textInput.Placeholder = "Enter RCON password..."
				m.textInput.Focus()
				return m, textinput.Blink
			} else {
				m.statusMsg = "RCON can only be set on Favourited servers."
			}
		case matchKey(msg.String(), m.cfg.Keybinds.ChangeName):
			m.inputMode = InputNickname
			m.textInput.Placeholder = "Enter new nickname..."
			m.textInput.SetValue(m.cfg.DefaultName)
			m.textInput.Focus()
			return m, textinput.Blink
		case matchKey(msg.String(), m.cfg.Keybinds.ToggleBookmark):
			if len(m.viewList) > 0 {
				target := m.viewList[m.cursor].IP
				if _, exists := m.favsData.Servers[target]; exists {
					delete(m.favsData.Servers, target)
					m.statusMsg = "Removed from Favourites."
				} else {
					m.favsData.Servers[target] = config.Favouriteserver{}
					m.statusMsg = "Added to Favourites."
				}
				config.SaveFavourites(m.favsData)
				m.applyFiltersAndSort()
			}
		case matchKey(msg.String(), m.cfg.Keybinds.Import):
			count := config.ImportSAMPFavourites(m.favsData)
			config.SaveFavourites(m.favsData)
			m.statusMsg = fmt.Sprintf("Imported %d servers from USERDATA.DAT", count)
			m.applyFiltersAndSort()
		case matchKey(msg.String(), m.cfg.Keybinds.ChangeSort):
			m.sortMode = (m.sortMode + 1) % 3
			m.applyFiltersAndSort()
		case matchKey(msg.String(), m.cfg.Keybinds.Enter):
			return m.launchCurrent()
		}

	case serverRulesMsg:
		if live, ok := m.liveData[msg.target]; ok {
			live.Rules = msg.rules
			m.liveData[msg.target] = live
		} else {
			m.liveData[msg.target] = core.ServerInfo{Rules: msg.rules}
		}

	case queryResultMsg:
		m.isQuerying = false // Unlock state to allow next batch query
		if msg.err != nil {
			m.statusMsg = "Batch query failure: " + msg.err.Error()
			break
		}
		for _, res := range msg.results {
			if res.Target == "" {
				continue
			}

			if existing, ok := m.liveData[res.Target]; ok && existing.Rules != nil {
				res.Rules = existing.Rules
			}

			m.liveData[res.Target] = res
			if res.Error == "" {
				m.pingHistory[res.Target] = append(m.pingHistory[res.Target], float64(res.PingMs))
				if len(m.pingHistory[res.Target]) > 20 {
					m.pingHistory[res.Target] = m.pingHistory[res.Target][1:]
				}
			}
		}
		if m.activeTab == TabFavourites {
			m.applyFiltersAndSort()
		}

	case clientsResultMsg:
		m.fetchingClients = false
		if msg.err != nil {
			m.statusMsg = "Failed to fetch players: " + msg.err.Error()
			m.showPlayers = false
		} else {
			m.currentClients = msg.clients
			m.playersCache[msg.target] = cachedPlayers{clients: msg.clients, fetchedAt: time.Now()}
			m.statusMsg = fmt.Sprintf("Fetched %d players successfully.", len(msg.clients))
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

		if len(m.viewList) > 0 {
			target := m.viewList[m.cursor].IP
			live, hasLive := m.liveData[target]

			if (!hasLive || live.Rules == nil) && m.viewList[m.cursor].Ru == nil {
				if hasLive {
					live.Rules = make(map[string]string)
					m.liveData[target] = live
				} else {
					m.liveData[target] = core.ServerInfo{Rules: make(map[string]string)}
				}

				parts := strings.Split(target, ":")
				if len(parts) == 2 {
					port, _ := strconv.Atoi(parts[1])
					cmds = append(cmds, fetchRulesCmd(target, parts[0], uint16(port)))
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.width == 0 {
		return i18n.T("init")
	}

	var leftInner, rightInner int
	var isSplit bool

	if m.width < 95 {
		isSplit = false
		leftInner = m.width - 6
		if leftInner < 15 {
			leftInner = 15
		}
	} else {
		isSplit = true
		leftOuter := int(float64(m.width) * 0.35)
		rightOuter := m.width - leftOuter - 6
		leftInner, rightInner = leftOuter-4, rightOuter-4
		if leftInner < 25 {
			leftInner = 25
		}
		if rightInner < 30 {
			rightInner = 30
		}
	}

	listStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).Width(leftInner).Height(m.height-7).Padding(0, 1)
	graphStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorderAct).Width(rightInner).Height(m.height-7).Padding(0, 1)

	tabStyle := lipgloss.NewStyle().Padding(0, 2)
	activeTabStyle := tabStyle.Background(ColorBorderAct).Foreground(ColorText).Bold(true)
	inactiveTabStyle := tabStyle.Foreground(ColorMuted)

	favTab, globTab := inactiveTabStyle.Render(i18n.T("favourites")), inactiveTabStyle.Render(i18n.T("global_servers"))
	if m.activeTab == TabGlobal {
		globTab = activeTabStyle.Render(i18n.T("global_servers"))
	} else {
		favTab = activeTabStyle.Render(i18n.T("favourites"))
	}

	var tabs string
	if len(m.apiServers) == 0 {
		tabs = favTab
	} else {
		tabs = lipgloss.JoinHorizontal(lipgloss.Top, favTab, "  ", globTab)
	}

	sortStr := i18n.T("players")
	if m.sortMode == SortPingAsc {
		sortStr = i18n.T("ping")
	} else if m.sortMode == SortName {
		sortStr = i18n.T("name")
	}

	infoBar := lipgloss.NewStyle().Foreground(ColorMuted).Render(
		fmt.Sprintf("Nick: %s  |  Sort: %s", lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(m.cfg.DefaultName), sortStr),
	)

	totalHeaderWidth := lipgloss.Width(tabs) + lipgloss.Width(infoBar)
	spaces := m.width - totalHeaderWidth - 4
	if spaces < 2 {
		spaces = 2
	}

	var topBar string
	if totalHeaderWidth+4 > m.width {
		topBar = tabs
	} else {
		topBar = lipgloss.JoinHorizontal(lipgloss.Top, tabs, strings.Repeat(" ", spaces), infoBar)
	}

	headerContent := " "
	if m.inputMode != InputNone {
		headerContent = lipgloss.NewStyle().Foreground(ColorCyan).Render(" > ") + m.textInput.View()
	} else if m.statusMsg != "" {
		headerContent = lipgloss.NewStyle().Foreground(ColorGreen).Render(" ! " + m.statusMsg)
	}
	header := lipgloss.JoinVertical(lipgloss.Left, topBar, headerContent)

	var rows []string
	if len(m.viewList) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(ColorMuted).Render(i18n.T("no_servers")))
	}

	end := m.scroll + m.visibleRows
	if end > len(m.viewList) {
		end = len(m.viewList)
	}

	innerListWidth := leftInner - listStyle.GetHorizontalFrameSize()
	rowWidth := innerListWidth - 2 - 2

	for i := m.scroll; i < end; i++ {
		s := m.viewList[i]
		cursor := "  "
		playerStr, pingMsVal, isLocked := fmt.Sprintf("(%d/%d)", s.Pc, s.Pm), "", s.Pa
		hostname := s.Hn

		if live, ok := m.liveData[s.IP]; ok {
			if live.Error != "" {
				playerStr = "[" + i18n.T("offline") + "]"
			} else {
				playerStr = fmt.Sprintf("(%d/%d)", live.Players, live.MaxPlayers)
				pingMsVal = fmt.Sprintf("%dms", live.PingMs)
				isLocked = live.Password
			}
		}

		favIcon := " "
		if _, exists := m.favsData.Servers[s.IP]; exists {
			favIcon = lipgloss.NewStyle().Foreground(ColorAmber).Render("★")
		}
		lockIcon := ""
		if isLocked {
			lockIcon = lipgloss.NewStyle().Foreground(ColorRed).Render("🔒")
		}

		playerStyle := lipgloss.NewStyle().Foreground(ColorMuted)
		if s.Pc > 0 {
			playerStyle = lipgloss.NewStyle().Foreground(ColorCyan)
		}
		pingStyle := lipgloss.NewStyle().Foreground(ColorGreen)

		var rightText string
		if playerStr == "["+i18n.T("offline")+"]" {
			rightText = lipgloss.NewStyle().Foreground(ColorRed).Render("[" + i18n.T("offline") + "]")
		} else {
			rightText = fmt.Sprintf("%s %s", playerStyle.Render(playerStr), pingStyle.Render(pingMsVal))
		}
		rightTextWidth := lipgloss.Width(rightText)

		prefixText := fmt.Sprintf("%s %s", favIcon, lockIcon)
		prefixWidth := lipgloss.Width(prefixText)

		maxHnLen := rowWidth - rightTextWidth - prefixWidth - 1
		if maxHnLen < 5 {
			maxHnLen = 5
		}

		hnRunes := []rune(hostname)
		if len(hnRunes) > maxHnLen {
			hostname = string(hnRunes[:maxHnLen-3]) + "..."
		}

		leftText := fmt.Sprintf("%s%s", prefixText, hostname)
		leftTextWidth := lipgloss.Width(leftText)

		spacerWidth := rowWidth - leftTextWidth - rightTextWidth
		if spacerWidth < 1 {
			spacerWidth = 1
		}
		spacer := strings.Repeat(" ", spacerWidth)

		line := leftText + spacer + rightText

		if m.cursor == i {
			cursor = lipgloss.NewStyle().Foreground(ColorAccent).Render("> ")
			line = lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(line)
		}
		rows = append(rows, cursor+line)
	}

	var details, graph string

	if isSplit && len(m.viewList) > 0 {
		selected := m.viewList[m.cursor]
		live, hasLive := m.liveData[selected.IP]
		gm, lang := selected.Gm, selected.La
		if hasLive && live.Error == "" {
			gm, lang = live.Gamemode, live.Language
		}

		truncate := func(s string, max int) string {
			runes := []rune(s)
			if len(runes) > max {
				return string(runes[:max-1]) + "…"
			}
			return s
		}

		titleInnerWidth := rightInner - graphStyle.GetHorizontalFrameSize() - 2
		title := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Background(ColorBorder).Padding(0, 1).Render(truncate(selected.Hn, titleInnerWidth))

		labelStyle, valueStyle := lipgloss.NewStyle().Foreground(ColorMuted).Bold(true), lipgloss.NewStyle().Foreground(ColorText)

		var statePill string
		if hasLive && live.Error != "" {
			statePill = lipgloss.NewStyle().Background(ColorRed).Foreground(ColorBg).Padding(0, 1).Bold(true).Render(i18n.T("offline"))
		} else {
			pingDisplay := "..."
			if hasLive {
				pingDisplay = fmt.Sprintf("%dms", live.PingMs)
			}
			statePill = lipgloss.NewStyle().Background(ColorGreen).Foreground(ColorBg).Padding(0, 1).Bold(true).Render(i18n.T("online") + " | " + pingDisplay)
		}

		valWidth := (rightInner - 8) / 2
		if valWidth < 10 {
			valWidth = 10
		}

		gmSafe := truncate(gm, valWidth)
		langSafe := truncate(lang, valWidth)
		ipSafe := truncate(selected.IP, valWidth)

		col1 := lipgloss.JoinVertical(lipgloss.Left, labelStyle.Render("IP ADDRESS"), valueStyle.Render(ipSafe), "", labelStyle.Render(i18n.T("status")), statePill)
		col2 := lipgloss.JoinVertical(lipgloss.Left, labelStyle.Render(i18n.T("gamemode")), valueStyle.Render(gmSafe), "", labelStyle.Render(i18n.T("language")), valueStyle.Render(langSafe))

		col1Width := lipgloss.Width(col1)
		col2Width := lipgloss.Width(col2)
		gridSpacing := rightInner - col1Width - col2Width - 4
		if gridSpacing < 2 {
			gridSpacing = 2
		}

		var metadataGrid string
		if rightInner < 38 {
			metadataGrid = lipgloss.JoinVertical(lipgloss.Left, col1, "", col2)
		} else {
			metadataGrid = lipgloss.JoinHorizontal(lipgloss.Top, col1, strings.Repeat(" ", gridSpacing), col2)
		}

		favStatus := ""
		if fav, exists := m.favsData.Servers[selected.IP]; exists {
			favIcon := lipgloss.NewStyle().Foreground(ColorAmber).Render("★ " + i18n.T("bookmarked"))
			passIcon, rconIcon := "", ""
			if fav.ServerPassword != "" {
				passIcon = lipgloss.NewStyle().Foreground(ColorGreen).Render("🔒 " + i18n.T("pass"))
			}
			if fav.RconPassword != "" {
				rconIcon = lipgloss.NewStyle().Foreground(ColorCyan).Render("💻 " + i18n.T("rcon"))
			}
			favStatus = lipgloss.JoinHorizontal(lipgloss.Center, favIcon, "  ", passIcon, "  ", rconIcon)
		}

		var rulesDisplay string
		rulesMap := selected.Ru
		if hasLive && live.Rules != nil && len(live.Rules) > 0 {
			rulesMap = live.Rules
		}

		if len(rulesMap) > 0 {
			var ruleLines []string
			ruleLines = append(ruleLines, labelStyle.Render(i18n.T("rules")))

			var keys []string
			for k := range rulesMap {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			displayCount := len(keys)
			if displayCount > 5 {
				displayCount = 5
			}

			for i := 0; i < displayCount; i++ {
				k := keys[i]
				v := rulesMap[k]
				line := lipgloss.NewStyle().Foreground(ColorCyan).Render(truncate(k, 12)) + ": " + valueStyle.Render(truncate(v, rightInner-18))
				ruleLines = append(ruleLines, line)
			}
			if len(keys) > displayCount {
				ruleLines = append(ruleLines, lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("... +%d more rules", len(keys)-displayCount)))
			}

			rulesDisplay = "\n\n" + strings.Join(ruleLines, "\n")
		}

		divider := lipgloss.NewStyle().Foreground(ColorBorder).Render(strings.Repeat("─", rightInner-graphStyle.GetHorizontalFrameSize()-2))
		details = lipgloss.JoinVertical(lipgloss.Left, title, "", metadataGrid, rulesDisplay, "", favStatus, "", divider)

		detailsHeight := lipgloss.Height(details)
		availHeight := (m.height - 9) - detailsHeight
		if availHeight < 4 {
			availHeight = 4
		}

		if hasLive && live.Error != "" {
			graph = "\n\n" + lipgloss.NewStyle().Foreground(ColorRed).Bold(true).Render(i18n.T("conn_failure")+": "+live.Error)
		} else {
			history := m.pingHistory[selected.IP]

			if m.showPlayers {
				if m.fetchingClients {
					graph = lipgloss.NewStyle().Foreground(ColorMuted).Render("\nQuerying player roster over UDP socket...")
				} else {
					colHeaderStyle, rowStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Underline(true), lipgloss.NewStyle().Foreground(ColorText)
					var tableRows []string

					headerText := fmt.Sprintf("%-3s │ %-24s │ %-6s │ %-4s", "ID", i18n.T("name"), "Score", i18n.T("ping"))
					tableRows = append(tableRows, colHeaderStyle.Render(headerText))
					tableRows = append(tableRows, lipgloss.NewStyle().Foreground(ColorBorder).Render(strings.Repeat("─", rightInner-graphStyle.GetHorizontalFrameSize()-2)))

					maxHeight := availHeight - 3
					if maxHeight < 2 {
						maxHeight = 2
					}

					displayCount := len(m.currentClients)
					if displayCount > maxHeight {
						displayCount = maxHeight
					}

					for i := 0; i < displayCount; i++ {
						c := m.currentClients[i]
						nameRunes := []rune(c.Name)
						nameStr := string(nameRunes)
						if len(nameRunes) > 24 {
							nameStr = string(nameRunes[:21]) + "..."
						}
						pingFmt := "-"
						if c.Ping != nil {
							pingFmt = fmt.Sprintf("%d", *c.Ping)
						}
						tableRows = append(tableRows, rowStyle.Render(fmt.Sprintf("%-3d │ %-24s │ %-6d │ %-4s", c.ID, nameStr, c.Score, pingFmt)))
					}

					if len(m.currentClients) > displayCount {
						tableRows = append(tableRows, lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("... and %d more", len(m.currentClients)-displayCount)))
					} else if len(m.currentClients) == 0 {
						tableRows = append(tableRows, lipgloss.NewStyle().Foreground(ColorMuted).Render("No players online."))
					}
					graph = "\n" + strings.Join(tableRows, "\n")
				}
			} else {
				if len(history) >= 2 {
					graphHeight := availHeight - 2
					if graphHeight > 9 {
						graphHeight = 9
					}
					if graphHeight < 4 {
						graphHeight = 4
					}

					plotWidth := rightInner - graphStyle.GetHorizontalFrameSize() - 8
					if plotWidth < 10 {
						plotWidth = 10
					}

					graph = "\n" + asciigraph.Plot(history, asciigraph.Height(graphHeight), asciigraph.Width(plotWidth), asciigraph.Caption("Live Ping Fluctuation (ms)"))
				} else {
					graph = lipgloss.NewStyle().Foreground(ColorMuted).Render("\n\nAwaiting incoming packet diagnostics to generate graph...")
				}
			}
		}
	} else {
		details = lipgloss.NewStyle().Foreground(ColorMuted).Render(i18n.T("select_server"))
	}

	footerLine := fmt.Sprintf(" Tab: %s • Plyrs: %s • Sort: %s • Find: %s • Add: %s • Fav: %s • Del: %s • Pass: %s • RCON: %s • Ver: %s (%s)",
		formatFirstKey(m.cfg.Keybinds.SwitchTab), formatFirstKey(m.cfg.Keybinds.TogglePlayers),
		formatFirstKey(m.cfg.Keybinds.ChangeSort), formatFirstKey(m.cfg.Keybinds.Search),
		formatFirstKey(m.cfg.Keybinds.AddIP), formatFirstKey(m.cfg.Keybinds.ToggleBookmark),
		formatFirstKey(m.cfg.Keybinds.Delete), formatFirstKey(m.cfg.Keybinds.SetPassword),
		formatFirstKey(m.cfg.Keybinds.SetRcon), formatFirstKey(m.cfg.Keybinds.SwitchVersion),
		lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render(m.selectedVer),
	)

	footer := lipgloss.NewStyle().Foreground(ColorMuted).Render(footerLine)

	var panels string
	if !isSplit {
		listStyle = listStyle.Width(leftInner)
		panels = listStyle.Render(strings.Join(rows, "\n"))
	} else {
		listStyle = listStyle.Width(leftInner)
		graphStyle = graphStyle.Width(rightInner)
		panels = lipgloss.JoinHorizontal(lipgloss.Top, listStyle.Render(strings.Join(rows, "\n")), graphStyle.Render(details+graph))
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, panels, footer)
}
