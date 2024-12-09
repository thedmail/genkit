// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snippets

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	// [START import]
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/plugins/vertexai"
	// [END import]
)

// Globals for simplification only.
// Bad style: don't do this.
var ctx = context.Background()
var gemini15pro ai.Model

func m1() error {
	// [START init]
	// Default to the value of GCLOUD_PROJECT for the project,
	// and "us-central1" for the location.
	// To specify these values directly, pass a vertexai.Config value to Init.
	if err := vertexai.Init(ctx, nil); err != nil {
		return err
	}
	// [END init]

	// [START model]
	model := vertexai.Model("gemini-1.5-flash")
	// [END model]

	// [START call]
	responseText, err := ai.GenerateText(ctx, model, ai.WithTextPrompt("Tell me a joke."))
	if err != nil {
		return err
	}
	fmt.Println(responseText)
	// [END call]
	return nil
}

func opts() error {
	model := vertexai.Model("gemini-1.5-flash")

	// [START options]
	response, err := ai.Generate(ctx, model,
		ai.WithTextPrompt("Tell me a joke about dogs."),
		ai.WithConfig(ai.GenerationCommonConfig{
			Temperature:     1.67,
			StopSequences:   []string{"cat"},
			MaxOutputTokens: 3,
		}))
	// [END options]

	_ = response
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func streaming() error {
	// [START streaming]
	response, err := ai.Generate(ctx, gemini15pro,
		ai.WithTextPrompt("Tell a long story about robots and ninjas."),
		// stream callback
		ai.WithStreaming(
			func(ctx context.Context, grc *ai.ModelResponseChunk) error {
				fmt.Printf("Chunk: %s\n", grc.Text())
				return nil
			}))
	if err != nil {
		return err
	}

	// You can also still get the full response.
	fmt.Println(response.Text())

	// [END streaming]
	return nil
}

func multi() error {
	// [START multimodal]
	imageBytes, err := os.ReadFile("img.jpg")
	if err != nil {
		return err
	}
	encodedImage := base64.StdEncoding.EncodeToString(imageBytes)

	resp, err := ai.Generate(ctx, gemini15pro, ai.WithMessages(
		ai.NewUserMessage(
			ai.NewTextPart("Describe the following image."),
			ai.NewMediaPart("", "data:image/jpeg;base64,"+encodedImage))))
	// [END multimodal]
	if err != nil {
		return err
	}
	_ = resp
	return nil
}

func tools() error {
	// [START tools]
	myJokeTool := ai.DefineTool(
		"myJoke",
		"useful when you need a joke to tell",
		func(ctx context.Context, input *any) (string, error) {
			return "haha Just kidding no joke! got you", nil
		},
	)

	response, err := ai.Generate(ctx, gemini15pro,
		ai.WithTextPrompt("Tell me a joke."),
		ai.WithTools(myJokeTool))
	// [END tools]
	_ = response
	return err
}

func history() error {
	var prompt string
	// [START hist1]
	history := []*ai.Message{{
		Content: []*ai.Part{ai.NewTextPart(prompt)},
		Role:    ai.RoleUser,
	}}

	response, err := ai.Generate(context.Background(), gemini15pro, ai.WithMessages(history...))
	// [END hist1]
	_ = err
	// [START hist2]
	history = append(history, response.Message)
	// [END hist2]

	// [START hist3]
	history = append(history, &ai.Message{
		Content: []*ai.Part{ai.NewTextPart(prompt)},
		Role:    ai.RoleUser,
	})

	response, err = ai.Generate(ctx, gemini15pro, ai.WithMessages(history...))
	// [END hist3]
	// [START hist4]
	history = []*ai.Message{{
		Content: []*ai.Part{ai.NewTextPart("Talk like a pirate.")},
		Role:    ai.RoleSystem,
	}}
	// [END hist4]
	return nil
}
