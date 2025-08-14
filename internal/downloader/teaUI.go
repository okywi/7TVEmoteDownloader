package downloader

import (
	"fmt"
	"slices"
	"time"

	"github.com/charmbracelet/bubbles/progress"
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
)

var selectionStates map[selectionState]string = map[selectionState]string{
	Username:   "Username",
	Emotesets:  "Emotesets",
	ImageTypes: "ImageTypes",
	ImageSizes: "ImageSizes",
	Download:   "Download",
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
	cursor           int
	choices          []string
	selected         map[int]any
	currentState     selectionState
	stateSelections  map[selectionState][]string
	userIdInput      textinput.Model
	stateHeader      *string
	emoteDownloader  *emoteDownloader
	downloadProgress progress.Model
}

type tickMsg time.Time

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

	stateHeader := "Please input the user id:"

	model := &model{
		emoteDownloader:  &emoteDownloader{},
		choices:          make([]string, 0),
		stateSelections:  make(map[selectionState][]string),
		selected:         make(map[int]any),
		userIdInput:      userIdInput,
		stateHeader:      &stateHeader,
		downloadProgress: progress,
	}

	return model
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return textinput.Blink
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
		m.userIdInput, cmd = m.userIdInput.Update(msg)

		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				userId := m.userIdInput.Value()

				var err error
				m.emoteDownloader.user, err = fetchUser(userId)
				if err != nil {
					m.userIdInput.SetValue(err.Error())
					m.userIdInput.Blur()
					return m, cmd
				}

				username := m.emoteDownloader.user["username"].(string)
				m.userIdInput.SetValue(username)

				cmd = m.changeState()

				return m, cmd
			}
		}
		return m, cmd
	case Download:
		switch msg.(type) {
		case tickMsg:
			cmd := m.downloadProgress.SetPercent(m.emoteDownloader.percentage)
			return m, tea.Batch(cmd, tickCmd())
			// FrameMsg is sent when the progress bar wants to animate itself
		case progress.FrameMsg:
			progressModel, cmd := m.downloadProgress.Update(msg)
			m.downloadProgress = progressModel.(progress.Model)
			return m, cmd
		}
		return m, tickCmd()

	default:
		switch msg := msg.(type) {

		// Is it a key press?
		case tea.KeyMsg:
			// Cool, what was the actual key pressed?
			switch msg.String() {
			case "enter", " ":
				_, ok := m.selected[m.cursor]
				if ok {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = m.choices[m.cursor]
				}
				return m, cmd
			case "s":
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

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*16, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *model) changeState() tea.Cmd {
	var cmd tea.Cmd
	nextState := selectionState.transition(m.currentState)

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
		cmd = tickCmd()
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

	s += fmt.Sprint(*m.stateHeader + "\n")

	if m.currentState == Emotesets || m.currentState == ImageTypes || m.currentState == ImageSizes {
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
	}

	if m.currentState == Download {
		s += fmt.Sprint(m.emoteDownloader.currentDownloadIndicator + "\n")
		s += m.downloadProgress.View() + "\n"
	}

	if m.currentState == Username {
		s += m.userIdInput.View() + "\n"
	}

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}
