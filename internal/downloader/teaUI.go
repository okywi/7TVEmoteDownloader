package downloader

import (
	"fmt"
	"slices"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	cursor           int
	choices          []string
	selected         map[int]any
	stateSelections  map[string][]string
	textInput        textinput.Model
	states           []string
	currentState     int
	stateHeader      string
	emoteDownloader  *emoteDownloader
	downloadProgress progress.Model
}

func InitialModel() model {
	// username input
	textInput := textinput.New()
	textInput.Width = 200
	textInput.Placeholder = "User id..."
	textInput.Prompt = ":3 | "
	textInput.Focus()

	// download progress
	progress := progress.New()
	progress.Width = 200

	return model{
		emoteDownloader:  &emoteDownloader{},
		choices:          make([]string, 0),
		states:           []string{"username", "emotesets", "imagetypes", "imagesizes", "download"},
		stateSelections:  make(map[string][]string),
		currentState:     0,
		selected:         make(map[int]any),
		stateHeader:      "Please input the user id:",
		textInput:        textInput,
		downloadProgress: progress,
	}
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

	switch m.states[m.currentState] {
	case "username":
		m.textInput, cmd = m.textInput.Update(msg)

		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				userId := m.textInput.Value()

				var err error
				m.emoteDownloader.user, err = fetchUser(userId)
				if err != nil {
					m.textInput.SetValue("User not found :/")
					return m, cmd
				}

				username := m.emoteDownloader.user["username"].(string)
				m.textInput.SetValue(username)

				m.changeState()
				return m, cmd
			}
		}
	case "download":
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
				m.changeState()
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
	return m, cmd
}

func (m *model) changeState() {
	// set choices according to state
	state := m.states[m.currentState]
	if state != "username" {
		stateSelection := []string{}
		for i, choice := range m.choices {
			if _, ok := m.selected[i]; ok {
				stateSelection = append(stateSelection, choice)
			}
		}
		m.stateSelections[state] = stateSelection
	}

	m.currentState = (m.currentState + 1) % len(m.states)

	switch m.states[m.currentState] {
	case "username":
		m.textInput.Focus()
	case "emotesets":
		// clear input
		m.textInput.Reset()
		m.textInput.Blur()

		m.stateHeader = "Select the emote sets you want to download:"
		m.emoteDownloader.emoteSets = fetchEmoteSetsOfUser(m.emoteDownloader.user["emote_sets"].([]any))

		m.choices = []string{}
		for _, set := range m.emoteDownloader.emoteSets {
			m.choices = append(m.choices, fmt.Sprintf("%s [%d Emotes]", set.name, len(set.emotes)))
		}
	case "imagetypes":
		m.stateHeader = "Select images types you want to download:"
		// get selected emote sets
		selectedSetNames := m.stateSelections["emotesets"]
		selectedSets := make([]*emoteSet, 0)
		for _, set := range m.emoteDownloader.emoteSets {
			if slices.Contains(selectedSetNames, set.String()) {
				selectedSets = append(selectedSets, set)
			}
		}
		m.emoteDownloader.emoteSets = selectedSets

		m.choices = []string{"webp", "avif", "png", "gif"}
	case "imagesizes":
		m.stateHeader = "Select the image sizes you want to download:"
		m.choices = []string{"1x", "2x", "3x", "4x"}
	case "download":
		m.stateHeader = "Downloading emotes..."
		go downloadAndWriteEmoteSets(m,
			m.emoteDownloader.emoteSets, m.stateSelections["imagetypes"], m.stateSelections["imagesizes"], m.emoteDownloader.user["username"].(string))
	}

	// reset selected
	m.selected = make(map[int]any)
}

func (m model) View() string {
	// The header
	s := "---- 7TV Emote Downloader ----\n" +
		"Downloads all emotes of a 7TV user/streamer\n\n"

	s += fmt.Sprint(m.stateHeader + "\n")

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

	if m.states[m.currentState] == "download" {
		s += fmt.Sprint("Indicator: " + m.emoteDownloader.currentDownloadIndicator + "\n")
	}

	if m.states[m.currentState] == "username" {
		s += m.textInput.View() + "\n"
	}

	// The footer
	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}
