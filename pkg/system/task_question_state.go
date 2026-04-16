package system

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type StarterTaskStatus string

const (
	TaskStatusPending        StarterTaskStatus = "pending"
	TaskStatusInProgress     StarterTaskStatus = "in_progress"
	TaskStatusWaitingForUser StarterTaskStatus = "waiting_for_user"
	TaskStatusCompleted      StarterTaskStatus = "completed"
	TaskStatusFailed         StarterTaskStatus = "failed"
	TaskStatusStopped        StarterTaskStatus = "stopped"
)

type StarterQuestionStatus string

const (
	QuestionStatusPending  StarterQuestionStatus = "pending"
	QuestionStatusAnswered StarterQuestionStatus = "answered"
)

type StarterTaskRecord struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Input      string            `json:"input"`
	Status     StarterTaskStatus `json:"status"`
	QuestionID *string           `json:"question_id,omitempty"`
	Answer     *string           `json:"answer,omitempty"`
	Output     *string           `json:"output,omitempty"`
	CreatedAt  string            `json:"created_at"`
	UpdatedAt  string            `json:"updated_at"`
}

type StarterQuestionRecord struct {
	ID         string                `json:"id"`
	TaskID     string                `json:"task_id"`
	Text       string                `json:"text"`
	Status     StarterQuestionStatus `json:"status"`
	Answer     *string               `json:"answer,omitempty"`
	CreatedAt  string                `json:"created_at"`
	AnsweredAt *string               `json:"answered_at,omitempty"`
}

type taskQuestionSnapshot struct {
	NextTaskIndex     int                     `json:"next_task_index"`
	NextQuestionIndex int                     `json:"next_question_index"`
	Tasks             []StarterTaskRecord     `json:"tasks"`
	Questions         []StarterQuestionRecord `json:"questions"`
}

type TaskQuestionStateStore struct {
	path              string
	transcriptPath    string
	nextTaskIndex     int
	nextQuestionIndex int
	tasks             []StarterTaskRecord
	questions         []StarterQuestionRecord
}

func resolveTaskQuestionStatePath(explicitPath string) string {
	if strings.TrimSpace(explicitPath) != "" {
		return explicitPath
	}
	if envPath := strings.TrimSpace(os.Getenv("EMBER_TASK_STATE_PATH")); envPath != "" {
		return envPath
	}
	if configHome := strings.TrimSpace(os.Getenv("EMBER_CONFIG_HOME")); configHome != "" {
		return filepath.Join(configHome, "task-question-state.json")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".emberforge", "task-question-state.json")
	}
	return filepath.Join(".emberforge", "task-question-state.json")
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func NewTaskQuestionStateStore(path string) *TaskQuestionStateStore {
	store := &TaskQuestionStateStore{
		path:              resolveTaskQuestionStatePath(path),
		transcriptPath:    filepath.Join(filepath.Dir(resolveTaskQuestionStatePath(path)), "task-question-transcript.jsonl"),
		nextTaskIndex:     1,
		nextQuestionIndex: 1,
		tasks:             []StarterTaskRecord{},
		questions:         []StarterQuestionRecord{},
	}
	store.load()
	return store
}

func (s *TaskQuestionStateStore) load() {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var snapshot taskQuestionSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return
	}
	if snapshot.NextTaskIndex > 0 {
		s.nextTaskIndex = snapshot.NextTaskIndex
	}
	if snapshot.NextQuestionIndex > 0 {
		s.nextQuestionIndex = snapshot.NextQuestionIndex
	}
	s.tasks = snapshot.Tasks
	s.questions = snapshot.Questions
}

func (s *TaskQuestionStateStore) persist() {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return
	}
	raw, err := json.MarshalIndent(taskQuestionSnapshot{
		NextTaskIndex:     s.nextTaskIndex,
		NextQuestionIndex: s.nextQuestionIndex,
		Tasks:             s.tasks,
		Questions:         s.questions,
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, append(raw, '\n'), 0o644)
}

