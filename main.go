package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type Agent struct {
	client         openai.Client
	getUserMessage func() (string, bool)
	messages       []openai.ChatCompletionMessageParamUnion
}

func NewAgent(client openai.Client, getUserMessage func() (string, bool)) *Agent {
	return &Agent{
		client:         client,
		getUserMessage: getUserMessage,
		messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(
				"You are Teddy, a helpful and patient tutor.",
			),
		}}
}

func (a *Agent) runInference(ctx context.Context, userInput string) (string, error) {
	a.messages = append(
		a.messages,
		openai.UserMessage(userInput),
	)
	response, err := a.client.Chat.Completions.New(
		ctx,
		openai.ChatCompletionNewParams{
			Model:    "Qwen3.8-27B-4bit",
			Messages: a.messages,
		},
	)
	if err != nil {
		return "", fmt.Errorf("chat completion: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("model returned no choices")
	}
	reply := response.Choices[0].Message.Content
	a.messages = append(
		a.messages,
		openai.AssistantMessage(reply),
	)
	return reply, nil
}

func (a *Agent) Run(ctx context.Context) error {
	fmt.Println("Chat with Teddy (use 'ctrl-c' to quit)")
	for {
		fmt.Print("\x1b[94mYou\x1b[0m: ")
		userInput, ok := a.getUserMessage()
		if !ok {
			break
		}
		reply, err := a.runInference(ctx, userInput)
		if err != nil {
			return err
		}
		fmt.Println("Teddy:", reply)
	}
	return nil
}

func main() {
	ctx := context.Background()
	// NOTE: Custom port/api example key change to your config&rotate
	client := openai.NewClient(option.WithBaseURL("http://127.0.0.1:8013/v1"), option.WithAPIKey("1133"))
	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}
	agent := NewAgent(client, getUserMessage)
	err := agent.Run(ctx)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}
