package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type executeRequest struct {
	RunID          string `json:"run_id"`
	StepName       string `json:"step_name"`
	Attempt        int    `json:"attempt"`
	Seed           string `json:"seed"`
	ArtifactBucket string `json:"artifact_bucket"`
	ArtifactPrefix string `json:"artifact_prefix"`
}

type artifactInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type executeResponse struct {
	Status            string         `json:"status"`
	LogsURI           string         `json:"logs_uri"`
	ProducedArtifacts []artifactInfo `json:"produced_artifacts"`
}

func main() {
	addr := strings.TrimSpace(os.Getenv("USERSPACE_HTTP_ADDR"))
	if addr == "" {
		addr = ":8090"
	}
	outputDir := strings.TrimSpace(os.Getenv("DEMO_OUTPUT_DIR"))
	if outputDir == "" {
		outputDir = "/outputs"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/execute-demo-step", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req executeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_json"})
			return
		}
		req.RunID = strings.TrimSpace(req.RunID)
		req.StepName = strings.TrimSpace(req.StepName)
		if req.RunID == "" || req.StepName == "" || req.Attempt <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
			return
		}
		seed := strings.TrimSpace(req.Seed)
		if seed == "" {
			seed = "demo-seed"
		}

		artifactDir := filepath.Join(outputDir, safeName(req.ArtifactPrefix), safeName(req.RunID), safeName(req.StepName))
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "write_failed"})
			return
		}

		hash := sha256.Sum256([]byte(seed + ":" + req.RunID + ":" + req.StepName + ":" + strconv.Itoa(req.Attempt)))
		payload := fmt.Sprintf("animus-demo\nrun_id=%s\nstep=%s\nattempt=%d\nseed=%s\nsha256=%s\n",
			req.RunID,
			req.StepName,
			req.Attempt,
			seed,
			hex.EncodeToString(hash[:]),
		)

		artifactPath := filepath.Join(artifactDir, "artifact.txt")
		if err := os.WriteFile(artifactPath, []byte(payload), 0o644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "write_failed"})
			return
		}

		logPath := filepath.Join(artifactDir, "logs.txt")
		if err := os.WriteFile(logPath, []byte("demo userspace execution\n"+payload), 0o644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "write_failed"})
			return
		}

		artifact, err := buildArtifactInfo(artifactPath, req.StepName+"-artifact")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "hash_failed"})
			return
		}

		resp := executeResponse{
			Status:            "succeeded",
			LogsURI:           "file://" + logPath,
			ProducedArtifacts: []artifactInfo{artifact},
		}
		writeJSON(w, http.StatusOK, resp)
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	log.Printf("userspace runner listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func buildArtifactInfo(path string, name string) (artifactInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return artifactInfo{}, err
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return artifactInfo{}, err
	}
	return artifactInfo{
		Name:   name,
		Path:   path,
		SHA256: hex.EncodeToString(h.Sum(nil)),
		Bytes:  size,
	}, nil
}

func safeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "demo"
	}
	value = strings.ReplaceAll(value, "..", "")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "\\", "_")
	return value
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
