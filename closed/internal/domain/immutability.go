package domain

import (
	"errors"
	"fmt"
	"reflect"
)

// EnsureDatasetVersionImmutable enforces immutability for dataset versions.
func EnsureDatasetVersionImmutable(before, after DatasetVersion) error {
	if before.ID == "" || after.ID == "" {
		return errors.New("dataset version ids are required")
	}
	if before.ID != after.ID {
		return fmt.Errorf("dataset version id changed from %q to %q", before.ID, after.ID)
	}
	if before.DatasetID != after.DatasetID {
		return errors.New("dataset id is immutable")
	}
	if before.ProjectID != after.ProjectID {
		return errors.New("project id is immutable")
	}
	if before.Ordinal != after.Ordinal {
		return errors.New("ordinal is immutable")
	}
	if before.ContentSHA256 != after.ContentSHA256 {
		return errors.New("content sha256 is immutable")
	}
	if before.ObjectKey != after.ObjectKey {
		return errors.New("object key is immutable")
	}
	if before.SizeBytes != after.SizeBytes {
		return errors.New("size bytes is immutable")
	}
	if !reflect.DeepEqual(before.Metadata, after.Metadata) {
		return errors.New("metadata is immutable")
	}
	return nil
}

// EnsureArtifactImmutable enforces immutability for artifact identity and storage.
func EnsureArtifactImmutable(before, after Artifact) error {
	if before.ID == "" || after.ID == "" {
		return errors.New("artifact ids are required")
	}
	if before.ID != after.ID {
		return fmt.Errorf("artifact id changed from %q to %q", before.ID, after.ID)
	}
	if before.ProjectID != after.ProjectID {
		return errors.New("project id is immutable")
	}
	if before.Kind != after.Kind {
		return errors.New("kind is immutable")
	}
	if before.ContentType != after.ContentType {
		return errors.New("content type is immutable")
	}
	if before.ObjectKey != after.ObjectKey {
		return errors.New("object key is immutable")
	}
	if before.SHA256 != after.SHA256 {
		return errors.New("sha256 is immutable")
	}
	if before.SizeBytes != after.SizeBytes {
		return errors.New("size bytes is immutable")
	}
	if !reflect.DeepEqual(before.Metadata, after.Metadata) {
		return errors.New("metadata is immutable")
	}
	return nil
}
