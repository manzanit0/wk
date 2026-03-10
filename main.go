package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func main() {
	todoCmd := &cobra.Command{
		Use:   "todo",
		Short: "List pending pull requests",
		Long:  "List pending pull requests",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Opening pending PRs in browser...")

			params := []string{
				"is:open",
				"is:pr",
				"archived:false",
				"user:docker",
				"review-requested:Manzanit0",
				"draft:false",
			}

			fmt.Println("🖥  opening pending work in browser...")
			query := strings.Join(params, "+")
			_ = exec.Command("open", fmt.Sprintf("https://github.com/pulls?q=%s", query)).Run()
		},
	}

	prCmd := &cobra.Command{
		Use:   "pr",
		Short: "Create PR for current HEAD",
		Long:  "Create PR for current HEAD",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			var body string
			if noBody := cmd.Flag("no-body").Value.String(); noBody != "true" {
				fmt.Println("📖 reading commits...")
				var logArgs []string
				for _, base := range []string{"origin/HEAD", "origin/main", "origin/master"} {
					out, mergeErr := exec.Command("git", "merge-base", "HEAD", base).CombinedOutput()
					if mergeErr == nil {
						mergeBase := strings.TrimSpace(string(out))
						logArgs = []string{"log", mergeBase + "..HEAD", "--format=%s%n%b"}
						break
					}
				}
				if logArgs == nil {
					logArgs = []string{"log", "-10", "HEAD", "--format=%s%n%b"}
				}
				b, err := exec.Command("git", logArgs...).CombinedOutput()
				if err != nil {
					fmt.Println("💥 failed to read commits:", string(b))
					return
				}
				commits := strings.TrimSpace(string(b))

				fmt.Println("🤖 generating PR body...")
				prompt := fmt.Sprintf(`You are helping write a GitHub pull request body. Based on the following commit messages, write a concise PR body with two sections: "## WHAT" and "## WHY". Keep it under 10 lines total. Be direct and specific. No filler.

Commits:
%s`, commits)
				b, err = exec.Command("claude", "-p", prompt).CombinedOutput()
				if err != nil {
					fmt.Println("💥 failed to generate PR body:", string(b))
					return
				}
				body = strings.TrimSpace(string(b))

				tmp, err := os.CreateTemp("", "wk-pr-body-*.md")
				if err != nil {
					fmt.Println("💥 failed to create temp file:", err)
					return
				}
				defer os.Remove(tmp.Name())

				if _, err = tmp.WriteString(body); err != nil {
					fmt.Println("💥 failed to write temp file:", err)
					return
				}
				tmp.Close()

				editor := os.Getenv("EDITOR")
				if editor == "" {
					editor = "vi"
				}
				editorCmd := exec.Command(editor, tmp.Name())
				editorCmd.Stdin = os.Stdin
				editorCmd.Stdout = os.Stdout
				editorCmd.Stderr = os.Stderr
				if err = editorCmd.Run(); err != nil {
					fmt.Println("💥 editor exited with error:", err)
					return
				}

				edited, err := os.ReadFile(tmp.Name())
				if err != nil {
					fmt.Println("💥 failed to read edited body:", err)
					return
				}
				body = strings.TrimSpace(string(edited))
			}

			fmt.Println("🪄 pushing to remote...")
			b, err := exec.Command("git", "push", "-u", "origin", "HEAD").CombinedOutput()
			if err != nil {
				fmt.Println("💥 failed to push HEAD to remote:", string(b))
				return
			}

			fmt.Println("🗞  creating pull request...")
			command := []string{"pr", "create", "--fill", "--assignee", "Manzanit0"}
			if body != "" {
				command = append(command, "--body", body)
			}
			if isDraft := cmd.Flag("draft").Value.String(); isDraft == "true" {
				command = append(command, "--draft")
			}
			if title := cmd.Flag("title").Value.String(); title != "" {
				command = append(command, "--title", title)
			}

			b, err = exec.Command("gh", command...).CombinedOutput()
			if err != nil {
				fmt.Println("💥 failed to create pull request:", string(b))
				return
			}

			if cmd.Flag("open").Value.String() == "false" {
				return
			}

			fmt.Println("🖥  opening pull request in browser...")
			b, err = exec.Command("gh", "pr", "view", "--web").CombinedOutput()
			if err != nil {
				fmt.Println("💥 failed to open pull request in browser:", string(b))
				return
			}

			fmt.Println(string(b))
		},
	}

	prCmd.PersistentFlags().Bool("draft", false, "The PR will be created as draft")
	prCmd.PersistentFlags().Bool("no-body", false, "Skip LLM-generated PR body")
	prCmd.PersistentFlags().Bool("open", false, "The PR will be opened in the browser")
	prCmd.PersistentFlags().String("title", "", "Title to give to the PR")

	rootCmd := &cobra.Command{Use: "wk"}
	rootCmd.AddCommand(prCmd, todoCmd)
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("command failed: %s", err.Error())
	}
}
