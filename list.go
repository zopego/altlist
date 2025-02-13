package altlist

import (
	"fmt"
	"io"
	"strings"

	A "github.com/IBM/fp-go/array"
	F "github.com/IBM/fp-go/function"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	tcellviews "github.com/gdamore/tcell/v2/views"
	teapb "github.com/zopego/panelbubble"
)

const (
	bullet       = "•"
	square       = "▪"
	empty_square = " "
	ellipsis     = "…"
)

type SearchList struct {
	List             list.Model
	SearchInput      textinput.Model
	MarkKeyMsgUnused func(msg *teapb.KeyMsg)
	redraw           bool
	view             *tcellviews.ViewPort
}

// type assertions
var _ teapb.ILeafModel = &SearchList{}
var _ teapb.ILeafModelWithView = &SearchList{}

func (s *SearchList) Init(MarkKeyMsgUnused func(msg *teapb.KeyMsg)) tea.Cmd {
	s.MarkKeyMsgUnused = MarkKeyMsgUnused
	return func() tea.Msg {
		return textinput.Blink()
	}
}

// Example implementation of HandlesRecvFocus & HandlesRecvFocusRevoke

func (s SearchList) HandleRecvFocus() {
	s.List.SetShowHelp(true)
	s.redraw = true
}

func (s SearchList) HandleRecvFocusRevoke() {
	s.List.SetShowHelp(false)
	s.redraw = true
}

/*
func (s *SearchList) HandleRecvFocus() (tea.Model, tea.Cmd) {
	return s, func() tea.Msg {
		h := help.New()
		h.ShowAll = true
		h.ShortSeparator = " • "
		items := s.List.FullHelp()
		shortlist := []key.Binding{}
		for _, item := range items {
			for _, i := range item {
				shortlist = append(shortlist, i)
			}
		}

		return teapb.ContextualHelpTextMsg{Text: h.ShortHelpView(shortlist)}
	}
}
*/

/*
func (s *SearchList) Draw(force bool) bool {
	if s.view == nil {
		teapb.DebugPrintf("SearchList has no view")
		return false
	}
	if s.redraw || force {
		panelbubble.TcellDrawHelper(s.View(), s.view)
		s.redraw = false
		return true
	}
	return false
}

func (s *SearchList) SetView(view *tcellviews.ViewPort) {
	s.view = view
} */

func (s *SearchList) NeedsRedraw() bool {
	return true
}

func (s *SearchList) Update(msg teapb.Msg) tea.Cmd {
	if _, ok := msg.(teapb.FocusGrantMsg); ok {
		s.HandleRecvFocus()
		return nil
	}
	if msg, ok := msg.(teapb.ResizeMsg); ok {
		s.HandleSizeMsg(msg)
		return nil
	}

	if _, ok := msg.(teapb.FocusRevokeMsg); ok {
		s.HandleRecvFocusRevoke()
		return nil
	}

	var keyMsg tea.KeyMsg
	if msg, ok := msg.(teapb.KeyMsg); ok {
		keyMsg, ok = teapb.MapKeyMsg(msg)
		if !ok {
			teapb.DebugPrintf("SearchList received KeyMsg that was not mapped: %T %+v\n", msg, msg)
			return nil
		}
		return s.UpdateTeaMsg(keyMsg, func() {
			s.MarkKeyMsgUnused(&msg)
		})
	}

	return s.UpdateTeaMsg(msg, func() {})
}

