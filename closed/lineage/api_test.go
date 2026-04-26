package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"
)

func TestBuildSubgraphRunToModelArtifact(t *testing.T) {
	now := time.Now().UTC()
	events := []lineageEvent{
		{
			EventID:     2,
			OccurredAt:  now,
			Actor:       "user-1",
			SubjectType: "experiment_run",
			SubjectID:   "run-1",
			Predicate:   "produced",
			ObjectType:  "model_version",
			ObjectID:    "mv-1",
		},
		{
			EventID:     1,
			OccurredAt:  now,
			Actor:       "user-1",
			SubjectType: "model_version",
			SubjectID:   "mv-1",
			Predicate:   "derived_from",
			ObjectType:  "artifact",
			ObjectID:    "art-1",
		},
	}

	api := &lineageAPI{
		edgesForNode: func(ctx context.Context, node lineageNode, limit int) ([]lineageEvent, error) {
			out := make([]lineageEvent, 0, len(events))
			for _, ev := range events {
				if (ev.SubjectType == node.Type && ev.SubjectID == node.ID) || (ev.ObjectType == node.Type && ev.ObjectID == node.ID) {
					out = append(out, ev)
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].EventID > out[j].EventID })
			if len(out) > limit {
				out = out[:limit]
			}
			return out, nil
		},
	}

	graph, err := api.buildSubgraph(context.Background(), lineageNode{Type: "experiment_run", ID: "run-1"}, 3, 10)
	if err != nil {
		t.Fatalf("buildSubgraph: %v", err)
	}

	if len(graph.Edges) != 2 {
		t.Fatalf("edges=%d, want 2", len(graph.Edges))
	}
	if !containsNode(graph.Nodes, lineageNode{Type: "experiment_run", ID: "run-1"}) {
		t.Fatalf("missing run node")
	}
	if !containsNode(graph.Nodes, lineageNode{Type: "model_version", ID: "mv-1"}) {
		t.Fatalf("missing model version node")
	}
	if !containsNode(graph.Nodes, lineageNode{Type: "artifact", ID: "art-1"}) {
		t.Fatalf("missing artifact node")
	}
}

func TestHandleRunSubgraph(t *testing.T) {
	api := lineageAPIWithStubEdges()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/runs/run-1", nil)
	req.SetPathValue("run_id", "run-1")

	api.handleRunSubgraph(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", w.Code)
	}
	var resp subgraphResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Root.Type != "experiment_run" || resp.Root.ID != "run-1" {
		t.Fatalf("unexpected root: %+v", resp.Root)
	}
}

func TestHandleModelVersionSubgraph(t *testing.T) {
	api := lineageAPIWithStubEdges()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/model-versions/mv-1", nil)
	req.SetPathValue("model_version_id", "mv-1")

	api.handleModelVersionSubgraph(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", w.Code)
	}
	var resp subgraphResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Root.Type != "model_version" || resp.Root.ID != "mv-1" {
		t.Fatalf("unexpected root: %+v", resp.Root)
	}
}

func lineageAPIWithStubEdges() *lineageAPI {
	events := []lineageEvent{
		{
			EventID:     2,
			OccurredAt:  time.Now().UTC(),
			Actor:       "user-1",
			SubjectType: "experiment_run",
			SubjectID:   "run-1",
			Predicate:   "produced",
			ObjectType:  "model_version",
			ObjectID:    "mv-1",
		},
		{
			EventID:     1,
			OccurredAt:  time.Now().UTC(),
			Actor:       "user-1",
			SubjectType: "model_version",
			SubjectID:   "mv-1",
			Predicate:   "derived_from",
			ObjectType:  "artifact",
			ObjectID:    "art-1",
		},
	}
	return &lineageAPI{
		edgesForNode: func(ctx context.Context, node lineageNode, limit int) ([]lineageEvent, error) {
			out := make([]lineageEvent, 0, len(events))
			for _, ev := range events {
				if (ev.SubjectType == node.Type && ev.SubjectID == node.ID) || (ev.ObjectType == node.Type && ev.ObjectID == node.ID) {
					out = append(out, ev)
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].EventID > out[j].EventID })
			if len(out) > limit {
				out = out[:limit]
			}
			return out, nil
		},
	}
}

func containsNode(nodes []lineageNode, needle lineageNode) bool {
	for _, node := range nodes {
		if node.Type == needle.Type && node.ID == needle.ID {
			return true
		}
	}
	return false
}
