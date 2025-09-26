package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/andrejsoucek/chronos/internal/action"
	"github.com/andrejsoucek/chronos/pkg/clockify"
	"github.com/andrejsoucek/chronos/pkg/gitlab"
	"github.com/andrejsoucek/chronos/pkg/linear"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"
)

func main() {
	projectId, l, g, cify := loadConfiguration()
	cmd := createCommands(projectId, l, g, cify)

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func loadConfiguration() (string, *linear.Linear, *gitlab.Gitlab, *clockify.Clockify) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Error getting user home directory", err)
	}

	homeEnvPath := filepath.Join(homeDir, ".chronos", ".env")
	err = godotenv.Load(homeEnvPath)
	if err != nil {
		err = godotenv.Load(".env")
		if err != nil {
			log.Fatal("Error loading .env file: neither", homeEnvPath, "nor .env could be loaded:", err)
		}
	}

	projectId := os.Getenv("CLOCKIFY_DEFAULT_PROJECT")

	l := linear.NewLinear(&linear.LinearConfig{
		APIKey:  os.Getenv("LINEAR_API_KEY"),
		BaseURL: os.Getenv("LINEAR_BASE_URL"),
	})

	g := gitlab.NewGitlab(&gitlab.GitlabConfig{
		APIKey:  os.Getenv("GITLAB_ACCESS_TOKEN"),
		BaseURL: os.Getenv("GITLAB_BASE_URL"),
		UserID:  os.Getenv("GITLAB_USER_ID"),
	})

	cify := clockify.NewClockify(&clockify.ClockifyConfig{
		APIKey:      os.Getenv("CLOCKIFY_API_KEY"),
		BaseURL:     os.Getenv("CLOCKIFY_BASE_URL"),
		UserURL:     os.Getenv("CLOCKIFY_USER_URL"),
		WorkspaceID: os.Getenv("CLOCKIFY_WORKSPACE"),
		UserID:      os.Getenv("CLOCKIFY_USER_ID"),
	})

	return projectId, l, g, cify
}

func createCommands(projectId string, l *linear.Linear, g *gitlab.Gitlab, cify *clockify.Clockify) *cli.Command {
	return &cli.Command{
		Name:                  "chronos",
		Usage:                 "A simple CLI tool to log time entries to Clockify",
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			{
				Name:    "workspace",
				Aliases: []string{"ws"},
				Usage:   "Get clockify workspace ID",
				Action: func(context.Context, *cli.Command) error {
					userInfo, err := action.GetWorkspaceID(cify)
					if err != nil {
						return err
					}
					log.Print(userInfo)
					return nil
				},
			},
			{
				Name:      "log",
				Aliases:   []string{"l"},
				Usage:     "Log time entry to Clockify",
				UsageText: "chronos log <duration> <task>",
				Arguments: []cli.Argument{
					&cli.StringArg{
						Name: "duration",
					},
					&cli.StringArg{
						Name: "task",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.StringArg("duration") == "" || cmd.StringArg("task") == "" {
						return errors.New("both duration and task arguments are required")
					}
					duration, err := time.ParseDuration(cmd.StringArg("duration"))
					if err != nil {
						return err
					}
					task := cmd.StringArg("task")
					err = action.LogTime(cify, projectId, duration, task)
					if err != nil {
						return err
					}
					log.Printf("Logged %s for task: %s", duration, task)
					return nil
				},
			},
			{
				Name:    "report",
				Aliases: []string{"r"},
				Usage:   "Show a report of logged time entries",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					now := time.Now()
					currentYear, currentMonth, _ := now.Date()
					currentLocation := now.Location()

					firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, currentLocation)
					lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
					err := action.ShowReport(cify, l, g, projectId, firstOfMonth, lastOfMonth)
					if err != nil {
						return err
					}
					return nil
				},
			},
		},
	}
}
