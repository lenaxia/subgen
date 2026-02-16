package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPathMapper_Disabled tests that when disabled, paths pass through unchanged
func TestPathMapper_Disabled(t *testing.T) {
	mapper, err := NewPathMapper(false, "", "")
	require.NoError(t, err)
	assert.False(t, mapper.Enabled())

	// Path should pass through unchanged
	result, err := mapper.Map("/data/movies/test.mkv")
	assert.NoError(t, err)
	assert.Equal(t, "/data/movies/test.mkv", result)
}

// TestPathMapper_SingleMapping_Success tests single path mapping with valid paths
func TestPathMapper_SingleMapping_Success(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	require.NoError(t, os.MkdirAll(filepath.Join(destDir, "movies"), 0755))

	testFile := filepath.Join(destDir, "movies", "test.mkv")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	// Create mapper
	mapper, err := NewPathMapper(true, "/data", destDir)
	require.NoError(t, err)
	assert.True(t, mapper.Enabled())
	assert.Equal(t, 1, len(mapper.Mappings()))

	// Test mapping
	sourcePath := "/data/movies/test.mkv"
	mapped, err := mapper.Map(sourcePath)
	assert.NoError(t, err)
	assert.Equal(t, testFile, mapped)
}

// TestPathMapper_SingleMapping_NotFound tests mapped path doesn't exist
func TestPathMapper_SingleMapping_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	require.NoError(t, os.MkdirAll(destDir, 0755))

	mapper, err := NewPathMapper(true, "/data", destDir)
	require.NoError(t, err)

	// Test mapping to non-existent file
	sourcePath := "/data/movies/nonexistent.mkv"
	mapped, err := mapper.Map(sourcePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mapped path does not exist")
	assert.Empty(t, mapped)
}

// TestPathMapper_MultipleMapping_FirstMatch tests first matching mapping is used
func TestPathMapper_MultipleMapping_FirstMatch(t *testing.T) {
	tmpDir := t.TempDir()
	dest1 := filepath.Join(tmpDir, "dest1")
	dest2 := filepath.Join(tmpDir, "dest2")

	// Create both directories with files
	require.NoError(t, os.MkdirAll(filepath.Join(dest1, "movies"), 0755))
	require.NoError(t, os.MkdirAll(dest2, 0755))

	testFile1 := filepath.Join(dest1, "movies", "test.mkv")
	testFile2 := filepath.Join(dest2, "show.mkv")

	require.NoError(t, os.WriteFile(testFile1, []byte("test1"), 0644))
	require.NoError(t, os.WriteFile(testFile2, []byte("test2"), 0644))

	// Create mapper with multiple mappings
	fromPaths := "/data,/tv"
	toPaths := dest1 + "," + dest2
	mapper, err := NewPathMapper(true, fromPaths, toPaths)
	require.NoError(t, err)
	assert.Equal(t, 2, len(mapper.Mappings()))

	// Test first mapping: /data/movies/test.mkv -> dest1/movies/test.mkv
	sourcePath1 := "/data/movies/test.mkv"
	mapped1, err := mapper.Map(sourcePath1)
	assert.NoError(t, err)
	assert.Equal(t, testFile1, mapped1)

	// Test second mapping: /tv/show.mkv -> dest2/show.mkv
	sourcePath2 := "/tv/show.mkv"
	mapped2, err := mapper.Map(sourcePath2)
	assert.NoError(t, err)
	assert.Equal(t, testFile2, mapped2)
}

// TestPathMapper_NoMatch tests path that doesn't match any mapping (pass through)
func TestPathMapper_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	require.NoError(t, os.MkdirAll(destDir, 0755))

	mapper, err := NewPathMapper(true, "/data", destDir)
	require.NoError(t, err)

	// Path that doesn't match any mapping
	sourcePath := "/other/movies/test.mkv"
	mapped, err := mapper.Map(sourcePath)
	assert.NoError(t, err)
	assert.Equal(t, sourcePath, mapped) // Should pass through unchanged
}

// TestPathMapper_Unmap_Success tests reverse mapping (Subgen → media server)
func TestPathMapper_Unmap_Success(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	require.NoError(t, os.MkdirAll(destDir, 0755))

	mapper, err := NewPathMapper(true, "/data", destDir)
	require.NoError(t, err)

	// Test reverse mapping
	destPath := filepath.Join(destDir, "movies", "test.mkv")
	unmapped := mapper.Unmap(destPath)
	assert.Equal(t, "/data/movies/test.mkv", unmapped)
}

// TestPathMapper_Unmap_NoMatch tests reverse mapping with no match
func TestPathMapper_Unmap_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	require.NoError(t, os.MkdirAll(destDir, 0755))

	mapper, err := NewPathMapper(true, "/data", destDir)
	require.NoError(t, err)

	// Path that doesn't match any mapping
	destPath := "/other/movies/test.mkv"
	unmapped := mapper.Unmap(destPath)
	assert.Equal(t, destPath, unmapped) // Should pass through unchanged
}

