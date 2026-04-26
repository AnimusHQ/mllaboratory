package main

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
	"github.com/animus-labs/animus-go/closed/internal/execution/specvalidator"
)

type runSpecPayload struct {
	RunSpecVersion  string                `json:"runSpecVersion"`
	ProjectID       string                `json:"projectId"`
	PipelineSpec    json.RawMessage       `json:"pipelineSpec"`
	DatasetBindings map[string]string     `json:"datasetBindings"`
	CodeRef         domain.CodeRef        `json:"codeRef"`
	EnvLock         domain.EnvLock        `json:"envLock"`
	Parameters      map[string]any        `json:"parameters"`
	PolicySnapshot  domain.PolicySnapshot `json:"policySnapshot"`
	CreatedAt       time.Time             `json:"createdAt"`
	CreatedBy       string                `json:"createdBy,omitempty"`
}

func parseRunSpec(raw json.RawMessage) (domain.RunSpec, error) {
	if len(raw) == 0 {
		return domain.RunSpec{}, errors.New("run spec is required")
	}
	var payload runSpecPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return domain.RunSpec{}, err
	}
	if len(payload.PipelineSpec) == 0 {
		return domain.RunSpec{}, errors.New("pipeline spec is required")
	}
	var pipeline domain.PipelineSpec
	if err := json.Unmarshal(payload.PipelineSpec, &pipeline); err != nil {
		return domain.RunSpec{}, err
	}
	if err := pipeline.ValidateBasicShape(); err != nil {
		return domain.RunSpec{}, err
	}

	params := payload.Parameters
	if params == nil {
		params = map[string]any{}
	}
	bindings := payload.DatasetBindings
	if bindings == nil {
		bindings = map[string]string{}
	}

	spec := domain.RunSpec{
		RunSpecVersion:  strings.TrimSpace(payload.RunSpecVersion),
		ProjectID:       strings.TrimSpace(payload.ProjectID),
		PipelineSpec:    pipeline,
		DatasetBindings: bindings,
		CodeRef:         payload.CodeRef,
		EnvLock:         payload.EnvLock,
		Parameters:      domain.Metadata(params),
		PolicySnapshot:  payload.PolicySnapshot,
		CreatedAt:       payload.CreatedAt,
		CreatedBy:       strings.TrimSpace(payload.CreatedBy),
	}
	if err := specvalidator.ValidateRunSpec(spec); err != nil {
		return domain.RunSpec{}, err
	}
	return spec, nil
}
