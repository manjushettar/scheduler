package db

import (
    "database/sql"
    "fmt"
    "os"
    "path/filepath"
    "time"
    
    _ "github.com/mattn/go-sqlite3"
)

type DB struct {
    *sql.DB
}

type Task struct {
    ID        int64
    Date      string
    TimeSlot  int
    Title     string
    Duration  int
    Done      bool
    CreatedAt time.Time
}

func NewDB() (*DB, error) {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return nil, fmt.Errorf("failed to get home directory: %v", err)
    }
    
    dbDir := filepath.Join(homeDir, ".scheduler")
    if err := os.MkdirAll(dbDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create database directory: %v", err)
    }
    
    dbPath := filepath.Join(dbDir, "scheduler.db")
    
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %v", err)
    }
    
    db.SetMaxOpenConns(1)
    db.SetMaxIdleConns(1)
    db.SetConnMaxLifetime(time.Hour)
    
    if err := initSchema(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to initialize schema: %v", err)
    }
    
    return &DB{db}, nil
}

func initSchema(db *sql.DB) error {
    schema := `
    CREATE TABLE IF NOT EXISTS tasks (
        id INTEGER PRIMARY KEY,
        date TEXT NOT NULL,
        time_slot INTEGER NOT NULL,
        title TEXT NOT NULL,
        duration INTEGER NOT NULL,
        done BOOLEAN NOT NULL DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_tasks_date ON tasks(date);
    `
    
    _, err := db.Exec(schema)
    return err
}

func (db *DB) SaveTask(date time.Time, timeSlot int, title string, duration int) error {
    dateStr := date.Format("2006-01-02")
    
    _, err := db.Exec(`
        INSERT INTO tasks (date, time_slot, title, duration, done)
        VALUES (?, ?, ?, ?, ?)
    `, dateStr, timeSlot, title, duration, false)
    
    return err
}

func (db *DB) GetTasksForDate(date time.Time) ([]Task, error) {
    dateStr := date.Format("2006-01-02")
    
    rows, err := db.Query(`
        SELECT id, time_slot, title, duration, done, created_at
        FROM tasks
        WHERE date = ?
        ORDER BY time_slot
    `, dateStr)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var tasks []Task
    for rows.Next() {
        var t Task
        err := rows.Scan(&t.ID, &t.TimeSlot, &t.Title, &t.Duration, &t.Done, &t.CreatedAt)
        if err != nil {
            return nil, err
        }
        t.Date = dateStr
        tasks = append(tasks, t)
    }
    
    return tasks, rows.Err()
}

func (db *DB) UpdateTaskDone(taskID int64, done bool) error {
    _, err := db.Exec(`
        UPDATE tasks
        SET done = ?
        WHERE id = ?
    `, done, taskID)
    return err
}

func (db *DB) DeleteTask(taskID int64) error {
    _, err := db.Exec(`
        DELETE FROM tasks
        WHERE id = ?
    `, taskID)
    return err
}
