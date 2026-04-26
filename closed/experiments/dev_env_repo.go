package main

import (
	"errors"
	"net/url"
	"strings"
)

type repoAllowlistEntry struct {
	host       string
	pathPrefix string
}

type repoURLInfo struct {
	Host   string
	Path   string
	Scheme string
}

var errRepoURLNotAllowed = errors.New("repo_url_not_allowed")

func parseRepoAllowlist(raw string) ([]repoAllowlistEntry, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]repoAllowlistEntry, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		normalized, err := normalizeRepoAllowlistEntry(entry)
		if err != nil {
			return nil, err
		}
		key := normalized.host + "/" + normalized.pathPrefix
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeRepoAllowlistEntry(entry string) (repoAllowlistEntry, error) {
	normalized := strings.TrimSpace(entry)
	if normalized != "" && !strings.Contains(normalized, "://") && !strings.HasPrefix(normalized, "git@") {
		normalized = "https://" + normalized
	}
	info, err := parseRepoURL(normalized, true)
	if err != nil {
		return repoAllowlistEntry{}, err
	}
	return repoAllowlistEntry{
		host:       info.Host,
		pathPrefix: strings.Trim(info.Path, "/"),
	}, nil
}

func validateRepoAllowlist(repoURL string, allowlist []repoAllowlistEntry) error {
	info, err := parseRepoURL(repoURL, false)
	if err != nil {
		return err
	}
	if len(allowlist) == 0 {
		return nil
	}
	for _, entry := range allowlist {
		if entry.host != info.Host {
			continue
		}
		if entry.pathPrefix == "" {
			return nil
		}
		if strings.HasPrefix(info.Path, entry.pathPrefix) {
			return nil
		}
	}
	return errRepoURLNotAllowed
}

func parseRepoURL(raw string, allowMissingPath bool) (repoURLInfo, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return repoURLInfo{}, errors.New("repo_url_required")
	}
	if strings.HasPrefix(raw, "git@") {
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return repoURLInfo{}, errors.New("invalid_repo_url")
		}
		host := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(parts[0], "git@")))
		path := strings.Trim(strings.TrimSpace(parts[1]), "/")
		if host == "" || (path == "" && !allowMissingPath) {
			return repoURLInfo{}, errors.New("invalid_repo_url")
		}
		return repoURLInfo{Host: host, Path: path, Scheme: "ssh"}, nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return repoURLInfo{}, errors.New("invalid_repo_url")
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return repoURLInfo{}, errors.New("invalid_repo_url")
	}
	if parsed.User != nil {
		return repoURLInfo{}, errors.New("repo_url_userinfo_not_allowed")
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	path := strings.Trim(strings.TrimSpace(parsed.Path), "/")
	if host == "" || (path == "" && !allowMissingPath) {
		return repoURLInfo{}, errors.New("invalid_repo_url")
	}
	return repoURLInfo{Host: host, Path: path, Scheme: strings.ToLower(parsed.Scheme)}, nil
}
