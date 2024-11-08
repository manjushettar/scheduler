package main

import (
    "scheduler/db"
    "fmt"
    "os"
    "time"
    "strconv"
    
    "github.com/charmbracelet/bubbles/textinput"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    
    "log"
)

var (
    appStyle = lipgloss.NewStyle().
        Padding(1, 2).
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("39")).
        Width(50)
    
    headerStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("219")).
        MarginBottom(1)
    
    timeSlotStyle = lipgloss.NewStyle().
        PaddingLeft(1).
        PaddingRight(1).
        Border(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("241")).
        Width(44)
    
    selectedTimeSlotStyle = lipgloss.NewStyle().
        PaddingLeft(1).
        PaddingRight(1).
        Border(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("218")).
        Foreground(lipgloss.Color("255")).
        Width(44)
    
    currentTimeSlotStyle = lipgloss.NewStyle().
        PaddingLeft(1).
        PaddingRight(1).
        Border(lipgloss.NormalBorder()).
        BorderForeground(lipgloss.Color("39"))

    taskStyle = lipgloss.NewStyle().
        PaddingLeft(1).
        Foreground(lipgloss.Color("219"))
    
    normalTaskStyle = lipgloss.NewStyle().
        PaddingLeft(1).
        Foreground(lipgloss.Color("86"))
    
    selectedTaskStyle = lipgloss.NewStyle().
        PaddingLeft(1).
        Bold(true).
        Foreground(lipgloss.Color("0")).
        Foreground(lipgloss.Color("86"))

    formStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("63")).
        Padding(1, 2).
        MarginTop(1).
        MarginBottom(1)
)

type tickMsg time.Time

const ID = 1

func doTick() tea.Msg {
    return tickMsg(time.Now())
}

type TimeSlot struct {
    StartTime time.Time
    Tasks     []Task
}

type Task struct {
    Time     time.Time
    Duration int 
    Title    string
    Done     bool
    ID       int64
}

type mode int

const (
    normalMode mode = iota
    taskCreationMode
    taskSelectionMode
)

type model struct {
    db          *db.DB
    currentDate time.Time
    currentTimeSlot int
    timeSlots   []TimeSlot
    cursor      int
    selected    int
    taskCursor int
    viewport    viewport
    mode        mode
    taskForm    taskForm
    errorMsg    string
    errorTimer  time.Time
    deletePending bool
}

type taskForm struct {
    titleInput    textinput.Model
    durationInput textinput.Model
    activeInput   int
    err          string
}

type viewport struct {
    top    int
    bottom int
    height int
}

func timeToSlotIndex(t time.Time) int {
    minutes := t.Hour() * 60 + t.Minute()
    return minutes / 30
}

func (m model) currentTaskCount () int {
    if m.cursor >= 0 && m.cursor < len(m.timeSlots){
        return len(m.timeSlots[m.cursor].Tasks)
    }
    return 0
}

func generateTimeSlots(date time.Time) []TimeSlot {
    slots := make([]TimeSlot, 48)
    
    baseTime := time.Date(
        date.Year(), date.Month(), date.Day(),
        0, 0, 0, 0, date.Location(),
    )
    
    for i := range slots {
        slots[i] = TimeSlot{
            StartTime: baseTime.Add(time.Duration(i) * 30 * time.Minute),
            Tasks:     make([]Task, 0),
        }
    }
    
    return slots
}

func initialTaskForm() taskForm {
    ti := textinput.New()
    ti.Placeholder = "Task title"
    ti.Focus()
    ti.CharLimit = 50
    ti.Width = 40

    di := textinput.New()
    di.Placeholder = "Duration (minutes)"
    di.CharLimit = 3
    
    return taskForm{
        titleInput:    ti,
        durationInput: di,
        activeInput:   0,
    }
}

