package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	*s = append(*s, value)
	return nil
}

type cpValues struct {
	Image struct {
		Repository string            `yaml:"repository"`
		Tag        string            `yaml:"tag"`
		Digest     string            `yaml:"digest"`
		Digests    map[string]string `yaml:"digests"`
	} `yaml:"image"`
	Services map[string]map[string]any `yaml:"services"`
	UI       struct {
		Enabled bool `yaml:"enabled"`
		Image   struct {
			Repository string `yaml:"repository"`
			Tag        string `yaml:"tag"`
			Digest     string `yaml:"digest"`
		} `yaml:"image"`
	} `yaml:"ui"`
	Postgres struct {
		Enabled bool   `yaml:"enabled"`
		Image   string `yaml:"image"`
	} `yaml:"postgres"`
	Minio struct {
		Enabled bool   `yaml:"enabled"`
		Image   string `yaml:"image"`
		MCImage string `yaml:"mcImage"`
	} `yaml:"minio"`
	Tests struct {
		Image struct {
			Repository string `yaml:"repository"`
			Tag        string `yaml:"tag"`
			Digest     string `yaml:"digest"`
		} `yaml:"image"`
	} `yaml:"tests"`
}

type dpValues struct {
	Image struct {
		Repository string `yaml:"repository"`
		Tag        string `yaml:"tag"`
		Digest     string `yaml:"digest"`
	} `yaml:"image"`
	Tests struct {
		Image struct {
			Repository string `yaml:"repository"`
			Tag        string `yaml:"tag"`
			Digest     string `yaml:"digest"`
		} `yaml:"image"`
	} `yaml:"tests"`
}

type chartMeta struct {
	Name string `yaml:"name"`
}

func main() {
	var charts stringSliceFlag
	var valuesFiles stringSliceFlag
	flag.Var(&charts, "chart", "path to chart (repeatable)")
	flag.Var(&valuesFiles, "values", "values.yaml override (repeatable)")
	flag.Parse()

	if len(charts) == 0 {
		fmt.Fprintln(os.Stderr, "at least one --chart is required")
		os.Exit(2)
	}

	imageSet := make(map[string]struct{})
	for _, chartPath := range charts {
		if err := collectChartImages(chartPath, valuesFiles, imageSet); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}

	images := make([]string, 0, len(imageSet))
	for img := range imageSet {
		images = append(images, img)
	}
	sort.Strings(images)
	for _, img := range images {
		fmt.Println(img)
	}
}

func collectChartImages(chartPath string, valuesFiles []string, imageSet map[string]struct{}) error {
	chartPath = strings.TrimSpace(chartPath)
	if chartPath == "" {
		return errors.New("chart path is required")
	}
	chartMetaPath := filepath.Join(chartPath, "Chart.yaml")
	chartMetaRaw, err := os.ReadFile(chartMetaPath)
	if err != nil {
		return fmt.Errorf("read chart metadata: %w", err)
	}
	var meta chartMeta
	if err := yaml.Unmarshal(chartMetaRaw, &meta); err != nil {
		return fmt.Errorf("parse chart metadata: %w", err)
	}
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		return fmt.Errorf("chart name missing in %s", chartMetaPath)
	}

	valuesMap, err := loadMergedValues(chartPath, valuesFiles)
	if err != nil {
		return err
	}

	switch name {
	case "animus-datapilot":
		var values cpValues
		if err := decodeValues(valuesMap, &values); err != nil {
			return err
		}
		addImages(imageSet, collectCPImages(values)...)
	case "animus-dataplane":
		var values dpValues
		if err := decodeValues(valuesMap, &values); err != nil {
			return err
		}
		addImages(imageSet, collectDPImages(values)...)
	default:
		return fmt.Errorf("unsupported chart %s", name)
	}
	return nil
}

func loadMergedValues(chartPath string, valuesFiles []string) (map[string]any, error) {
	basePath := filepath.Join(chartPath, "values.yaml")
	base, err := loadYAMLMap(basePath)
	if err != nil {
		return nil, fmt.Errorf("load values: %w", err)
	}
	for _, file := range valuesFiles {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		override, err := loadYAMLMap(file)
		if err != nil {
			return nil, fmt.Errorf("load values override: %w", err)
		}
		mergeMaps(base, override)
	}
	return base, nil
}

