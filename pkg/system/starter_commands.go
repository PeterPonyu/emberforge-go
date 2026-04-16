package system

import (
	"fmt"
	"os"
	"strings"
)

func renderStarterHelp(app *StarterSystemApplication) string {
	lines := []string{"available commands:"}
	for _, command := range app.CommandRegistry.List() {
		suffix := ""
		if strings.TrimSpace(command.ArgumentHint) != "" {
			suffix = " " + command.ArgumentHint
		}
		lines = append(lines, fmt.Sprintf("  /%s%s -- %s", command.Name, suffix, command.Description))
	}
	return strings.Join(lines, "\n")
}

func renderTask(task StarterTaskRecord, header string) string {
	questionID := "none"
	if task.QuestionID != nil {
		questionID = *task.QuestionID
	}
	answer := "none"
	if task.Answer != nil {
		answer = *task.Answer
	}
	output := "none"
	if task.Output != nil {
		output = *task.Output
	}
	return strings.Join([]string{
		header,
		fmt.Sprintf("task_id: %s", task.ID),
		fmt.Sprintf("kind: %s", task.Kind),
		fmt.Sprintf("status: %s", task.Status),
		fmt.Sprintf("input: %s", task.Input),
		fmt.Sprintf("question_id: %s", questionID),
		fmt.Sprintf("answer: %s", answer),
		fmt.Sprintf("output: %s", output),
	}, "\n")
}

func renderPendingQuestions(questions []StarterQuestionRecord) string {
	if len(questions) == 0 {
		return strings.Join([]string{"[command] questions pending", "pending: 0"}, "\n")
	}
	lines := []string{"[command] questions pending", fmt.Sprintf("pending: %d", len(questions))}
	for _, question := range questions {
		lines = append(lines, fmt.Sprintf("%s -> %s :: %s", question.ID, question.TaskID, question.Text))
	}
	return strings.Join(lines, "\n")
}

func executeTasksCommand(app *StarterSystemApplication, payload string) string {
	parts := strings.Fields(strings.TrimSpace(payload))
	action := "list"
	if len(parts) > 0 {
		action = parts[0]
		parts = parts[1:]
	}

	switch action {
	case "create":
		if len(parts) < 2 || parts[0] != "prompt" {
			return "[command] tasks: usage /tasks create prompt <text>"
		}
		task := app.TaskQuestionStore.CreatePromptTask(strings.Join(parts[1:], " "))
		return renderTask(task, "[command] tasks create")
	case "list":
		tasks := app.TaskQuestionStore.ListTasks()
		if len(tasks) == 0 {
			return strings.Join([]string{"[command] tasks list", "tasks: 0"}, "\n")
		}
		lines := []string{"[command] tasks list", fmt.Sprintf("tasks: %d", len(tasks))}
		for _, task := range tasks {
			lines = append(lines, fmt.Sprintf("%s :: %s :: %s", task.ID, task.Status, task.Input))
		}
		return strings.Join(lines, "\n")
	case "show":
		if len(parts) < 1 {
			return "[command] tasks: usage /tasks show <task-id>"
		}
		task, ok := app.TaskQuestionStore.GetTask(parts[0])
		if !ok {
			return fmt.Sprintf("[command] tasks: task not found %s", parts[0])
		}
		return renderTask(task, "[command] tasks show")
	case "stop":
		if len(parts) < 1 {
			return "[command] tasks: usage /tasks stop <task-id>"
		}
		task, err := app.TaskQuestionStore.StopTask(parts[0])
		if err != nil {
			return fmt.Sprintf("[command] tasks: %s", err.Error())
		}
		return renderTask(task, "[command] tasks stop")
	default:
		return fmt.Sprintf("[command] tasks: unsupported action %s", action)
	}
}