func initialModel() model {
    database, err := db.NewDB()
    if err != nil {
        log.Fatalf("Failed to initialize database: %v\n", err)
    }

    currentDate := time.Now()
    currentTime := timeToSlotIndex(currentDate)
    m := model{
        db:          database,
        currentDate: currentDate,
        currentTimeSlot: currentTime,
        timeSlots:   generateTimeSlots(currentDate),
        cursor:      currentTime,
        selected:    currentTime,
        viewport: viewport{
            top:    currentTime - 3,
            bottom: currentTime + 2,
            height: 6,
        },
        taskCursor: 0,
        deletePending: false,
        mode:     normalMode,
        taskForm: initialTaskForm(),
    }

    if err := m.loadTasks(); err != nil {
        log.Printf("Failed to load initial tasks: %v\n", err)
    }
    return m
}

func (m *model) jumpToCurrentTime() {
    now := time.Now()
    m.currentDate = now
    m.currentTimeSlot = timeToSlotIndex(now)
    m.cursor = m.currentTimeSlot
    m.timeSlots = generateTimeSlots(m.currentDate)
    
    m.viewport.top = m.currentTimeSlot - 3
    m.viewport.bottom = m.currentTimeSlot + 2
    
    if m.viewport.top < 0 {
        m.viewport.top = 0
        m.viewport.bottom = 5
    }
    if m.viewport.bottom >= len(m.timeSlots) {
        m.viewport.bottom = len(m.timeSlots) - 1
        m.viewport.top = m.viewport.bottom - 5
    }
    
    if err := m.loadTasks(); err != nil {
        m.errorMsg = fmt.Sprintf("Failed to load tasks: %v", err)
        m.errorTimer = time.Now()
    }
}


func (m *model) loadTasks() error {
    tasks, err := m.db.GetTasksForDate(m.currentDate)
    if err != nil {
        return err
    }
    
    for i := range m.timeSlots {
        m.timeSlots[i].Tasks = nil
    }
    
    for _, task := range tasks {
        if task.TimeSlot >= 0 && task.TimeSlot < len(m.timeSlots) {
            m.timeSlots[task.TimeSlot].Tasks = append(
                m.timeSlots[task.TimeSlot].Tasks,
                Task{
                    Time:     m.timeSlots[task.TimeSlot].StartTime,
                    Duration: task.Duration,
                    Title:    task.Title,
                    Done:     task.Done,
                    ID:       task.ID,
                },
            )
        }
    }
    
    return nil
}



func (m model) Init() tea.Cmd {
    return textinput.Blink
}

func (m *model) updateViewport() {
    if m.cursor > m.viewport.bottom {
        diff := m.cursor - m.viewport.bottom
        m.viewport.top += diff
        m.viewport.bottom += diff
    }
    
    if m.cursor < m.viewport.top {
        diff := m.viewport.top - m.cursor
        m.viewport.top -= diff
        m.viewport.bottom -= diff
    }
    
    if m.viewport.bottom >= len(m.timeSlots) {
        m.viewport.bottom = len(m.timeSlots) - 1
        m.viewport.top = m.viewport.bottom - m.viewport.height + 1
    }
    
    if m.viewport.top < 0 {
        m.viewport.top = 0
        m.viewport.bottom = m.viewport.top + m.viewport.height - 1
    }
}

func formatTimeSlot(slot TimeSlot) string {
    startTime := slot.StartTime.Format("3:04 PM") 
    endTime := slot.StartTime.Add(30 * time.Minute).Format("3:04 PM")
    return fmt.Sprintf("%s - %s", startTime, endTime)
}


