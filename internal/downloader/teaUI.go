package downloader

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type selectionState int

const (
	Username selectionState = iota
	Emotesets
	ImageTypes
	ImageSizes
	Download
	ErrorLog
)

var selectionStates map[selectionState]string = map[selectionState]string{
	Username:   "Username",
	Emotesets:  "Emotesets",
	ImageTypes: "ImageTypes",
	ImageSizes: "ImageSizes",
	Download:   "Download",
	ErrorLog:   "ErrorLog",
}

func (state selectionState) String() string {
	return selectionStates[state]
}

func (state selectionState) transition() selectionState {
	switch state {
	case Username:
		return Emotesets
	case Emotesets:
		return ImageTypes
	case ImageTypes:
		return ImageSizes
	case ImageSizes:
		return Download
	case Download:
		return Username
	default:
		return 0
	}
}

type model struct {
	downloadChannel  chan struct{}
	cursor           int
	choices          []string
	selected         map[int]any
	currentState     selectionState
	stateSelections  map[selectionState][]string
	userIdInput      textinput.Model
	userLoadingInfo  string
	stateHeader      *string
	emoteDownloader  *emoteDownloader
	downloadProgress progress.Model
	errorLogArea     textarea.Model
}

type downloadMsg struct{}

func InitialModel() *model {
	// username input
	userIdInput := textinput.New()
	userIdInput.Width = 200
	userIdInput.Placeholder = "User id..."
	userIdInput.Prompt = ":3 | "
	userIdInput.Focus()

	// download progress
	progress := progress.New(progress.WithDefaultGradient())
	progress.Width = 50

	// error log area
	errorLogArea := textarea.New()
	errorLogArea.SetWidth(50)
	errorLogArea.SetHeight(30)
	errorLogArea.ShowLineNumbers = false

	stateHeader := "Please input the user id:"

	model := &model{
		downloadChannel: make(chan struct{}),
		emoteDownloader: &emoteDownloader{
			finished:   false,
			errorLog:   make([]string, 0),
			showErrors: false,
		},
		choices:          make([]string, 0),
		stateSelections:  make(map[selectionState][]string),
		selected:         make(map[int]any),
		userIdInput:      userIdInput,
		userLoadingInfo:  "",
		stateHeader:      &stateHeader,
		downloadProgress: progress,
		errorLogArea:     errorLogArea,
	}

	return model
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return m.userIdInput.Cursor.BlinkCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}

	switch m.currentState {
	case Username:
		m.userIdInput.Focus()
		m.userIdInput, cmd = m.userIdInput.Update(msg)

		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				userId := m.userIdInput.Value()

				var err error
				m.emoteDownloader.user, err = fetchUser(userId)
				if err != nil {
					m.userIdInput.Reset()

					m.userLoadingInfo = fmt.Sprintf("\n%s - Please try again.\n", err.Error())
					return m, cmd
				}

				return m, tea.Batch(cmd, m.changeState())
			}
		}
		return m, cmd
	case Download:
		switch msg := msg.(type) {
		case downloadMsg:
			cmd := m.downloadProgress.SetPercent(m.emoteDownloader.percentage)
			return m, tea.Sequence(cmd, downloadListenCmd(m.downloadChannel))
		// FrameMsg is sent when the progress bar wants to animate itself
		case progress.FrameMsg:
			progressModel, cmd := m.downloadProgress.Update(msg)
			m.downloadProgress = progressModel.(progress.Model)
			return m, cmd
		case tea.KeyMsg:
			switch msg.String() {
			case "f":
				if len(m.emoteDownloader.errorLog) == 0 || !m.emoteDownloader.finished {
					return m, nil
				}
				m.emoteDownloader.showErrors = true
				m.currentState = ErrorLog

				m.errorLogArea.SetValue(strings.Join(m.emoteDownloader.errorLog, "\n"))
				cmd = m.errorLogArea.Focus()
				m.errorLogArea.CursorStart()
				m.errorLogArea.SetCursor(0)
				m.errorLogArea.Cursor.SetMode(cursor.CursorStatic)

				return m, cmd
			}
		}
		return m, tea.Batch(cmd, downloadListenCmd(m.downloadChannel))
	case ErrorLog:
		switch msg := msg.(type) {

		case tea.KeyMsg:
			switch msg.String() {
			case "f":
				m.emoteDownloader.showErrors = false
				m.currentState = Download

				return m, cmd
			case "up", "down", "left", "right":
			default:
				return m, cmd
			}

		}

		m.errorLogArea, cmd = m.errorLogArea.Update(msg)

		return m, cmd
	default:
		switch msg := msg.(type) {

		// Is it a key press?
		case tea.KeyMsg:
			// Cool, what was the actual key pressed?
			switch msg.String() {
			case " ":
				_, ok := m.selected[m.cursor]
				if ok {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = m.choices[m.cursor]
				}
				return m, cmd
			case "enter":
				isEmpty := true
				for i := range m.selected {
					if _, exists := m.selected[i]; exists {
						isEmpty = false
					}
				}
				if !isEmpty {
					cmd = m.changeState()
				}
				return m, cmd
			case "backspace":
				m.currentState -= 2
				cmd := m.changeState()
				return m, cmd
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.choices)-1 {
					m.cursor++
				}
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func downloadListenCmd(sub chan struct{}) tea.Cmd {
	return func() tea.Msg {
		return downloadMsg(<-sub)
	}
}

func (m *model) changeState() tea.Cmd {
	var cmd tea.Cmd
	nextState := selectionState.transition(m.currentState)

	m.cursor = 0

	// set choices according to state
	if nextState != Username {
		stateSelection := []string{}
		for i, choice := range m.choices {
			if _, ok := m.selected[i]; ok {
				stateSelection = append(stateSelection, choice)
			}
		}
		m.stateSelections[m.currentState] = stateSelection
	}

	switch nextState {
	case Emotesets:
		// clear input
		m.userIdInput.Reset()
		m.userIdInput.Blur()

		*m.stateHeader = "Select the emote sets you want to download:"
		m.emoteDownloader.emoteSets = fetchEmoteSetsOfUser(m.emoteDownloader.user["emote_sets"].([]any))

		m.choices = []string{}
		for _, set := range m.emoteDownloader.emoteSets {
			m.choices = append(m.choices, set.String())
		}
	case ImageTypes:
		*m.stateHeader = "Select images types you want to download:"
		// get selected emote sets
		selectedSetNames := m.stateSelections[Emotesets]
		selectedSets := make([]*emoteSet, 0)
		for _, set := range m.emoteDownloader.emoteSets {
			if slices.Contains(selectedSetNames, set.String()) {
				selectedSets = append(selectedSets, set)
			}
		}
		m.emoteDownloader.emoteSets = selectedSets

		m.choices = []string{"webp", "avif", "png", "gif"}
	case ImageSizes:
		*m.stateHeader = "Select the image sizes you want to download:"
		m.choices = []string{"1x", "2x", "3x", "4x"}
	case Download:
		go downloadAndWriteEmoteSets(m, m.emoteDownloader.emoteSets, m.stateSelections[ImageTypes], m.stateSelections[ImageSizes], m.emoteDownloader.user["username"].(string))
		cmd = downloadListenCmd(m.downloadChannel)
	}

	// reset selected
	m.selected = make(map[int]any)

	m.currentState = nextState
	return cmd
}

func (m model) View() string {
	// The header
	s := "---- 7TV Emote Downloader ----\n" +
		"Downloads all emotes of a 7TV user/streamer\n\n"

	if !m.emoteDownloader.finished {
		s += fmt.Sprint(*m.stateHeader + "\n")
	}

	switch m.currentState {
	case Username:
		s += m.userIdInput.View() + "\n"
		s += m.userLoadingInfo
	case Emotesets, ImageTypes, ImageSizes:
		// Iterate over our choices
		for i, choice := range m.choices {

			// Is the cursor pointing at this choice?
			cursor := " " // no cursor
			if m.cursor == i {
				cursor = ">" // cursor!
			}

			// Is this choice selected?
			checked := " " // not selected
			if _, ok := m.selected[i]; ok {
				checked = "x" // selected!
			}

			// Render the row
			s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice)
		}

		s += "\nPress space to choose. | Press enter to confirm. | Press backspace to go back.\n"
	case Download:
		if !m.emoteDownloader.finished {
			s += m.downloadProgress.View() + "\n\n"
		}

		s += fmt.Sprint(m.emoteDownloader.currentDownloadIndicator + "\n")
	case ErrorLog:
		s += m.errorLogArea.View()
		s += "\nPress f to toggle errors.\n"
	}

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}
