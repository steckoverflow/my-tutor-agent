package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

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

type ListFilesInput struct {
	Path string `json:"path,omitempty" jsonschema_description:"Optional relative path to list files from. Defaults to current directory if not provided."`
}

var ListFilesInputSchema = GenerateSchema[ListFilesInput]()

var ListFilesDefinition = ToolDefinition{
	Name:        "list_files",
	Description: "List files and directories at a given path. If no path is provided, lists files in the current directory.",
	InputSchema: ListFilesInputSchema,
	Function:    ListFiles,
}

func ListFiles(input json.RawMessage) (string, error) {
	listFilesInput := ListFilesInput{}
	err := json.Unmarshal(input, &listFilesInput)
	if err != nil {
		panic(err)
	}
	dir := "."
	if listFilesInput.Path != "" {
		dir = listFilesInput.Path
	}
	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if relPath != "." {
			if info.IsDir() {
				files = append(files, relPath+"/")
			} else {
				files = append(files, relPath)
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	result, err := json.Marshal(files)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

var EditFileDefinition = ToolDefinition{
	Name: "edit_file",
	Description: `Make edits to a text file.

Replaces 'old_str' with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other.

If the file specified with path doesn't exist, it will be created.
`,
	InputSchema: EditFileInputSchema,
	Function:    EditFile,
}

type EditFileInput struct {
	Path   string `json:"path" jsonschema_description:"The path to the file"`
	OldStr string `json:"old_str" jsonschema_description:"Text to search for - must match exactly and must only have one match exactly"`
	NewStr string `json:"new_str" jsonschema_description:"Text to replace old_str with"`
}

var EditFileInputSchema = GenerateSchema[EditFileInput]()

func EditFile(input json.RawMessage) (string, error) {
	editFileInput := EditFileInput{}
	err := json.Unmarshal(input, &editFileInput)
	if err != nil {
		return "", err
	}
	if editFileInput.Path == "" || editFileInput.OldStr == editFileInput.NewStr {
		return "", fmt.Errorf("invalid input parameters")
	}
	content, err := os.ReadFile(editFileInput.Path)
	if err != nil {
		if os.IsNotExist(err) && editFileInput.OldStr == "" {
			return createNewFile(editFileInput.Path, editFileInput.NewStr)
		}
		return "", err
	}
	oldContent := string(content)
	newContent := strings.Replace(oldContent, editFileInput.OldStr, editFileInput.NewStr, -1)
	if oldContent == newContent && editFileInput.OldStr != "" {
		return "", fmt.Errorf("old_str not found in file")
	}
	err = os.WriteFile(editFileInput.Path, []byte(newContent), 0644)
	if err != nil {
		return "", err
	}
	return "OK", nil
}

func createNewFile(filePath, content string) (string, error) {
	dir := path.Dir(filePath)
	if dir != "." {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}
	}
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	return fmt.Sprintf("Successfully created file %s", filePath), nil
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

func (a *Agent) executeTool(id, name string, input json.RawMessage) {
	var def ToolDefinition
	found := false
	for _, t := range a.tools {
		if t.Name == name {
			def, found = t, true
			break
		}
	}
	if !found {
		a.messages = append(a.messages, openai.ToolMessage("tool not found", id))
		return
	}
	fmt.Printf("\x1b[92mtool\x1b[0m: %s(%s)\n", name, input)
	out, err := def.Function(input)
	if err != nil {
		out = "error: " + err.Error()
	}
	a.messages = append(a.messages, openai.ToolMessage(out, id))
}

func (a *Agent) runInference(ctx context.Context) (openai.ChatCompletionMessage, error) {
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
	response, err := a.client.Chat.Completions.New(
		ctx,
		openai.ChatCompletionNewParams{
			Model:    "Qwen3.8-27B-4bit",
			Messages: a.messages,
			Tools:    oiTools,
		},
	)
	if err != nil {
		return openai.ChatCompletionMessage{}, fmt.Errorf("chat completion: %w", err)
	}
	if len(response.Choices) == 0 {
		return openai.ChatCompletionMessage{}, fmt.Errorf("model returned no choices")
	}
	reply := response.Choices[0].Message
	a.messages = append(a.messages, reply.ToParam())
	return reply, nil
}

func (a *Agent) Run(ctx context.Context) error {
	fmt.Println("Chat with Teddy (use 'ctrl-c' to quit)")
	readUserInput := true
	for {
		if readUserInput {
			fmt.Print("\x1b[94mYou\x1b[0m: ")
			userInput, ok := a.getUserMessage()
			if !ok {
				break
			}
			a.messages = append(a.messages, openai.UserMessage(userInput))
		}
		reply, err := a.runInference(ctx)
		if err != nil {
			return err
		}
		if reply.Content != "" {
			fmt.Printf("\x1b[93mTeddy\x1b[0m: %s\n", reply.Content)
		}
		if len(reply.ToolCalls) == 0 {
			readUserInput = true
			continue
		}
		for _, tc := range reply.ToolCalls {
			a.executeTool(tc.ID, tc.Function.Name, json.RawMessage(tc.Function.Arguments))
		}
		readUserInput = false
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
	tools := []ToolDefinition{ReadFileDefinition, ListFilesDefinition, EditFileDefinition}
	agent := NewAgent(client, getUserMessage, tools)
	err := agent.Run(ctx)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}