func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    var cmd tea.Cmd
    var cmds []tea.Cmd

    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch m.mode {
        case normalMode:
            switch msg.String() {
            case "ctrl+c", "q":
                return m, tea.Quit
            case "left":
                m.currentDate = m.currentDate.AddDate(0, 0, -1)
                m.timeSlots = generateTimeSlots(m.currentDate)
                if err := m.loadTasks(); err != nil {
                    m.errorMsg = fmt.Sprintf("Failed to load tasks: %v", err)
                    m.errorTimer = time.Now()
                }
            case "right":
                m.currentDate = m.currentDate.AddDate(0, 0, 1)
                m.timeSlots = generateTimeSlots(m.currentDate)
                if err := m.loadTasks(); err != nil {
                    m.errorMsg = fmt.Sprintf("Failed to load tasks: %v", err)
                    m.errorTimer = time.Now()
                }
            case "up":
                if m.cursor > 0 {
                    m.cursor--
                    m.updateViewport()
                }
            case "down":
                if m.cursor < len(m.timeSlots)-1 {
                    m.cursor++
                    m.updateViewport()
                }
            case "n":
                m.selected = m.cursor
                m.mode = taskCreationMode
                m.taskForm = initialTaskForm()
                return m, textinput.Blink
            case "enter":
                if len(m.timeSlots[m.cursor].Tasks) > 0 {
                    m.mode = taskSelectionMode
                    m.taskCursor = 0
                }
            case "t", "T":
                now := time.Now()
                m.currentDate = now
                m.currentTimeSlot = timeToSlotIndex(now)
                m.cursor = m.currentTimeSlot
                m.timeSlots = generateTimeSlots(m.currentDate)
                
                m.viewport.top = m.currentTimeSlot - 3
                m.viewport.bottom = m.currentTimeSlot + 2
                
                if m.viewport.top < 0 {
                    m.viewport.top = 0
                    m.viewport.bottom = 5
                }
                if m.viewport.bottom >= len(m.timeSlots) {
                    m.viewport.bottom = len(m.timeSlots) - 1
                    m.viewport.top = m.viewport.bottom - 5
                }
                
                if err := m.loadTasks(); err != nil {
                    m.errorMsg = fmt.Sprintf("Failed to load tasks: %v", err)
                    m.errorTimer = time.Now()
                }
            }
        
        case taskSelectionMode:
            switch msg.String() {
            case "esc":
                m.mode = normalMode
                m.taskCursor = 0
                m.deletePending = false
            case "up":
                if m.taskCursor > 0 {
                    m.taskCursor--
                }
            case "down":
                if m.taskCursor < len(m.timeSlots[m.cursor].Tasks)-1 {
                    m.taskCursor++
                }
            case "d":
                if !m.deletePending{
                    m.deletePending = true
                    return m, nil
                }
                if m.deletePending {
                    tasks := m.timeSlots[m.cursor].Tasks
                    if len(tasks) > 0 && m.taskCursor < len(tasks) {
                        taskID := tasks[m.taskCursor].ID
                        if err := m.db.DeleteTask(taskID); err != nil {
                            m.errorMsg = fmt.Sprintf("Failed to delete task: %v", err)
                            m.errorTimer = time.Now()
                        } else {
                            m.timeSlots[m.cursor].Tasks = append(
                                tasks[:m.taskCursor],
                                tasks[m.taskCursor+1:]...
                            )
                            
                            if len(m.timeSlots[m.cursor].Tasks) == 0 {
                                m.mode = normalMode
                            } else if m.taskCursor >= len(m.timeSlots[m.cursor].Tasks) {
                                m.taskCursor = len(m.timeSlots[m.cursor].Tasks) - 1
                            }
                        }
                    }
                    m.deletePending = false
                } 
            default:
                m.deletePending = false
            }
        
        case taskCreationMode:
            switch msg.String() {
            case "esc":
                m.mode = normalMode
                m.taskForm.err = ""
            case "tab":
                m.taskForm.activeInput = (m.taskForm.activeInput + 1) % 2
                if m.taskForm.activeInput == 0 {
                    m.taskForm.titleInput.Focus()
                    m.taskForm.durationInput.Blur()
                } else {
                    m.taskForm.titleInput.Blur()
                    m.taskForm.durationInput.Focus()
                }
            case "enter":
                if m.taskForm.titleInput.Value() == "" {
                    m.taskForm.err = "Title cannot be empty"
                    return m, nil
                }

                duration := 30 
                if m.taskForm.durationInput.Value() != "" {
                    var err error
                    duration, err = strconv.Atoi(m.taskForm.durationInput.Value())
                    if err != nil || duration <= 0 {
                        m.taskForm.err = "Invalid duration"
                        return m, nil
                    }
                }
                
                err := m.db.SaveTask(
                    m.currentDate,
                    m.cursor,
                    m.taskForm.titleInput.Value(),
                    duration,
                )
                if err != nil {
                    m.taskForm.err = "Failed to save task"
                    return m, nil
                }
                
                err = m.loadTasks()
                if err != nil {
                    m.taskForm.err = "Failed to reload tasks"
                    return m, nil
                }
                
                m.mode = normalMode
                m.taskForm = initialTaskForm()
                return m, nil
            }
            
            if m.taskForm.activeInput == 0 {
                m.taskForm.titleInput, cmd = m.taskForm.titleInput.Update(msg)
                cmds = append(cmds, cmd)
            } else {
                m.taskForm.durationInput, cmd = m.taskForm.durationInput.Update(msg)
                cmds = append(cmds, cmd)
            }
        }
    
    case tickMsg:
        newTimeSlot := timeToSlotIndex(time.Time(msg))
        if newTimeSlot != m.currentTimeSlot {
            m.currentTimeSlot = newTimeSlot
            return m, nil
        }
    }

    return m, tea.Batch(cmds...)
}

