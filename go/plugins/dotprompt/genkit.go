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

package dotprompt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/tracing"
)

// PromptRequest is a request to execute a dotprompt template and
// pass the result to a [Model].
type PromptRequest struct {
	// Input fields for the prompt. If not nil this should be a struct
	// or pointer to a struct that matches the prompt's input schema.
	Variables any `json:"variables,omitempty"`
	// Number of candidates to return; if 0, will be taken
	// from the prompt config; if still 0, will use 1.
	Candidates int `json:"candidates,omitempty"`
	// Model configuration. If nil will be taken from the prompt config.
	Config *ai.GenerationCommonConfig `json:"config,omitempty"`
	// Context to pass to model, if any.
	Context []any `json:"context,omitempty"`
	// The model to use. This overrides any model specified by the prompt.
	Model string `json:"model,omitempty"`
}

// buildVariables returns a map holding prompt field values based
// on a struct or a pointer to a struct. The struct value should have
// JSON tags that correspond to the Prompt's input schema.
// Only exported fields of the struct will be used.
func (p *Prompt) buildVariables(variables any) (map[string]any, error) {
	if variables == nil {
		return nil, nil
	}

	v := reflect.Indirect(reflect.ValueOf(variables))
	if v.Kind() == reflect.Map {
		return variables.(map[string]any), nil
	}
	if v.Kind() != reflect.Struct {
		return nil, errors.New("dotprompt: fields not a struct or pointer to a struct or a map")
	}
	vt := v.Type()

	// TODO: Verify the struct with p.Config.InputSchema.

	m := make(map[string]any)

fieldLoop:
	for i := 0; i < vt.NumField(); i++ {
		ft := vt.Field(i)
		if ft.PkgPath != "" {
			continue
		}

		jsonTag := ft.Tag.Get("json")
		jsonName, rest, _ := strings.Cut(jsonTag, ",")
		if jsonName == "" {
			jsonName = ft.Name
		}

		vf := v.Field(i)

		// If the field is the zero value, and omitempty is set,
		// don't pass it as a prompt input variable.
		if vf.IsZero() {
			for rest != "" {
				var key string
				key, rest, _ = strings.Cut(rest, ",")
				if key == "omitempty" {
					continue fieldLoop
				}
			}
		}

		m[jsonName] = vf.Interface()
	}

	return m, nil
}

// buildRequest prepares an [ai.ModelRequest] based on the prompt,
// using the input variables and other information in the [ai.PromptRequest].
func (p *Prompt) buildRequest(ctx context.Context, input any) (*ai.ModelRequest, error) {
	req := &ai.ModelRequest{}

	m, err := p.buildVariables(input)
	if err != nil {
		return nil, err
	}
	if req.Messages, err = p.RenderMessages(m); err != nil {
		return nil, err
	}

	req.Config = p.GenerationConfig

	var outputSchema map[string]any
	if p.OutputSchema != nil {
		jsonBytes, err := p.OutputSchema.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal output schema JSON: %w", err)
		}
		err = json.Unmarshal(jsonBytes, &outputSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal output schema JSON: %w", err)
		}
	}

	req.Output = &ai.ModelRequestOutput{
		Format: p.OutputFormat,
		Schema: outputSchema,
	}

	var tds []*ai.ToolDefinition
	for _, t := range p.Tools {
		tds = append(tds, t.Definition())
	}
	req.Tools = tds

	return req, nil
}

// Register registers an action to render a prompt.
func (p *Prompt) Register() error {
	if p.prompt != nil {
		return nil
	}

	name := p.Name
	if name == "" {
		return errors.New("attempt to register unnamed prompt")
	}
	if p.Variant != "" {
		name += "." + p.Variant
	}

	// TODO: Undo clearing of the Version once Monaco Editor supports newer than JSON schema draft-07.
	p.InputSchema.Version = ""

	metadata := map[string]any{
		"prompt": map[string]any{
			"name":     p.Name,
			"input":    map[string]any{"schema": p.InputSchema},
			"output":   map[string]any{"format": p.OutputFormat},
			"template": p.TemplateText,
		},
	}
	p.prompt = ai.DefinePrompt("dotprompt", name, metadata, p.Config.InputSchema, p.buildRequest)

	return nil
}

// Generate executes a prompt. It does variable substitution and
// passes the rendered template to the AI model specified by
// the prompt.
//
// This implements the [ai.Prompt] interface.
func (p *Prompt) Generate(ctx context.Context, pr *PromptRequest, cb func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	tracing.SetCustomMetadataAttr(ctx, "subtype", "prompt")

	var genReq *ai.ModelRequest
	var err error
	if p.prompt != nil {
		genReq, err = p.prompt.Render(ctx, pr.Variables)
	} else {
		genReq, err = p.buildRequest(ctx, pr.Variables)
	}
	if err != nil {
		return nil, err
	}

	// Let some fields in pr override those in the prompt config.
	if pr.Config != nil {
		genReq.Config = pr.Config
	}
	if len(pr.Context) > 0 {
		genReq.Context = pr.Context
	}

	model := p.Model
	if model == nil {
		modelName := p.ModelName
		if pr.Model != "" {
			modelName = pr.Model
		}
		if modelName == "" {
			return nil, errors.New("dotprompt execution: model not specified")
		}
		provider, name, found := strings.Cut(modelName, "/")
		if !found {
			return nil, errors.New("dotprompt model not in provider/name format")
		}

		model = ai.LookupModel(provider, name)
		if model == nil {
			return nil, fmt.Errorf("no model named %q for provider %q", name, provider)
		}
	}

	resp, err := model.Generate(ctx, genReq, cb)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