func decodeValues(values map[string]any, out any) error {
	payload, err := yaml.Marshal(values)
	if err != nil {
		return fmt.Errorf("marshal values: %w", err)
	}
	if err := yaml.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("parse values: %w", err)
	}
	return nil
}

func loadYAMLMap(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func mergeMaps(dst, src map[string]any) {
	for key, value := range src {
		if valueMap, ok := value.(map[string]any); ok {
			if existing, ok := dst[key].(map[string]any); ok {
				mergeMaps(existing, valueMap)
				continue
			}
		}
		dst[key] = value
	}
}

func collectCPImages(values cpValues) []string {
	images := make([]string, 0)
	for name := range values.Services {
		images = append(images, serviceImage(values.Image.Repository, values.Image.Tag, values.Image.Digest, values.Image.Digests, name))
	}
	if values.UI.Enabled {
		images = append(images, uiImage(values))
	}
	if values.Postgres.Enabled && strings.TrimSpace(values.Postgres.Image) != "" {
		images = append(images, strings.TrimSpace(values.Postgres.Image))
	}
	if values.Minio.Enabled {
		if strings.TrimSpace(values.Minio.Image) != "" {
			images = append(images, strings.TrimSpace(values.Minio.Image))
		}
		if strings.TrimSpace(values.Minio.MCImage) != "" {
			images = append(images, strings.TrimSpace(values.Minio.MCImage))
		}
	}
	images = append(images, testImage(values.Tests.Image.Repository, values.Tests.Image.Tag, values.Tests.Image.Digest))
	return filterEmpty(images)
}

func collectDPImages(values dpValues) []string {
	images := []string{
		dataplaneImage(values.Image.Repository, values.Image.Tag, values.Image.Digest),
		testImage(values.Tests.Image.Repository, values.Tests.Image.Tag, values.Tests.Image.Digest),
	}
	return filterEmpty(images)
}

func serviceImage(repo, tag, digest string, digests map[string]string, name string) string {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return ""
	}
	resolvedDigest := ""
	if digests != nil {
		resolvedDigest = strings.TrimSpace(digests[name])
	}
	if resolvedDigest == "" {
		resolvedDigest = strings.TrimSpace(digest)
	}
	if resolvedDigest != "" {
		return fmt.Sprintf("%s/%s@%s", repo, name, resolvedDigest)
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("%s/%s:%s", repo, name, tag)
}

func uiImage(values cpValues) string {
	repo := strings.TrimSpace(values.UI.Image.Repository)
	digest := strings.TrimSpace(values.UI.Image.Digest)
	tag := strings.TrimSpace(values.UI.Image.Tag)
	if repo != "" {
		if digest != "" {
			return fmt.Sprintf("%s@%s", repo, digest)
		}
		if tag == "" {
			tag = strings.TrimSpace(values.Image.Tag)
		}
		if tag == "" {
			tag = "latest"
		}
		return fmt.Sprintf("%s:%s", repo, tag)
	}
	repo = strings.TrimSpace(values.Image.Repository)
	if repo == "" {
		return ""
	}
	if digest != "" {
		return fmt.Sprintf("%s/ui@%s", repo, digest)
	}
	if tag == "" {
		tag = strings.TrimSpace(values.Image.Tag)
	}
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("%s/ui:%s", repo, tag)
}

func dataplaneImage(repo, tag, digest string) string {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return ""
	}
	digest = strings.TrimSpace(digest)
	if digest != "" {
		return fmt.Sprintf("%s/dataplane@%s", repo, digest)
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("%s/dataplane:%s", repo, tag)
}

func testImage(repo, tag, digest string) string {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return ""
	}
	digest = strings.TrimSpace(digest)
	if digest != "" {
		return fmt.Sprintf("%s@%s", repo, digest)
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("%s:%s", repo, tag)
}

func filterEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func addImages(set map[string]struct{}, images ...string) {
	for _, img := range images {
		img = strings.TrimSpace(img)
		if img == "" {
			continue
		}
		set[img] = struct{}{}
	}
}
