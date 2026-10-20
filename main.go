package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/conversations"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
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

func (a *Agent) runInference(ctx context.Context, conversation *conversations.Conversation, userMessage responses.ResponseNewParamsInputUnion) (*responses.Response, error) {
	response, err := a.client.Responses.New(ctx, responses.ResponseNewParams{
		Model: "Qwen3.8-27B-4bit",
		Input: userMessage,
		Conversation: responses.ResponseNewParamsConversationUnion{
			OfConversationObject: &responses.ResponseConversationParam{
				ID: conversation.ID,
			},
		},
	})
	if err != nil {
		panic(err)
	}
	return response, nil
}

func (a *Agent) Run(ctx context.Context) error {
	conversation, err := a.client.Conversations.New(ctx, conversations.ConversationNewParams{})
	if err != nil {
		panic(err)
	}
	fmt.Println("Created conversation:", conversation.ID)
	fmt.Println("Chat with Teddy (use 'ctrl-c' to quit)")
	for {
		fmt.Print("\\u001b[94mYou\\u001b[0m: ")
		userInput, ok := a.getUserMessage()
		if !ok {
			break
		}
		userMessage := responses.ResponseNewParamsInputUnion{
			OfString: openai.String(userInput),
		}
		message, err := a.runInference(ctx, conversation, userMessage)
		if err != nil {
			return err
		}
		fmt.Println("Teddy: ", message.OutputText())
		items, err := a.client.Conversations.Items.List(ctx, conversation.ID, conversations.ItemListParams{})
		if err != nil {
			return err
		}
		fmt.Println("Conversation has", len(items.Data), "items")
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
