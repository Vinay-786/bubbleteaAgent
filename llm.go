package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/ai"
	"github.com/cloudflare/cloudflare-go/v4/option"
	"github.com/joho/godotenv"
)

func callAgent(msg string, m []ai.AIRunParamsBodyTextGenerationMessage) (*ai.AIRunResponseUnion, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	secretToken := os.Getenv("CLOUDFLARE_API_TOKEN")

	client := cloudflare.NewClient(
		option.WithAPIToken(secretToken), // defaults to os.LookupEnv("CLOUDFLARE_API_TOKEN")
	)

	agent := NewAgent(client, msg)
	res, err := agent.Run(context.TODO(), m)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func NewAgent(client *cloudflare.Client, userMessage string) *Agent {
	return &Agent{
		client:      client,
		userMessage: userMessage,
	}
}

type Agent struct {
	client      *cloudflare.Client
	userMessage string
}

func (a *Agent) Run(ctx context.Context, conversation []ai.AIRunParamsBodyTextGenerationMessage) (*ai.AIRunResponseUnion, error) {
	for {
		userInput := a.userMessage

		userMessage := ai.AIRunParamsBodyTextGenerationMessage{
			Role:    cloudflare.String("user"),    // The role should be "user" for user input
			Content: cloudflare.String(userInput), // The content is the actual message from the user
		}

		conversation = append(conversation, userMessage)

		message, err := a.runInference(ctx, conversation)
		if err != nil {
			return nil, err
		}

		return message, nil

	}
}

func (a *Agent) runInference(ctx context.Context, data []ai.AIRunParamsBodyTextGenerationMessage) (*ai.AIRunResponseUnion, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	ACC_ID := os.Getenv("CLOUDFLARE_ACC_ID")
	message, err := a.client.AI.Run(
		ctx, "@cf/meta/llama-3.1-8b-instruct-fast", ai.AIRunParams{
			AccountID: cloudflare.F(ACC_ID),
			Body: ai.AIRunParamsBodyTextGeneration{
				Messages: cloudflare.F(data),
			},
		},
	)
	return message, err
}

// type Usage struct {
// 	CompletionTokens int `json:"completion_tokens"`
// 	PromptTokens     int `json:"prompt_tokens"`
// 	TotalTokens      int `json:"total_tokens"`
// }

// type ServerResponse struct {
// 	Response  string `json:"response"`
// 	ToolCalls []any  `json:"tool_calls"` // You can replace interface{} with a specific struct if you know the type
// 	Usage     Usage  `json:"usage"`
// }