func executeQuestionsCommand(app *StarterSystemApplication, payload string) string {
	parts := strings.Fields(strings.TrimSpace(payload))
	action := "pending"
	if len(parts) > 0 {
		action = parts[0]
		parts = parts[1:]
	}

	switch action {
	case "pending":
		return renderPendingQuestions(app.TaskQuestionStore.ListQuestions(QuestionStatusPending))
	case "ask":
		if len(parts) < 2 {
			return "[command] questions: usage /questions ask <task-id> <text>"
		}
		task, question, err := app.TaskQuestionStore.AskQuestion(parts[0], strings.Join(parts[1:], " "))
		if err != nil {
			return fmt.Sprintf("[command] questions: %s", err.Error())
		}
		return strings.Join([]string{
			"[command] questions ask",
			fmt.Sprintf("question_id: %s", question.ID),
			fmt.Sprintf("task_id: %s", task.ID),
			fmt.Sprintf("status: %s", task.Status),
			fmt.Sprintf("question: %s", question.Text),
		}, "\n")
	case "answer":
		if len(parts) < 2 {
			return "[command] questions: usage /questions answer <question-id> <text>"
		}
		task, question, err := app.TaskQuestionStore.AnswerQuestion(parts[0], strings.Join(parts[1:], " "))
		if err != nil {
			return fmt.Sprintf("[command] questions: %s", err.Error())
		}
		answer := ""
		if question.Answer != nil {
			answer = *question.Answer
		}
		return strings.Join([]string{
			"[command] questions answer",
			fmt.Sprintf("question_id: %s", question.ID),
			fmt.Sprintf("task_id: %s", task.ID),
			fmt.Sprintf("task_status: %s", task.Status),
			fmt.Sprintf("answer: %s", answer),
		}, "\n")
	default:
		return fmt.Sprintf("[command] questions: unsupported action %s", action)
	}
}

func ExecuteStarterSlashCommand(app *StarterSystemApplication, input string) (string, bool) {
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, "/") {
		return "", false
	}

	withoutSlash := strings.TrimPrefix(trimmed, "/")
	parts := strings.Fields(withoutSlash)
	if len(parts) == 0 {
		return "", false
	}
	commandName := parts[0]
	payload := ""
	if len(parts) > 1 {
		payload = strings.Join(parts[1:], " ")
	}
	report := app.Report()

	switch commandName {
	case "help":
		return renderStarterHelp(app), true
	case "status":
		return fmt.Sprintf("[command] status: lifecycle=%s handled=%d turns=%d", report.LifecycleState, report.HandledRequestCount, report.TurnCount), true
	case "doctor":
		if payload == "" || payload == "quick" {
			return BuildDoctorReport(report), true
		}
		if payload == "status" {
			return strings.Join([]string{
				"emberforge-go doctor status",
				fmt.Sprintf("lifecycle: %s", report.LifecycleState),
				fmt.Sprintf("handled_requests: %d", report.HandledRequestCount),
				fmt.Sprintf("turns: %d", report.TurnCount),
				fmt.Sprintf("last_route: %s", fallbackString(report.LastRoute, "none")),
			}, "\n"), true
		}
		return fmt.Sprintf("[command] doctor: unsupported mode %s", payload), true
	case "model":
		activeModel := os.Getenv("OLLAMA_MODEL")
		if strings.TrimSpace(activeModel) == "" {
			activeModel = os.Getenv("EMBER_MODEL")
		}
		if strings.TrimSpace(activeModel) == "" {
			activeModel = "qwen3:8b"
		}
		if payload == "list" {
			return fmt.Sprintf("[command] model list: %s", activeModel), true
		}
		if strings.TrimSpace(payload) == "" {
			payload = activeModel
		}
		return fmt.Sprintf("[command] model: %s", payload), true
	case "questions":
		return executeQuestionsCommand(app, payload), true
	case "tasks":
		return executeTasksCommand(app, payload), true
	case "buddy":
		return ExecuteBuddyCommand(app.Buddy, payload), true
	case "compact":
		return fmt.Sprintf("[command] compact: turns=%d handled=%d lifecycle=%s", report.TurnCount, report.HandledRequestCount, report.LifecycleState), true
	case "review":
		return strings.Join([]string{
			"[command] review",
			fmt.Sprintf("scope: %s", fallbackString(payload, "workspace")),
			fmt.Sprintf("commands: %d", report.CommandCount),
			fmt.Sprintf("tools: %d", report.ToolCount),
			fmt.Sprintf("plugins: %d", report.PluginCount),
			"note: starter translation review placeholder",
		}, "\n"), true
	case "commit":
		return strings.Join([]string{
			"[command] commit",
			fmt.Sprintf("summary: %s", fallbackString(payload, "starter translation update")),
			fmt.Sprintf("lifecycle: %s", report.LifecycleState),
			fmt.Sprintf("turns: %d", report.TurnCount),
			"note: starter commit workflow placeholder",
		}, "\n"), true
	case "pr":
		return strings.Join([]string{
			"[command] pr",
			fmt.Sprintf("context: %s", fallbackString(payload, "starter translation update")),
			fmt.Sprintf("commands: %d", report.CommandCount),
			fmt.Sprintf("handled_requests: %d", report.HandledRequestCount),
			"note: starter pull request workflow placeholder",
		}, "\n"), true
	default:
		return "", false
	}
}

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
