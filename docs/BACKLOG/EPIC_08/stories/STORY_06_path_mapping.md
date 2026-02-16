# Story 06: Path Mapping Application

**Epic**: EPIC_08  
**Status**: Not Started  
**Effort**: 2-3 hours  
**Priority**: HIGH  
**Assignee**: Delegation Agent

---

## User Story

As a Subgen user running in Docker,
I want path mapping to translate media server paths to Subgen's mounted paths,
So that I can use different mount points between containers without path mismatches.

---

## Background

When Subgen runs in a container separate from the media server (Plex/Jellyfin/Emby), the file paths reported by the media server may differ from the paths Subgen can access. Path mapping allows translation between these two views of the filesystem.

**Example:**
- Plex sees: `/data/movies/action/movie.mkv`
- Subgen sees: `/mnt/media/movies/action/movie.mkv`
- Mapping: `/data` → `/mnt/media`

The original subgen.py (lines 2062-2066) had configuration variables but they were never actually applied in the code. This story implements the actual path mapping logic.

---

## Acceptance Criteria

- [ ] Use existing config: `USE_PATH_MAPPING`, `PATH_MAPPING_FROM`, `PATH_MAPPING_TO`
- [ ] Apply mapping in webhook handlers **before** queueing tasks
- [ ] Support multiple mappings (comma-separated)
- [ ] Bidirectional mapping support (from media server to Subgen paths)
- [ ] Validation: Warn if mapped path doesn't exist
- [ ] Validation: Error if mapping is enabled but paths are invalid
- [ ] Unit tests with mocked filesystem (happy + unhappy paths)
- [ ] Integration tests with actual temp directories
- [ ] Type checking passes
- [ ] Work log created

---

## Technical Design

### Approach

Create a `PathMapper` utility in `orchestrator/internal/util/` package that handles path translation. The webhook handlers will call this mapper before queueing tasks.

### PathMapper Interface

```go
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
func NewPathMapper(enabled bool, fromPaths, toPaths string) (*PathMapper, error) {
	if !enabled {
		return &PathMapper{enabled: false}, nil
	}
	
	// Parse comma-separated paths
	froms := strings.Split(fromPaths, ",")
	tos := strings.Split(toPaths, ",")
	
	if len(froms) != len(tos) {
		return nil, fmt.Errorf("path mapping mismatch: %d from paths, %d to paths", len(froms), len(tos))
	}
	
	mappings := make([]PathMapping, len(froms))
	for i := range froms {
		mappings[i] = PathMapping{
			From: strings.TrimSpace(froms[i]),
			To:   strings.TrimSpace(tos[i]),
		}
	}
	
	return &PathMapper{
		enabled:  true,
		mappings: mappings,
	}, nil
}

// Map translates a path from media server view to Subgen view
func (pm *PathMapper) Map(path string) (string, error) {
	if !pm.enabled {
		return path, nil
	}
	
	for _, mapping := range pm.mappings {
		if strings.HasPrefix(path, mapping.From) {
			mapped := strings.Replace(path, mapping.From, mapping.To, 1)
			
			// Validate mapped path exists
			if _, err := os.Stat(mapped); err != nil {
				if os.IsNotExist(err) {
					return "", fmt.Errorf("mapped path does not exist: %s (original: %s)", mapped, path)
				}
				return "", fmt.Errorf("cannot access mapped path: %s: %w", mapped, err)
			}
			
			return mapped, nil
		}
	}
	
	// No mapping matched - return original path
	return path, nil
}

// Unmap translates a path from Subgen view to media server view (reverse mapping)
func (pm *PathMapper) Unmap(path string) string {
	if !pm.enabled {
		return path
	}
	
	for _, mapping := range pm.mappings {
		if strings.HasPrefix(path, mapping.To) {
			return strings.Replace(path, mapping.To, mapping.From, 1)
		}
	}
	
	// No mapping matched - return original path
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
```

### Files to Create

1. **orchestrator/internal/util/path_mapper.go**
   - PathMapper struct and implementation
   - Map() method for forward translation
   - Unmap() method for reverse translation
   - Validation logic

2. **orchestrator/internal/util/path_mapper_test.go**
   - Unit tests with mocked filesystem
   - Tests for single and multiple mappings
   - Tests for validation errors
   - Tests for bidirectional mapping

### Files to Modify

1. **orchestrator/internal/config/config.go**
   - Add PathMapping to Config struct (if not already present)
   - Load USE_PATH_MAPPING, PATH_MAPPING_FROM, PATH_MAPPING_TO

2. **orchestrator/internal/webhooks/server.go**
   - Add PathMapper field to Server struct
   - Initialize PathMapper in NewServer()

3. **orchestrator/internal/webhooks/plex.go**
   - Call pathMapper.Map() on file path before queueing

4. **orchestrator/internal/webhooks/jellyfin.go**
   - Call pathMapper.Map() on file path before queueing

5. **orchestrator/internal/webhooks/emby.go**
   - Call pathMapper.Map() on file path before queueing

### Configuration

```go
// Config struct additions
type Config struct {
	// ... existing fields ...
	
	PathMapping PathMappingConfig
}

type PathMappingConfig struct {
	Enabled bool
	From    string // Comma-separated source paths
	To      string // Comma-separated destination paths
}

// Load from environment
PathMapping: PathMappingConfig{
	Enabled: v.GetBool("USE_PATH_MAPPING"),
	From:    v.GetString("PATH_MAPPING_FROM"),
	To:      v.GetString("PATH_MAPPING_TO"),
},
```

### Integration Points

