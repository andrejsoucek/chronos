package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/andrejsoucek/chronos/internal/action"
	"github.com/andrejsoucek/chronos/pkg"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v3"
)

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Error getting user home directory", err)
	}
	err = godotenv.Load(filepath.Join(homeDir, ".chronos", ".env"))
	if err != nil {
		log.Fatal("Error loading .env file", err)
	}

	projectId := os.Getenv("CLOCKIFY_DEFAULT_PROJECT")

	clockify := pkg.NewClockify(pkg.ClockifyConfig{
		APIKey:    os.Getenv("CLOCKIFY_API_KEY"),
		BaseURL:   os.Getenv("CLOCKIFY_BASE_URL"),
		UserURL:   os.Getenv("CLOCKIFY_USER_URL"),
		Workspace: os.Getenv("CLOCKIFY_WORKSPACE"),
	})

	cmd := &cli.Command{
		Name:                  "chronos",
		Usage:                 "A simple CLI tool to log time entries to Clockify",
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			{
				Name:    "workspace",
				Aliases: []string{"ws"},
				Usage:   "Get clockify workspace ID",
				Action: func(context.Context, *cli.Command) error {
					userInfo, err := action.GetWorkspaceID(clockify)
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
					err = action.LogTime(clockify, projectId, duration, task)
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
					// Placeholder for future report functionality
					log.Println("Report functionality is not yet implemented.")
					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