func (s *TaskQuestionStateStore) appendTranscript(blocks []map[string]any) {
	if err := os.MkdirAll(filepath.Dir(s.transcriptPath), 0o755); err != nil {
		return
	}

	if _, err := os.Stat(s.transcriptPath); err != nil {
		meta, metaErr := json.Marshal(map[string]any{
			"type":      "session",
			"id":        "task-question-runtime",
			"createdAt": nowISO(),
			"planMode":  false,
		})
		if metaErr == nil {
			_ = os.WriteFile(s.transcriptPath, append(meta, '\n'), 0o644)
		}
	}

	file, err := os.OpenFile(s.transcriptPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()

	record, err := json.Marshal(map[string]any{
		"type":      "message",
		"role":      "system",
		"blocks":    blocks,
		"timestamp": nowISO(),
	})
	if err != nil {
		return
	}
	writer := bufio.NewWriter(file)
	_, _ = writer.Write(append(record, '\n'))
	_ = writer.Flush()
}

func (s *TaskQuestionStateStore) CreatePromptTask(input string) StarterTaskRecord {
	task := StarterTaskRecord{
		ID:        fmt.Sprintf("task-%d", s.nextTaskIndex),
		Kind:      "prompt",
		Input:     strings.TrimSpace(input),
		Status:    TaskStatusInProgress,
		CreatedAt: nowISO(),
		UpdatedAt: nowISO(),
	}
	s.nextTaskIndex++
	s.tasks = append(s.tasks, task)
	s.persist()
	s.appendTranscript([]map[string]any{
		{
			"type":    "task_state",
			"task_id": task.ID,
			"status":  task.Status,
			"input":   task.Input,
		},
	})
	return task
}

func (s *TaskQuestionStateStore) ListTasks() []StarterTaskRecord {
	return append([]StarterTaskRecord(nil), s.tasks...)
}

func (s *TaskQuestionStateStore) GetTask(taskID string) (StarterTaskRecord, bool) {
	for _, task := range s.tasks {
		if task.ID == taskID {
			return task, true
		}
	}
	return StarterTaskRecord{}, false
}

func (s *TaskQuestionStateStore) AskQuestion(taskID string, text string) (StarterTaskRecord, StarterQuestionRecord, error) {
	for index, task := range s.tasks {
		if task.ID != taskID {
			continue
		}
		switch task.Status {
		case TaskStatusCompleted, TaskStatusFailed, TaskStatusStopped:
			return StarterTaskRecord{}, StarterQuestionRecord{}, fmt.Errorf("task is not active: %s", taskID)
		}
		question := StarterQuestionRecord{
			ID:        fmt.Sprintf("question-%d", s.nextQuestionIndex),
			TaskID:    taskID,
			Text:      strings.TrimSpace(text),
			Status:    QuestionStatusPending,
			CreatedAt: nowISO(),
		}
		s.nextQuestionIndex++
		task.QuestionID = &question.ID
		task.Status = TaskStatusWaitingForUser
		task.UpdatedAt = nowISO()
		s.tasks[index] = task
		s.questions = append(s.questions, question)
		s.persist()
		s.appendTranscript([]map[string]any{
			{
				"type":        "question_state",
				"question_id": question.ID,
				"task_id":     question.TaskID,
				"status":      question.Status,
				"text":        question.Text,
			},
			{
				"type":        "task_state",
				"task_id":     task.ID,
				"status":      task.Status,
				"question_id": question.ID,
			},
		})
		return task, question, nil
	}
	return StarterTaskRecord{}, StarterQuestionRecord{}, fmt.Errorf("task not found: %s", taskID)
}

func (s *TaskQuestionStateStore) ListQuestions(status StarterQuestionStatus) []StarterQuestionRecord {
	if status == "" {
		return append([]StarterQuestionRecord(nil), s.questions...)
	}
	result := make([]StarterQuestionRecord, 0)
	for _, question := range s.questions {
		if question.Status == status {
			result = append(result, question)
		}
	}
	return result
}

func (s *TaskQuestionStateStore) AnswerQuestion(questionID string, answer string) (StarterTaskRecord, StarterQuestionRecord, error) {
	for qIndex, question := range s.questions {
		if question.ID != questionID {
			continue
		}
		if question.Status == QuestionStatusAnswered {
			return StarterTaskRecord{}, StarterQuestionRecord{}, fmt.Errorf("question already answered: %s", questionID)
		}
		trimmedAnswer := strings.TrimSpace(answer)
		answeredAt := nowISO()
		question.Status = QuestionStatusAnswered
		question.Answer = &trimmedAnswer
		question.AnsweredAt = &answeredAt
		s.questions[qIndex] = question

		for tIndex, task := range s.tasks {
			if task.ID != question.TaskID {
				continue
			}
			task.Answer = &trimmedAnswer
			output := fmt.Sprintf("Task resumed after %s and completed with answer: %s", question.ID, trimmedAnswer)
			task.Output = &output
			task.Status = TaskStatusCompleted
			task.UpdatedAt = nowISO()
			s.tasks[tIndex] = task
			s.persist()
			s.appendTranscript([]map[string]any{
				{
					"type":        "question_state",
					"question_id": question.ID,
					"task_id":     question.TaskID,
					"status":      question.Status,
					"answer":      trimmedAnswer,
				},
				{
					"type":        "task_state",
					"task_id":     task.ID,
					"status":      task.Status,
					"question_id": question.ID,
					"answer":      trimmedAnswer,
					"output":      output,
				},
			})
			return task, question, nil
		}
		return StarterTaskRecord{}, StarterQuestionRecord{}, fmt.Errorf("task not found for question: %s", question.TaskID)
	}
	return StarterTaskRecord{}, StarterQuestionRecord{}, fmt.Errorf("question not found: %s", questionID)
}

func (s *TaskQuestionStateStore) StopTask(taskID string) (StarterTaskRecord, error) {
	for index, task := range s.tasks {
		if task.ID != taskID {
			continue
		}
		switch task.Status {
		case TaskStatusCompleted, TaskStatusFailed:
			return task, nil
		default:
			task.Status = TaskStatusStopped
			task.UpdatedAt = nowISO()
			s.tasks[index] = task
			s.persist()
			s.appendTranscript([]map[string]any{
				{
					"type":    "task_state",
					"task_id": task.ID,
					"status":  task.Status,
				},
			})
			return task, nil
		}
	}
	return StarterTaskRecord{}, fmt.Errorf("task not found: %s", taskID)
}