func (m model) View() string {
    // Header with current date
    header := headerStyle.Render(fmt.Sprintf("📅 %s", m.currentDate.Format("Monday, January 2, 2006")))
    
    // Time slots view
    var slots string
    for i := m.viewport.top; i <= m.viewport.bottom; i++ {
        slot := m.timeSlots[i]
        timeStr := formatTimeSlot(slot)
        
        // Determine time slot style
        var style lipgloss.Style
        switch {
        case i == m.cursor && i == m.currentTimeSlot && m.currentDate.Format("2006-01-02") == time.Now().Format("2006-01-02"):
            style = selectedTimeSlotStyle.Copy().Background(lipgloss.Color("52"))
        case i == m.cursor:
            style = selectedTimeSlotStyle
        case i == m.currentTimeSlot && m.currentDate.Format("2006-01-02") == time.Now().Format("2006-01-02"):
            style = currentTimeSlotStyle
        default:
            style = timeSlotStyle
        }
        
        slots += style.Render(timeStr) + "\n"
        
        // Show tasks for this time slot
        if len(slot.Tasks) > 0 {
            for taskIndex, task := range slot.Tasks {
                var taskStyle lipgloss.Style
                
                // Apply selected task style in task selection mode
                if i == m.cursor && m.mode == taskSelectionMode && taskIndex == m.taskCursor {
                    taskStyle = selectedTaskStyle
                } else {
                    taskStyle = normalTaskStyle
                }
                
                taskStr := fmt.Sprintf(" • %s (%dm)", task.Title, task.Duration)
                if task.Done {
                    taskStr = fmt.Sprintf(" ✓ %s", task.Title)
                }
                slots += taskStyle.Render(taskStr) + "\n"
            }
        }
    }
    
    // Task creation form
    var form string
    if m.mode == taskCreationMode {
        form = formStyle.Render(fmt.Sprintf(
            "New Task at %s\n\n%s\n%s\n\n%s\n\nTab: Switch fields • Enter: Save • Esc: Cancel",
            formatTimeSlot(m.timeSlots[m.cursor]),
            m.taskForm.titleInput.View(),
            m.taskForm.durationInput.View(),
            m.taskForm.err,
        ))
    }
    
    // Help text
    var help string
    switch m.mode {
    case normalMode:
        help = "\nNavigate: ↑/↓ • Change Day: ←/→ • New Task: n • Enter Time Slot: Enter • Current Time: T • Quit: q"
    case taskSelectionMode:
        help = "\nNavigate Tasks: ↑/↓ • Delete: dd • Exit Selection: Esc"
    case taskCreationMode:
        help = "\nTab: Switch fields • Enter: Save • Esc: Cancel"
    }
    
    // Error message
    var errorDisplay string
    if m.errorMsg != "" && time.Since(m.errorTimer) < 3*time.Second {
        errorStyle := lipgloss.NewStyle().
            Foreground(lipgloss.Color("196")).
            Margin(1)
        errorDisplay = errorStyle.Render(m.errorMsg)
    }
    
    return appStyle.Render(header + "\n" + slots + errorDisplay + form + help)
}
func main() {
    p := tea.NewProgram(initialModel(), tea.WithAltScreen())
    go func() {
        ticker := time.NewTicker(time.Minute)
        defer ticker.Stop()
        
        for range ticker.C {
            p.Send(doTick())
        }
    }()

    if _, err := p.Run(); err != nil {
        fmt.Printf("Error running program: %v", err)
        os.Exit(1)
    }
}
