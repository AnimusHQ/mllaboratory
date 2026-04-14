package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	policyPath := flag.String("policy", "deploy/policy/kyverno-signed-images.yaml", "Path to admission policy YAML")
	signedPath := flag.String("signed", "deploy/policy/samples/pod-signed.yaml", "Path to digest-pinned sample pod")
	unsignedPath := flag.String("unsigned", "deploy/policy/samples/pod-unsigned.yaml", "Path to tag-based sample pod")
	flag.Parse()

	if err := validatePolicy(*policyPath); err != nil {
		fmt.Fprintf(os.Stderr, "policy-validate: %v\n", err)
		os.Exit(1)
	}
	if err := validateSamplePod(*signedPath, true); err != nil {
		fmt.Fprintf(os.Stderr, "policy-validate: %v\n", err)
		os.Exit(1)
	}
	if err := validateSamplePod(*unsignedPath, false); err != nil {
		fmt.Fprintf(os.Stderr, "policy-validate: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("policy-validate: ok")
}

func validatePolicy(path string) error {
	docs, err := decodeYAMLDocs(path)
	if err != nil {
		return fmt.Errorf("decode policy yaml: %w", err)
	}
	if len(docs) == 0 {
		return fmt.Errorf("policy file has no YAML documents: %s", path)
	}

	doc := docs[0]
	if asString(doc["kind"]) != "ClusterPolicy" {
		return fmt.Errorf("policy kind must be ClusterPolicy")
	}
	if !strings.HasPrefix(asString(doc["apiVersion"]), "kyverno.io/") {
		return fmt.Errorf("policy apiVersion must start with kyverno.io/")
	}

	spec, ok := asMap(doc["spec"])
	if !ok {
		return fmt.Errorf("policy spec missing")
	}
	rules, ok := asSlice(spec["rules"])
	if !ok || len(rules) == 0 {
		return fmt.Errorf("policy rules missing")
	}

	hasDigestRule := false
	hasVerifyRule := false
	for _, ruleAny := range rules {
		rule, ok := asMap(ruleAny)
		if !ok {
			continue
		}
		if validateRule, ok := asMap(rule["validate"]); ok {
			if pattern, ok := asMap(validateRule["pattern"]); ok && hasStringRecursive(pattern, "@sha256:") {
				hasDigestRule = true
			}
		}
		if verifyEntries, ok := asSlice(rule["verifyImages"]); ok && len(verifyEntries) > 0 {
			hasVerifyRule = true
		}
	}

	if !hasDigestRule {
		return fmt.Errorf("policy missing digest pin enforcement rule")
	}
	if !hasVerifyRule {
		return fmt.Errorf("policy missing verifyImages signature rule")
	}
	if !hasStringRecursive(doc, "https://token.actions.githubusercontent.com") {
		return fmt.Errorf("policy missing OIDC issuer constraint")
	}
	if !hasStringRecursive(doc, ".github/workflows/release-images.yml") {
		return fmt.Errorf("policy missing workflow subject constraint")
	}

	return nil
}

type podManifest struct {
	Spec struct {
		Containers         []containerSpec `yaml:"containers"`
		InitContainers     []containerSpec `yaml:"initContainers"`
		EphemeralContainer []containerSpec `yaml:"ephemeralContainers"`
	} `yaml:"spec"`
}

type containerSpec struct {
	Image string `yaml:"image"`
}

func validateSamplePod(path string, expectDigest bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read sample manifest: %s: %w", path, err)
	}
	var pod podManifest
	if err := yaml.Unmarshal(data, &pod); err != nil {
		return fmt.Errorf("decode sample manifest: %s: %w", path, err)
	}

	images := collectImages(pod)
	if len(images) == 0 {
		return fmt.Errorf("sample manifest has no container images: %s", path)
	}

	hasDigest := true
	for _, image := range images {
		if !strings.Contains(image, "@sha256:") {
			hasDigest = false
			break
		}
	}

	if expectDigest && !hasDigest {
		return fmt.Errorf("signed sample must use digest-pinned images: %s", path)
	}
	if !expectDigest && hasDigest {
		return fmt.Errorf("unsigned sample must include at least one non-digest image: %s", path)
	}
	return nil
}

func collectImages(pod podManifest) []string {
	images := make([]string, 0, len(pod.Spec.Containers)+len(pod.Spec.InitContainers)+len(pod.Spec.EphemeralContainer))
	for _, c := range pod.Spec.Containers {
		if strings.TrimSpace(c.Image) != "" {
			images = append(images, c.Image)
		}
	}
	for _, c := range pod.Spec.InitContainers {
		if strings.TrimSpace(c.Image) != "" {
			images = append(images, c.Image)
		}
	}
	for _, c := range pod.Spec.EphemeralContainer {
		if strings.TrimSpace(c.Image) != "" {
			images = append(images, c.Image)
		}
	}
	return images
}

func decodeYAMLDocs(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	var docs []map[string]any
	for {
		var doc map[string]any
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(doc) == 0 {
			continue
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func hasStringRecursive(v any, needle string) bool {
	switch t := v.(type) {
	case string:
		return strings.Contains(t, needle)
	case []any:
		for _, item := range t {
			if hasStringRecursive(item, needle) {
				return true
			}
		}
	case map[string]any:
		for _, item := range t {
			if hasStringRecursive(item, needle) {
				return true
			}
		}
	}
	return false
}

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asSlice(v any) ([]any, bool) {
	s, ok := v.([]any)
	return s, ok
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