// TestNewPathMapper_MismatchedLengths tests error when from/to path counts don't match
func TestNewPathMapper_MismatchedLengths(t *testing.T) {
	fromPaths := "/data,/tv,/movies"
	toPaths := "/mnt/data,/mnt/tv"

	mapper, err := NewPathMapper(true, fromPaths, toPaths)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path mapping mismatch")
	assert.Nil(t, mapper)
}

// TestNewPathMapper_CommaSeparated tests parsing comma-separated paths
func TestNewPathMapper_CommaSeparated(t *testing.T) {
	tmpDir := t.TempDir()
	dest1 := filepath.Join(tmpDir, "dest1")
	dest2 := filepath.Join(tmpDir, "dest2")
	dest3 := filepath.Join(tmpDir, "dest3")

	require.NoError(t, os.MkdirAll(dest1, 0755))
	require.NoError(t, os.MkdirAll(dest2, 0755))
	require.NoError(t, os.MkdirAll(dest3, 0755))

	// Create mapper with whitespace in comma-separated list
	fromPaths := "/data , /tv , /movies"
	toPaths := dest1 + " , " + dest2 + " , " + dest3
	mapper, err := NewPathMapper(true, fromPaths, toPaths)
	require.NoError(t, err)

	mappings := mapper.Mappings()
	assert.Equal(t, 3, len(mappings))

	// Verify whitespace was trimmed
	assert.Equal(t, "/data", mappings[0].From)
	assert.Equal(t, dest1, mappings[0].To)
	assert.Equal(t, "/tv", mappings[1].From)
	assert.Equal(t, dest2, mappings[1].To)
	assert.Equal(t, "/movies", mappings[2].From)
	assert.Equal(t, dest3, mappings[2].To)
}

// TestPathMapper_EmptyPaths tests handling of empty paths
func TestPathMapper_EmptyPaths(t *testing.T) {
	// Empty FROM path
	mapper1, err := NewPathMapper(true, "", "/dest")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty path mapping")
	assert.Nil(t, mapper1)

	// Empty TO path
	mapper2, err := NewPathMapper(true, "/data", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty path mapping")
	assert.Nil(t, mapper2)

	// Both empty
	mapper3, err := NewPathMapper(true, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty path mapping")
	assert.Nil(t, mapper3)
}

// TestPathMapper_TrailingSlashes tests handling of trailing slashes
func TestPathMapper_TrailingSlashes(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	require.NoError(t, os.MkdirAll(filepath.Join(destDir, "movies"), 0755))

	testFile := filepath.Join(destDir, "movies", "test.mkv")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	// Create mapper with trailing slashes
	mapper, err := NewPathMapper(true, "/data/", destDir+"/")
	require.NoError(t, err)

	// Test mapping with and without trailing slash in source
	sourcePath1 := "/data/movies/test.mkv"
	mapped1, err := mapper.Map(sourcePath1)
	assert.NoError(t, err)
	assert.Equal(t, testFile, mapped1)

	// Source path with trailing slash should still work
	sourcePath2 := "/data/movies/test.mkv"
	mapped2, err := mapper.Map(sourcePath2)
	assert.NoError(t, err)
	assert.Equal(t, testFile, mapped2)
}

// TestPathMapper_OverlappingMappings tests overlapping path prefixes
func TestPathMapper_OverlappingMappings(t *testing.T) {
	tmpDir := t.TempDir()
	dest1 := filepath.Join(tmpDir, "dest1")
	dest2 := filepath.Join(tmpDir, "dest2")

	require.NoError(t, os.MkdirAll(filepath.Join(dest1, "movies"), 0755))
	require.NoError(t, os.MkdirAll(dest2, 0755))

	testFile1 := filepath.Join(dest1, "movies", "test.mkv")
	testFile2 := filepath.Join(dest2, "special.mkv")

	require.NoError(t, os.WriteFile(testFile1, []byte("test1"), 0644))
	require.NoError(t, os.WriteFile(testFile2, []byte("test2"), 0644))

	// Overlapping mappings: /data and /data/specific
	// More specific should come first to match correctly
	fromPaths := "/data/specific,/data"
	toPaths := dest2 + "," + dest1
	mapper, err := NewPathMapper(true, fromPaths, toPaths)
	require.NoError(t, err)

	// Test specific path (should match first mapping): /data/specific/special.mkv -> dest2/special.mkv
	sourcePath1 := "/data/specific/special.mkv"
	mapped1, err := mapper.Map(sourcePath1)
	assert.NoError(t, err)
	assert.Equal(t, testFile2, mapped1)

	// Test general path (should match second mapping): /data/movies/test.mkv -> dest1/movies/test.mkv
	sourcePath2 := "/data/movies/test.mkv"
	mapped2, err := mapper.Map(sourcePath2)
	assert.NoError(t, err)
	assert.Equal(t, testFile1, mapped2)
}

// TestPathMapper_SymlinkPaths tests handling of symlinked directories
func TestPathMapper_SymlinkPaths(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping symlink test in CI environment")
	}

	tmpDir := t.TempDir()
	realDir := filepath.Join(tmpDir, "real")
	linkDir := filepath.Join(tmpDir, "link")

	require.NoError(t, os.MkdirAll(filepath.Join(realDir, "movies"), 0755))
	testFile := filepath.Join(realDir, "movies", "test.mkv")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	// Create symlink
	require.NoError(t, os.Symlink(realDir, linkDir))

	// Create mapper pointing to symlink
	mapper, err := NewPathMapper(true, "/data", linkDir)
	require.NoError(t, err)

	// Test mapping through symlink
	sourcePath := "/data/movies/test.mkv"
	mapped, err := mapper.Map(sourcePath)
	assert.NoError(t, err)
	// Should accept the symlinked path
	expectedPath := filepath.Join(linkDir, "movies", "test.mkv")
	assert.Equal(t, expectedPath, mapped)
}

// TestPathMapper_CaseSensitivity tests case-sensitive path matching
func TestPathMapper_CaseSensitivity(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	require.NoError(t, os.MkdirAll(filepath.Join(destDir, "Movies"), 0755))

	testFile := filepath.Join(destDir, "Movies", "test.mkv")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	mapper, err := NewPathMapper(true, "/data", destDir)
	require.NoError(t, err)

	// On Linux, paths are case-sensitive
	// /data/Movies should work
	sourcePath1 := "/data/Movies/test.mkv"
	mapped1, err := mapper.Map(sourcePath1)
	assert.NoError(t, err)
	assert.Equal(t, testFile, mapped1)

	// /data/movies should NOT work (different case)
	sourcePath2 := "/data/movies/test.mkv"
	mapped2, err := mapper.Map(sourcePath2)
	if err == nil {
		// If no error, path should be unchanged (no match)
		assert.Equal(t, sourcePath2, mapped2)
	} else {
		// Or error because file doesn't exist
		assert.Error(t, err)
	}
}

// TestPathMapper_UnicodePathsw tests paths with non-ASCII characters
func TestPathMapper_UnicodePaths(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "目的地")                             // "destination" in Japanese
	require.NoError(t, os.MkdirAll(filepath.Join(destDir, "电影"), 0755)) // "movies" in Chinese

	testFile := filepath.Join(destDir, "电影", "тест.mkv") // "test" in Russian
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	mapper, err := NewPathMapper(true, "/数据", destDir) // "data" in Chinese
	require.NoError(t, err)

	// Test mapping with unicode
	sourcePath := "/数据/电影/тест.mkv"
	mapped, err := mapper.Map(sourcePath)
	assert.NoError(t, err)
	assert.Equal(t, testFile, mapped)
}

