package util

import (
	"fmt"
	"os"
	"strings"
)

// PathMapping represents a single path mapping rule
type PathMapping struct {
	From string // Source path prefix (as seen by media server)
	To   string // Destination path prefix (as seen by Subgen)
}

// PathMapper handles path translation between media server and Subgen
type PathMapper struct {
	enabled  bool
	mappings []PathMapping
}

// NewPathMapper creates a new path mapper from configuration
//
// Parameters:
//   - enabled: Whether path mapping is enabled
//   - fromPaths: Comma-separated list of source path prefixes
//   - toPaths: Comma-separated list of destination path prefixes
//
// Returns an error if:
//   - Path mapping is enabled but paths are empty
//   - Number of from paths doesn't match number of to paths
//   - Any from or to path is empty after trimming
func NewPathMapper(enabled bool, fromPaths, toPaths string) (*PathMapper, error) {
	if !enabled {
		return &PathMapper{enabled: false}, nil
	}

	// Validate paths are not empty when enabled
	if strings.TrimSpace(fromPaths) == "" || strings.TrimSpace(toPaths) == "" {
		return nil, fmt.Errorf("empty path mapping configuration: FROM='%s', TO='%s'", fromPaths, toPaths)
	}

	// Parse comma-separated paths
	froms := strings.Split(fromPaths, ",")
	tos := strings.Split(toPaths, ",")

	if len(froms) != len(tos) {
		return nil, fmt.Errorf("path mapping mismatch: %d from paths, %d to paths", len(froms), len(tos))
	}

	mappings := make([]PathMapping, 0, len(froms))
	for i := range froms {
		fromPath := strings.TrimSpace(froms[i])
		toPath := strings.TrimSpace(tos[i])

		// Validate individual paths are not empty
		if fromPath == "" || toPath == "" {
			return nil, fmt.Errorf("empty path mapping at index %d: FROM='%s', TO='%s'", i, fromPath, toPath)
		}

		// Normalize trailing slashes - remove them for consistent matching
		fromPath = strings.TrimSuffix(fromPath, "/")
		toPath = strings.TrimSuffix(toPath, "/")

		mappings = append(mappings, PathMapping{
			From: fromPath,
			To:   toPath,
		})
	}

	return &PathMapper{
		enabled:  true,
		mappings: mappings,
	}, nil
}

// Map translates a path from media server view to Subgen view
//
// The function:
//  1. Returns the original path unchanged if mapping is disabled
//  2. Iterates through mappings and applies the first matching one
//  3. Validates that the mapped path exists on the filesystem
//  4. Returns the original path unchanged if no mapping matches
//
// Returns an error if the mapped path doesn't exist or can't be accessed.
func (pm *PathMapper) Map(path string) (string, error) {
	if !pm.enabled {
		return path, nil
	}

	for _, mapping := range pm.mappings {
		if strings.HasPrefix(path, mapping.From) {
			// Replace the prefix
			mapped := mapping.To + strings.TrimPrefix(path, mapping.From)

			// Validate mapped path exists
			if _, err := os.Stat(mapped); err != nil {
				if os.IsNotExist(err) {
					return "", fmt.Errorf("mapped path does not exist: %s (original: %s, mapping: %s -> %s)",
						mapped, path, mapping.From, mapping.To)
				}
				return "", fmt.Errorf("cannot access mapped path: %s: %w", mapped, err)
			}

			return mapped, nil
		}
	}

	// No mapping matched - return original path unchanged
	return path, nil
}

// Unmap translates a path from Subgen view to media server view (reverse mapping)
//
// This is useful for logging or API responses that need to reference paths
// from the media server's perspective.
//
// Returns the original path unchanged if:
//   - Mapping is disabled
//   - No mapping matches the path
func (pm *PathMapper) Unmap(path string) string {
	if !pm.enabled {
		return path
	}

	for _, mapping := range pm.mappings {
		if strings.HasPrefix(path, mapping.To) {
			// Replace the prefix in reverse
			return mapping.From + strings.TrimPrefix(path, mapping.To)
		}
	}

	// No mapping matched - return original path unchanged
	return path
}

// Enabled returns true if path mapping is enabled
func (pm *PathMapper) Enabled() bool {
	return pm.enabled
}

// Mappings returns the list of configured mappings
func (pm *PathMapper) Mappings() []PathMapping {
	return pm.mappings
}
