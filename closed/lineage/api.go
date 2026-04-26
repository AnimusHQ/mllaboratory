package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type lineageAPI struct {
	logger       *slog.Logger
	db           *sql.DB
	edgesForNode func(ctx context.Context, node lineageNode, limit int) ([]lineageEvent, error)
}

func newLineageAPI(logger *slog.Logger, db *sql.DB) *lineageAPI {
	api := &lineageAPI{
		logger: logger,
		db:     db,
	}
	api.edgesForNode = api.queryEdgesForNode
	return api
}

func (api *lineageAPI) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /events", api.handleListEvents)

	mux.HandleFunc("GET /subgraphs/datasets/{dataset_id}", api.handleDatasetSubgraph)
	mux.HandleFunc("GET /subgraphs/dataset-versions/{version_id}", api.handleDatasetVersionSubgraph)
	mux.HandleFunc("GET /subgraphs/experiment-runs/{run_id}", api.handleRunSubgraph)
	mux.HandleFunc("GET /subgraphs/git-commits/{commit}", api.handleCommitSubgraph)
	mux.HandleFunc("GET /runs/{run_id}", api.handleRunSubgraph)
	mux.HandleFunc("GET /model-versions/{model_version_id}", api.handleModelVersionSubgraph)
}

type lineageEvent struct {
	EventID     int64           `json:"event_id"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Actor       string          `json:"actor"`
	RequestID   string          `json:"request_id,omitempty"`
	SubjectType string          `json:"subject_type"`
	SubjectID   string          `json:"subject_id"`
	Predicate   string          `json:"predicate"`
	ObjectType  string          `json:"object_type"`
	ObjectID    string          `json:"object_id"`
	Metadata    json.RawMessage `json:"metadata"`
}

type lineageNode struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func (api *lineageAPI) handleListEvents(w http.ResponseWriter, r *http.Request) {
	limit := clampInt(parseIntQuery(r, "limit", 100), 1, 500)
	beforeID := parseInt64Query(r, "before_event_id", 0)

	subjectType := strings.TrimSpace(r.URL.Query().Get("subject_type"))
	subjectID := strings.TrimSpace(r.URL.Query().Get("subject_id"))
	objectType := strings.TrimSpace(r.URL.Query().Get("object_type"))
	objectID := strings.TrimSpace(r.URL.Query().Get("object_id"))
	predicate := strings.TrimSpace(r.URL.Query().Get("predicate"))

	where := make([]string, 0, 6)
	args := make([]any, 0, 8)

	if beforeID > 0 {
		args = append(args, beforeID)
		where = append(where, "event_id < $"+strconv.Itoa(len(args)))
	}
	if subjectType != "" {
		args = append(args, subjectType)
		where = append(where, "subject_type = $"+strconv.Itoa(len(args)))
	}
	if subjectID != "" {
		args = append(args, subjectID)
		where = append(where, "subject_id = $"+strconv.Itoa(len(args)))
	}
	if objectType != "" {
		args = append(args, objectType)
		where = append(where, "object_type = $"+strconv.Itoa(len(args)))
	}
	if objectID != "" {
		args = append(args, objectID)
		where = append(where, "object_id = $"+strconv.Itoa(len(args)))
	}
	if predicate != "" {
		args = append(args, predicate)
		where = append(where, "predicate = $"+strconv.Itoa(len(args)))
	}

	args = append(args, limit)
	query := `SELECT event_id, occurred_at, actor, request_id, subject_type, subject_id, predicate, object_type, object_id, metadata
		FROM lineage_events`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY event_id DESC LIMIT $" + strconv.Itoa(len(args))

	rows, err := api.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	defer rows.Close()

	events := make([]lineageEvent, 0, limit)
	for rows.Next() {
		var (
			ev          lineageEvent
			requestID   sql.NullString
			metadataRaw []byte
		)
		if err := rows.Scan(&ev.EventID, &ev.OccurredAt, &ev.Actor, &requestID, &ev.SubjectType, &ev.SubjectID, &ev.Predicate, &ev.ObjectType, &ev.ObjectID, &metadataRaw); err != nil {
			api.writeError(w, r, http.StatusInternalServerError, "internal_error")
			return
		}
		ev.RequestID = strings.TrimSpace(requestID.String)
		ev.Metadata = normalizeJSON(metadataRaw)
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}

	resp := map[string]any{"events": events}
	if len(events) > 0 {
		resp["next_before_event_id"] = events[len(events)-1].EventID
	}
	api.writeJSON(w, http.StatusOK, resp)
}

func (api *lineageAPI) handleDatasetSubgraph(w http.ResponseWriter, r *http.Request) {
	datasetID := strings.TrimSpace(r.PathValue("dataset_id"))
	if datasetID == "" {
		api.writeError(w, r, http.StatusBadRequest, "dataset_id_required")
		return
	}
	api.handleSubgraph(w, r, lineageNode{Type: "dataset", ID: datasetID})
}

func (api *lineageAPI) handleDatasetVersionSubgraph(w http.ResponseWriter, r *http.Request) {
	versionID := strings.TrimSpace(r.PathValue("version_id"))
	if versionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "version_id_required")
		return
	}
	api.handleSubgraph(w, r, lineageNode{Type: "dataset_version", ID: versionID})
}

func (api *lineageAPI) handleRunSubgraph(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimSpace(r.PathValue("run_id"))
	if runID == "" {
		api.writeError(w, r, http.StatusBadRequest, "run_id_required")
		return
	}
	api.handleSubgraph(w, r, lineageNode{Type: "experiment_run", ID: runID})
}

func (api *lineageAPI) handleModelVersionSubgraph(w http.ResponseWriter, r *http.Request) {
	versionID := strings.TrimSpace(r.PathValue("model_version_id"))
	if versionID == "" {
		api.writeError(w, r, http.StatusBadRequest, "model_version_id_required")
		return
	}
	api.handleSubgraph(w, r, lineageNode{Type: "model_version", ID: versionID})
}

func (api *lineageAPI) handleCommitSubgraph(w http.ResponseWriter, r *http.Request) {
	commit := strings.TrimSpace(r.PathValue("commit"))
	if commit == "" {
		api.writeError(w, r, http.StatusBadRequest, "commit_required")
		return
	}
	api.handleSubgraph(w, r, lineageNode{Type: "git_commit", ID: commit})
}

func (api *lineageAPI) queryEdgesForNode(ctx context.Context, node lineageNode, limit int) ([]lineageEvent, error) {
	if api == nil || api.db == nil {
		return nil, errors.New("lineage store unavailable")
	}
	rows, err := api.db.QueryContext(
		ctx,
		`SELECT event_id, occurred_at, actor, request_id, subject_type, subject_id, predicate, object_type, object_id, metadata
			 FROM lineage_events
			 WHERE (subject_type = $1 AND subject_id = $2) OR (object_type = $1 AND object_id = $2)
			 ORDER BY event_id DESC
			 LIMIT $3`,
		strings.TrimSpace(node.Type),
		strings.TrimSpace(node.ID),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]lineageEvent, 0, limit)
	for rows.Next() {
		var (
			ev          lineageEvent
			requestID   sql.NullString
			metadataRaw []byte
		)
		if err := rows.Scan(&ev.EventID, &ev.OccurredAt, &ev.Actor, &requestID, &ev.SubjectType, &ev.SubjectID, &ev.Predicate, &ev.ObjectType, &ev.ObjectID, &metadataRaw); err != nil {
			return nil, err
		}
		ev.RequestID = strings.TrimSpace(requestID.String)
		ev.Metadata = normalizeJSON(metadataRaw)
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (api *lineageAPI) handleSubgraph(w http.ResponseWriter, r *http.Request, root lineageNode) {
	depth := clampInt(parseIntQuery(r, "depth", 3), 1, 5)
	maxEdges := clampInt(parseIntQuery(r, "max_edges", 2000), 1, 5000)

	graph, err := api.buildSubgraph(r.Context(), root, depth, maxEdges)
	if err != nil {
		api.writeError(w, r, http.StatusInternalServerError, "internal_error")
		return
	}
	api.writeJSON(w, http.StatusOK, graph)
}

type subgraphResponse struct {
	Root  lineageNode    `json:"root"`
	Nodes []lineageNode  `json:"nodes"`
	Edges []lineageEvent `json:"edges"`
}

type nodeKey struct {
	Type string
	ID   string
}

func (api *lineageAPI) buildSubgraph(ctx context.Context, root lineageNode, depth int, maxEdges int) (subgraphResponse, error) {
	rootKey := nodeKey{Type: strings.TrimSpace(root.Type), ID: strings.TrimSpace(root.ID)}
	if rootKey.Type == "" || rootKey.ID == "" {
		return subgraphResponse{}, errors.New("root is required")
	}

	type queueItem struct {
		Node  nodeKey
		Depth int
	}

	nodes := map[nodeKey]struct{}{rootKey: {}}
	visited := map[nodeKey]struct{}{rootKey: {}}
	edgesByID := make(map[int64]struct{})
	edges := make([]lineageEvent, 0, 64)

	queue := []queueItem{{Node: rootKey, Depth: 0}}
	for len(queue) > 0 && len(edges) < maxEdges {
		item := queue[0]
		queue = queue[1:]

		if item.Depth >= depth {
			continue
		}

		remaining := maxEdges - len(edges)
		perNodeLimit := remaining
		if perNodeLimit > 500 {
			perNodeLimit = 500
		}

		if api.edgesForNode == nil {
			return subgraphResponse{}, errors.New("lineage edges provider not initialized")
		}
		nodeEdges, err := api.edgesForNode(ctx, lineageNode{Type: item.Node.Type, ID: item.Node.ID}, perNodeLimit)
		if err != nil {
			return subgraphResponse{}, err
		}
		for _, ev := range nodeEdges {
			if _, ok := edgesByID[ev.EventID]; ok {
				continue
			}
			edgesByID[ev.EventID] = struct{}{}
			edges = append(edges, ev)

			subj := nodeKey{Type: strings.TrimSpace(ev.SubjectType), ID: strings.TrimSpace(ev.SubjectID)}
			obj := nodeKey{Type: strings.TrimSpace(ev.ObjectType), ID: strings.TrimSpace(ev.ObjectID)}

			if subj.Type != "" && subj.ID != "" {
				nodes[subj] = struct{}{}
				if _, ok := visited[subj]; !ok {
					visited[subj] = struct{}{}
					queue = append(queue, queueItem{Node: subj, Depth: item.Depth + 1})
				}
			}
			if obj.Type != "" && obj.ID != "" {
				nodes[obj] = struct{}{}
				if _, ok := visited[obj]; !ok {
					visited[obj] = struct{}{}
					queue = append(queue, queueItem{Node: obj, Depth: item.Depth + 1})
				}
			}

			if len(edges) >= maxEdges {
				break
			}
		}
	}

	nodeList := make([]lineageNode, 0, len(nodes))
	for nk := range nodes {
		nodeList = append(nodeList, lineageNode{Type: nk.Type, ID: nk.ID})
	}
	sort.Slice(nodeList, func(i, j int) bool {
		if nodeList[i].Type == nodeList[j].Type {
			return nodeList[i].ID < nodeList[j].ID
		}
		return nodeList[i].Type < nodeList[j].Type
	})
	sort.Slice(edges, func(i, j int) bool { return edges[i].EventID > edges[j].EventID })

	return subgraphResponse{
		Root:  root,
		Nodes: nodeList,
		Edges: edges,
	}, nil
}

func (api *lineageAPI) writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(body)
}

func (api *lineageAPI) writeError(w http.ResponseWriter, r *http.Request, status int, code string) {
	api.writeJSON(w, status, map[string]any{
		"error":      code,
		"request_id": r.Header.Get("X-Request-Id"),
	})
}

func normalizeJSON(raw []byte) json.RawMessage {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return []byte("{}")
	}
	return raw
}

func bytesTrimSpace(in []byte) []byte {
	return []byte(strings.TrimSpace(string(in)))
}

func parseIntQuery(r *http.Request, key string, def int) int {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return def
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return parsed
}

func parseInt64Query(r *http.Request, key string, def int64) int64 {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return def
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return parsed
}

func clampInt(v int, min int, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
