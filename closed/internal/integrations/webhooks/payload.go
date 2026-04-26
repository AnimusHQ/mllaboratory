package webhooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func EventID(eventType EventType, projectID, subjectID string) (string, error) {
	if !eventType.Valid() {
		return "", fmt.Errorf("invalid event type")
	}
	projectID = strings.TrimSpace(projectID)
	subjectID = strings.TrimSpace(subjectID)
	if projectID == "" || subjectID == "" {
		return "", fmt.Errorf("project_id and subject_id are required")
	}
	seed := strings.Join([]string{eventType.String(), projectID, subjectID}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "wh-" + hex.EncodeToString(sum[:]), nil
}

func RunFinishedPayload(projectID, runID string, emittedAt time.Time) (Payload, error) {
	if strings.TrimSpace(runID) == "" {
		return Payload{}, fmt.Errorf("run_id is required")
	}
	if emittedAt.IsZero() {
		emittedAt = time.Now().UTC()
	}
	eventID, err := EventID(EventRunFinished, projectID, runID)
	if err != nil {
		return Payload{}, err
	}
	return Payload{
		EventID:   eventID,
		EventType: EventRunFinished,
		EmittedAt: emittedAt.UTC(),
		ProjectID: strings.TrimSpace(projectID),
		Subject:   SubjectRef{RunID: strings.TrimSpace(runID)},
		Links: map[string]string{
			"run": fmt.Sprintf("/projects/%s/runs/%s", strings.TrimSpace(projectID), strings.TrimSpace(runID)),
		},
	}, nil
}

func ModelApprovedPayload(projectID, modelVersionID string, emittedAt time.Time) (Payload, error) {
	if strings.TrimSpace(modelVersionID) == "" {
		return Payload{}, fmt.Errorf("model_version_id is required")
	}
	if emittedAt.IsZero() {
		emittedAt = time.Now().UTC()
	}
	eventID, err := EventID(EventModelApproved, projectID, modelVersionID)
	if err != nil {
		return Payload{}, err
	}
	return Payload{
		EventID:   eventID,
		EventType: EventModelApproved,
		EmittedAt: emittedAt.UTC(),
		ProjectID: strings.TrimSpace(projectID),
		Subject:   SubjectRef{ModelVersionID: strings.TrimSpace(modelVersionID)},
		Links: map[string]string{
			"model_version": fmt.Sprintf("/projects/%s/model-versions/%s", strings.TrimSpace(projectID), strings.TrimSpace(modelVersionID)),
		},
	}, nil
}

func DatasetVersionCreatedPayload(projectID, datasetVersionID string, emittedAt time.Time) (Payload, error) {
	if strings.TrimSpace(datasetVersionID) == "" {
		return Payload{}, fmt.Errorf("dataset_version_id is required")
	}
	if emittedAt.IsZero() {
		emittedAt = time.Now().UTC()
	}
	eventID, err := EventID(EventDatasetVersionCreated, projectID, datasetVersionID)
	if err != nil {
		return Payload{}, err
	}
	return Payload{
		EventID:   eventID,
		EventType: EventDatasetVersionCreated,
		EmittedAt: emittedAt.UTC(),
		ProjectID: strings.TrimSpace(projectID),
		Subject:   SubjectRef{DatasetVersionID: strings.TrimSpace(datasetVersionID)},
		Links: map[string]string{
			"dataset_version":  fmt.Sprintf("/dataset-versions/%s", strings.TrimSpace(datasetVersionID)),
			"dataset_download": fmt.Sprintf("/dataset-versions/%s/download", strings.TrimSpace(datasetVersionID)),
		},
	}, nil
}

func PayloadJSON(payload Payload) ([]byte, error) {
	return json.Marshal(payload)
}