- **Webhook Handlers** - Call pathMapper.Map() before queueing
- **Task Queue** - Receives already-mapped paths
- **Worker** - Works with mapped paths (no changes needed)
- **Skip Logic** - Checks subtitle existence using mapped paths

---

## Testing Strategy

### Unit Tests

**path_mapper_test.go:**
```go
func TestPathMapper_Disabled(t *testing.T) {
	// When disabled, path should pass through unchanged
}

func TestPathMapper_SingleMapping_Success(t *testing.T) {
	// Test single path mapping with valid paths
}

func TestPathMapper_SingleMapping_NotFound(t *testing.T) {
	// Test mapped path doesn't exist
}

func TestPathMapper_MultipleMapping_FirstMatch(t *testing.T) {
	// Test first matching mapping is used
}

func TestPathMapper_MultipleMapping_SecondMatch(t *testing.T) {
	// Test second mapping when first doesn't match
}

func TestPathMapper_NoMatch(t *testing.T) {
	// Test path that doesn't match any mapping (pass through)
}

func TestPathMapper_Unmap_Success(t *testing.T) {
	// Test reverse mapping (Subgen → media server)
}

func TestPathMapper_Unmap_NoMatch(t *testing.T) {
	// Test reverse mapping with no match
}

func TestNewPathMapper_MismatchedLengths(t *testing.T) {
	// Test error when from/to path counts don't match
}

func TestNewPathMapper_CommaSeparated(t *testing.T) {
	// Test parsing comma-separated paths
}

func TestPathMapper_SpecialCharacters(t *testing.T) {
	// Test paths with spaces, unicode, etc.
}

func TestPathMapper_WindowsPaths(t *testing.T) {
	// Test Windows-style paths (C:\, D:\)
}
```

### Integration Tests

**Create temporary directory structures:**
```go
func TestPathMapper_Integration(t *testing.T) {
	// Create temp directories
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	destDir := filepath.Join(tmpDir, "dest")
	
	os.MkdirAll(filepath.Join(destDir, "movies"), 0755)
	testFile := filepath.Join(destDir, "movies", "test.mkv")
	os.WriteFile(testFile, []byte("test"), 0644)
	
	// Create mapper
	mapper, err := NewPathMapper(true, sourceDir, destDir)
	assert.NoError(t, err)
	
	// Test mapping
	sourcePath := filepath.Join(sourceDir, "movies", "test.mkv")
	mapped, err := mapper.Map(sourcePath)
	assert.NoError(t, err)
	assert.Equal(t, testFile, mapped)
}
```

### Manual Testing

```bash
# Test 1: Single mapping (basic)
export USE_PATH_MAPPING=true
export PATH_MAPPING_FROM=/data
export PATH_MAPPING_TO=/mnt/media

# Create test structure
mkdir -p /mnt/media/movies
touch /mnt/media/movies/test.mkv

# Trigger webhook with /data/movies/test.mkv
# Verify mapped to /mnt/media/movies/test.mkv

# Test 2: Multiple mappings
export PATH_MAPPING_FROM="/data,/tv"
export PATH_MAPPING_TO="/mnt/media,/mnt/television"

# Create test structures
mkdir -p /mnt/media/movies /mnt/television/shows
touch /mnt/media/movies/movie.mkv
touch /mnt/television/shows/show.mkv

# Trigger webhooks with /data/movies/movie.mkv and /tv/shows/show.mkv
# Verify both map correctly

# Test 3: Path doesn't exist
export PATH_MAPPING_FROM=/data
export PATH_MAPPING_TO=/nonexistent

# Trigger webhook with /data/movies/test.mkv
# Verify error logged: "mapped path does not exist"

# Test 4: Disabled mapping
export USE_PATH_MAPPING=false

# Trigger webhook with /data/movies/test.mkv
# Verify path passes through unchanged
```

---

## Definition of Done

- [ ] Story file created (this document)
- [ ] Tests written FIRST (TDD)
- [ ] PathMapper implemented in util/path_mapper.go
- [ ] PathMapper integrated into webhook handlers
- [ ] Configuration loaded from environment
- [ ] All unit tests passing (>90% coverage)
- [ ] Integration tests passing
- [ ] Manual testing completed
- [ ] Type checking passes (go vet)
- [ ] Code follows Go best practices
- [ ] Error messages are clear and actionable
- [ ] Logging includes original and mapped paths
- [ ] Work log created (0022_2026-02-16_epic08_story06_path_mapping.md)
- [ ] Code committed and pushed

---

## Edge Cases to Handle

1. **Empty paths** - FROM or TO is empty string
2. **Trailing slashes** - Handle `/data/` and `/data` equivalently
3. **Overlapping mappings** - `/data` and `/data/movies` both mapped
4. **Symlinks** - Mapped path may be symlink
5. **Case sensitivity** - Linux vs Windows filesystem differences
6. **Unicode paths** - Non-ASCII characters in paths
7. **Network paths** - SMB/NFS mounts (\\server\share)
8. **Disabled but paths configured** - Should not error

---

## Success Criteria

1. **Accuracy**: All paths mapped correctly
2. **Validation**: Clear errors for invalid configurations
3. **Performance**: Mapping is O(1) per path
4. **Logging**: Clear logs showing path translation
5. **Reliability**: No panics on edge cases

---

## References

- **Original Config**: subgen.py lines 2062-2066
- **Docker Compose Example**: docs/BACKLOG/EPIC_08/README.md lines 331-345
- **Go filepath package**: Standard library for path manipulation
- **Go os package**: File existence validation

---

**Story Created**: 2026-02-16  
**Last Updated**: 2026-02-16
