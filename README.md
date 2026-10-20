# my-tutor-agent

A simple CLI chat tutor ("Teddy") in Go, built on the OpenAI-compatible chat completions API.

![screenshot](v1.jpg)

## Requirements

- Go 1.26+
- An OpenAI-compatible endpoint (e.g. a local server exposing `/v1`)

## Usage

```bash
go run .
```

Then chat with Teddy in the terminal (`ctrl-c` to quit).

Configure the endpoint URL, API key, and model in `main.go` (see `openai.NewClient` and the `Model` field).