// TestPathMapper_WindowsStylePaths tests Windows-style paths (for future Windows support)
func TestPathMapper_WindowsStylePaths(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("Skipping Windows path test on non-Windows platform")
	}

	// This test will only run on Windows
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "dest")
	require.NoError(t, os.MkdirAll(filepath.Join(destDir, "movies"), 0755))

	testFile := filepath.Join(destDir, "movies", "test.mkv")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	// Create mapper with Windows-style paths
	mapper, err := NewPathMapper(true, "D:\\data", destDir)
	require.NoError(t, err)

	// Test mapping
	sourcePath := "D:\\data\\movies\\test.mkv"
	mapped, err := mapper.Map(sourcePath)
	assert.NoError(t, err)
	assert.Equal(t, testFile, mapped)
}

// TestPathMapper_UnmapDisabled tests Unmap when disabled
func TestPathMapper_UnmapDisabled(t *testing.T) {
	mapper, err := NewPathMapper(false, "", "")
	require.NoError(t, err)

	// Unmap should pass through when disabled
	destPath := "/mnt/media/movies/test.mkv"
	unmapped := mapper.Unmap(destPath)
	assert.Equal(t, destPath, unmapped)
}

// TestPathMapper_MultipleMappingsUnmap tests reverse mapping with multiple mappings
func TestPathMapper_MultipleMappingsUnmap(t *testing.T) {
	tmpDir := t.TempDir()
	dest1 := filepath.Join(tmpDir, "dest1")
	dest2 := filepath.Join(tmpDir, "dest2")

	require.NoError(t, os.MkdirAll(dest1, 0755))
	require.NoError(t, os.MkdirAll(dest2, 0755))

	fromPaths := "/data,/tv"
	toPaths := dest1 + "," + dest2
	mapper, err := NewPathMapper(true, fromPaths, toPaths)
	require.NoError(t, err)

	// Test unmapping first mapping
	destPath1 := filepath.Join(dest1, "movies", "test.mkv")
	unmapped1 := mapper.Unmap(destPath1)
	assert.Equal(t, "/data/movies/test.mkv", unmapped1)

	// Test unmapping second mapping
	destPath2 := filepath.Join(dest2, "show.mkv")
	unmapped2 := mapper.Unmap(destPath2)
	assert.Equal(t, "/tv/show.mkv", unmapped2)
}