func (s *SearchList) UpdateTeaMsg(msg tea.Msg, markMsgUnused func()) tea.Cmd {
	cmds := []tea.Cmd{}

	wasFiltering := s.List.FilterState() == list.Filtering
	wasFullHelp := s.List.Help.ShowAll

	prevState := s.List.FilterState()
	initialIndex := s.List.Index()
	updatedList, cmd := s.List.Update(msg)
	cmds = append(cmds, cmd)
	isFullHelp := updatedList.Help.ShowAll
	s.List = updatedList

	afterState := s.List.FilterState()

	updatedIndex := s.List.Index()
	indexChanged := initialIndex != updatedIndex

	nowFiltering := s.List.FilterState() == list.Filtering
	filterStateTransitioned := wasFiltering != nowFiltering
	stateChanged := wasFullHelp != isFullHelp ||
		prevState != afterState ||
		indexChanged

	// Always set the search input to the current filter value
	// & bring the cursor to the end
	s.SearchInput.SetValue(s.List.FilterValue())
	s.SearchInput.CursorEnd()

	// Handle focus transitions
	if filterStateTransitioned {
		if nowFiltering {
			cmds = append(cmds, s.SearchInput.Focus())
		} else {
			s.SearchInput.Blur()
		}
	}

	// This is needed to pass blinking to the search input
	if nowFiltering {
		if _, ok := msg.(teapb.KeyMsg); ok {
			s.redraw = true
			si, cmd := s.SearchInput.Update(msg)
			s.SearchInput = si
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if msg, ok := msg.(cursor.BlinkMsg); ok {
			s.redraw = true
			si, cmd := s.SearchInput.Update(msg)
			s.SearchInput = si
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	if _, ok := msg.(tea.KeyMsg); ok {
		if !nowFiltering && !stateChanged {
			markMsgUnused()
		}
	}

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}

	return nil
}

func (s SearchList) View() string {
	return lipgloss.JoinVertical(lipgloss.Top, s.SearchInput.View(), s.List.View())
}

func (d DefaultItemDelegateAlt) Render(w io.Writer, m list.Model, index int, item list.Item) {
	var (
		title, desc  string
		matchedRunes []int
		s            = &d.Styles
	)

	if i, ok := item.(list.DefaultItem); ok {
		title = i.Title()
		desc = i.Description()
	} else {
		return
	}

	prefix := empty_square
	if _, ok := d.itemSelected[index]; ok {
		if d.itemSelected[index] {
			prefix = bullet
		}
	}
	title = fmt.Sprintf("%s%s", prefix, title)

	if m.Width() <= 0 {
		// short-circuit
		return
	}

	// Prevent text from exceeding list width
	textwidth := m.Width() - s.NormalTitle.GetHorizontalPadding() - 3
	title = ansi.Truncate(title, textwidth, ellipsis)
	if d.ShowDescription {
		var lines []string
		for i, line := range strings.Split(desc, "\n") {
			if i >= d.Height()-1 {
				break
			}
			lines = append(lines, ansi.Truncate(line, textwidth, ellipsis))
		}
		desc = strings.Join(lines, "\n")
	}

	// Conditions
	var (
		isSelected  = index == m.Index()
		emptyFilter = m.FilterState() == list.Filtering && m.FilterValue() == ""
		isFiltered  = m.FilterState() == list.Filtering || m.FilterState() == list.FilterApplied
	)

	if isFiltered && index < len(m.Items()) {
		// Get indices of matched characters
		matchedRunes = F.Pipe1(
			m.MatchesForItem(index),
			A.Map(func(r int) int { return r + 1 }),
		)
	}

	if emptyFilter {
		title = s.DimmedTitle.Render(title)
		desc = s.DimmedDesc.Render(desc)
	} else if isSelected && m.FilterState() != list.Filtering {
		if isFiltered {
			// Highlight matches
			unmatched := s.SelectedTitle.Inline(true)
			matched := unmatched.Inherit(s.FilterMatch)
			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
		}
		title = s.SelectedTitle.Render(title)
		desc = s.SelectedDesc.Render(desc)
	} else {
		if isFiltered {
			// Highlight matches
			unmatched := s.NormalTitle.Inline(true)
			matched := unmatched.Inherit(s.FilterMatch)
			title = lipgloss.StyleRunes(title, matchedRunes, matched, unmatched)
		}
		title = s.NormalTitle.Render(title)
		desc = s.NormalDesc.Render(desc)
	}

	if d.ShowDescription {
		fmt.Fprintf(w, "%s\n%s", title, desc) //nolint: errcheck
		return
	}
	fmt.Fprintf(w, "%s", title) //nolint: errcheck

}

func convertToDefaultItems[T list.DefaultItem](items []T) []list.Item {
	ditems := make([]list.Item, len(items))
	for i, item := range items {
		ditems[i] = item
	}
	return ditems
}

func setPadding(s *lipgloss.Style, p int, p2 int, p3 int, p4 int) {
	*s = s.Padding(p, p2, p3, p4)
}

func SelectableItemsDelegate(selectionToggleKey key.Binding, selectionChanged func(item interface{}, selected bool) tea.Cmd) DefaultItemDelegateAlt {
	d := DefaultItemDelegateAlt{DefaultDelegate: list.NewDefaultDelegate(), itemSelected: make(map[int]bool)}
	d.Styles = list.NewDefaultItemStyles()
	setPadding(&d.Styles.NormalTitle, 0, 0, 0, 1)
	setPadding(&d.Styles.SelectedTitle, 0, 0, 0, 0)
	setPadding(&d.Styles.DimmedTitle, 0, 0, 0, 1)

	d.DefaultDelegate.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		if msg, ok := msg.(tea.KeyMsg); ok && key.Matches(msg, selectionToggleKey) {
			idx := m.Index()
			if _, ok := d.itemSelected[idx]; !ok {
				d.itemSelected[idx] = true
			} else {
				d.itemSelected[idx] = !d.itemSelected[idx]
			}
			return selectionChanged(m.Items()[idx], d.itemSelected[idx])
		}
		return nil
	}
	return d
}

type SearchListConfig struct {
	Width         int
	Height        int
	MultiSelect   bool
	SortByMatches bool
}

func KeyUsedByList(k list.KeyMap, msg tea.Msg) bool {
	if msg, ok := msg.(tea.KeyMsg); ok {
		p := []*key.Binding{
			&k.AcceptWhileFiltering,
			&k.CancelWhileFiltering,
			&k.Filter,
			&k.ClearFilter,
			&k.CursorUp,
			&k.CursorDown,
			&k.GoToStart,
			&k.GoToEnd,
			&k.NextPage,
			&k.PrevPage,
			&k.ShowFullHelp,
			&k.CloseFullHelp,
			&k.Quit,
			&k.ForceQuit,
		}
		for _, b := range p {
			if key.Matches(msg, *b) {
				return true
			}
		}
	}
	return false
}

func (s *SearchList) HandleSizeMsg(msg teapb.ResizeMsg) {
	teapb.DebugPrintf("SearchList received size message: %+v\n", msg)
	s.List.SetWidth(msg.Width)
	s.List.SetHeight(msg.Height - 1) //1 for the search input
}

func NewSearchList[T list.DefaultItem](items []T, config SearchListConfig, d list.ItemDelegate) SearchList {
	ditems := convertToDefaultItems(items)
	if d == nil {
		d = SelectableItemsDelegate(key.NewBinding(), func(item interface{}, selected bool) tea.Cmd {
			return nil
		})
	}

	l := list.New(ditems, d, config.Width, config.Height)
	l.SetShowFilter(false)
	l.SetShowTitle(false)
	l.Filter = MakeSearchFunc(SearchOption{MatchesOnly: false, CaseSensitive: false})
	l.Title = "Select notebooks"
	l.Styles.StatusBar = l.Styles.StatusBar.Padding(0, 0, 1)
	l.Styles.TitleBar = l.Styles.TitleBar.Padding(0, 0)
	l.SetHeight(config.Height)
	ti := textinput.New()
	ti.Prompt = "🔍: "
	ti.Width = config.Width
	l.DisableQuitKeybindings()
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{}
	}
	l.KeyMap.CloseFullHelp.SetEnabled(false)
	l.SetShowHelp(false)
	return SearchList{
		List:        l,
		SearchInput: ti,
	}
}

type DefaultItemDelegateAlt struct {
	list.DefaultDelegate
	itemSelected map[int]bool
}
