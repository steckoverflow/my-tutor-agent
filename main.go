package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

func GenerateSchema[T any]() openai.FunctionParameters {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)

	b, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	var params openai.FunctionParameters
	if err := json.Unmarshal(b, &params); err != nil {
		panic(err)
	}
	return params
}

type ToolDefinition struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	InputSchema openai.FunctionParameters `json:"input_schema"`
	Function    func(input json.RawMessage) (string, error)
}

type ReadFileInput struct {
	Path string `json:"path" jsonschema_description:"The relative path of a file in the working directory."`
}

func ReadFile(input json.RawMessage) (string, error) {
	readFileInput := ReadFileInput{}
	err := json.Unmarshal(input, &readFileInput)
	if err != nil {
		panic(err)
	}
	content, err := os.ReadFile(readFileInput.Path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

var ReadFileInputSchema = GenerateSchema[ReadFileInput]()

var ReadFileDefinition = ToolDefinition{
	Name:        "read_file",
	Description: "Read the contents of a given relative file path. Use this when you want  to see what's inside a file. Do not use this with directory names.",
	InputSchema: ReadFileInputSchema,
	Function:    ReadFile,
}

type Agent struct {
	client         openai.Client
	getUserMessage func() (string, bool)
	messages       []openai.ChatCompletionMessageParamUnion
	tools          []ToolDefinition
}

func NewAgent(client openai.Client, getUserMessage func() (string, bool), tools []ToolDefinition) *Agent {
	return &Agent{
		client:         client,
		getUserMessage: getUserMessage,
		messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(
				"You are Teddy, a helpful and patient tutor.",
			),
		},
		tools: tools}
}

func (a *Agent) runInference(ctx context.Context, userInput string) (string, error) {
	oiTools := []openai.ChatCompletionToolUnionParam{}
	for _, tool := range a.tools {
		oiTools = append(oiTools, openai.ChatCompletionFunctionTool(
			shared.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  tool.InputSchema,
			},
		))
	}
	a.messages = append(
		a.messages,
		openai.UserMessage(userInput),
	)
	response, err := a.client.Chat.Completions.New(
		ctx,
		openai.ChatCompletionNewParams{
			Model:    "Qwen3.8-27B-4bit",
			Messages: a.messages,
			Tools:    oiTools,
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
	tools := []ToolDefinition{ReadFileDefinition}
	agent := NewAgent(client, getUserMessage, tools)
	err := agent.Run(ctx)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}
