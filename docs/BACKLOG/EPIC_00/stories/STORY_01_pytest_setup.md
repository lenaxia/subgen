# Story 01: Set up pytest Infrastructure with Fixtures

**Epic**: EPIC_00 - Testing Infrastructure  
**Status**: Not Started  
**Estimated Time**: 4-6 hours  
**Assignee**: TBD

---

## User Story

As a **developer working on Subgen**,  
I want **a fully configured pytest testing infrastructure with reusable fixtures and mocks**,  
So that **I can write and run tests efficiently for all components without loading real Whisper models or making real API calls**.

---

## Context

Subgen currently has **ZERO tests**. The repository has no `tests/` directory, no `pytest.ini`, no test dependencies in `requirements.txt`. Before we can write any tests (Stories 02-10), we need to create the entire testing infrastructure from scratch.

This story establishes the foundation that all other testing stories depend on. Without this infrastructure:
- We cannot write unit tests for `language_code.py` (Story 02)
- We cannot test the queue deduplication logic (Story 03)
- We cannot test webhook endpoints (Story 05)
- We cannot run tests in CI/CD (Story 08)

**Why this matters**: Every code change to Subgen is currently tested manually in production. This is risky and slow. Automated tests will catch bugs before they reach production and enable safe refactoring.

**What you'll build**: A complete pytest setup with:
1. Test discovery configuration (`pytest.ini`)
2. Coverage reporting configuration (`.coveragerc`)
3. Test directory structure (`tests/unit/`, `tests/integration/`, `tests/e2e/`)
4. Shared test fixtures (`tests/conftest.py`)
5. Mock objects for expensive operations (Whisper model, FFmpeg, media server APIs)
6. One example test to prove everything works

---

## Acceptance Criteria

- [ ] **15 test files pass** when running `pytest -v`
- [ ] `requirements-dev.txt` created with pytest and 4 plugins
- [ ] `pytest.ini` configured with 8+ settings
- [ ] `.coveragerc` configured with source/omit paths
- [ ] `tests/` directory created with 3 subdirectories
- [ ] `tests/conftest.py` created with 6+ fixtures
- [ ] Mock Whisper model fixture that doesn't load real models
- [ ] Mock FastAPI TestClient fixture for endpoint testing
- [ ] Example test file `tests/unit/test_example.py` with 3 passing tests
- [ ] Coverage report generates successfully
- [ ] All tests passing with `pytest tests/ -v`
- [ ] Work log created at `docs/WORKLOGS/NNNN_2026-02-15_EPIC_00_story_01_pytest_setup.md`

---

## Technical Design

### Files to Create

**1. `requirements-dev.txt`** (at project root: `/home/mikekao/personal/subgen/requirements-dev.txt`)
- **Purpose**: Contains all testing dependencies separate from production dependencies
- **Why separate**: Development dependencies shouldn't be in production Docker images
- **Contents**:
  ```txt
  # Testing Framework
  pytest>=7.4.0
  pytest-cov>=4.1.0
  pytest-mock>=3.11.0
  pytest-asyncio>=0.21.0
  
  # HTTP Testing
  httpx>=0.24.0  # Required for FastAPI TestClient
  ```

**2. `pytest.ini`** (at project root: `/home/mikekao/personal/subgen/pytest.ini`)
- **Purpose**: Configures pytest behavior, test discovery, and default options
- **Location**: MUST be at project root (same directory as `subgen.py`)
- **Contents**:
  ```ini
  [pytest]
  # Where to look for tests
  testpaths = tests
  
  # What files are tests (must start with test_)
  python_files = test_*.py
  python_classes = Test*
  python_functions = test_*
  
  # Default options when running pytest
  addopts = 
      -v                              # Verbose output
      --strict-markers                # Error if unknown markers used
      --cov=.                         # Measure coverage of current directory
      --cov-report=term-missing       # Show which lines aren't covered
      --cov-report=html               # Generate HTML coverage report
      --cov-report=xml                # Generate XML for CI/CD
      --tb=short                      # Shorter tracebacks
  
  # Custom test markers
  markers =
      slow: marks tests as slow (deselect with '-m "not slow"')
      integration: marks tests as integration tests
      e2e: marks tests as end-to-end tests
      requires_model: marks tests that need real Whisper model
  ```

**3. `.coveragerc`** (at project root: `/home/mikekao/personal/subgen/.coveragerc`)
- **Purpose**: Configures what code to measure and how to report coverage
- **Why**: Prevents measuring test files themselves and excludes boilerplate code
- **Contents**:
  ```ini
  [run]
  source = .
  omit = 
      tests/*
      launcher.py
      .venv/*
      venv/*
      */site-packages/*
      */dist-packages/*
      */__pycache__/*
  
  [report]
  # Don't complain about these lines in coverage
  exclude_lines =
      pragma: no cover
      def __repr__
      raise AssertionError
      raise NotImplementedError
      if __name__ == .__main__.:
      if TYPE_CHECKING:
      @abstractmethod
      @abc.abstractmethod
  
  # Show files with 100% coverage too
  skip_covered = False
  
  # Fail if coverage below 60%
  fail_under = 60
  ```

**4. `tests/` directory structure**
- **Purpose**: Organizes tests by type (unit, integration, e2e)
- **Location**: `/home/mikekao/personal/subgen/tests/`
- **Structure**:
  ```
  tests/
  ├── __init__.py                  # Makes tests a package (empty file)
  ├── conftest.py                  # Shared fixtures (see below)
  ├── unit/                        # Fast tests, no I/O
  │   ├── __init__.py
  │   └── test_example.py          # Example test (this story)
  ├── integration/                 # Test multiple components
  │   └── __init__.py
  ├── e2e/                         # Full pipeline tests
  │   └── __init__.py
  └── fixtures/                    # Test data
      ├── audio/                   # Sample audio files
      ├── video/                   # Sample video files
      └── webhooks/                # Sample webhook JSON
  ```

**5. `tests/conftest.py`** (MOST IMPORTANT FILE)
- **Purpose**: Shared pytest fixtures used by all test files
- **Location**: `/home/mikekao/personal/subgen/tests/conftest.py`
- **Why conftest.py**: pytest automatically loads fixtures from this file
- **Full contents** (copy this exactly):

```python
"""
Shared pytest fixtures for Subgen tests.

This file is automatically discovered by pytest and makes fixtures
available to all test files without needing to import them.
"""

import pytest
from unittest.mock import Mock, MagicMock, patch
from fastapi.testclient import TestClient
from language_code import LanguageCode
import numpy as np
import io


@pytest.fixture
def mock_whisper_model():
    """
    Mock Whisper model that doesn't load real model weights.
    
    Returns a mock object with transcribe() method that returns
    fake transcription results without actually running inference.
    
    Usage in tests:
        def test_something(mock_whisper_model):
            result = mock_whisper_model.transcribe("audio_data")
            assert result.language == "English"
    """
    model = Mock()
    
    # Mock the transcribe method
    mock_result = Mock()
    mock_result.language = "English"
    mock_result.segments = [
        Mock(
            start=0.0,
            end=2.0,
            text="This is a test transcription",
            id=0
        ),
        Mock(
            start=2.0,
            end=4.5,
            text="with multiple segments",
            id=1
        )
    ]
    mock_result.to_srt_vtt = Mock(return_value=None)
    
    model.transcribe = Mock(return_value=mock_result)
    
    return model


@pytest.fixture
def test_client():
    """
    FastAPI TestClient for testing HTTP endpoints.
    
    Returns a client that can make HTTP requests to the app
    without starting a real server.
    
    Usage in tests:
        def test_webhook(test_client):
            response = test_client.post("/plex", ...)
            assert response.status_code == 200
    """
    # Import app here to avoid circular imports
    from subgen import app
    return TestClient(app)


@pytest.fixture
def sample_audio_bytes():
    """
    Generate 1 second of silence as audio data.
    
    Returns 16kHz, 16-bit PCM audio (standard for Whisper).
    Useful for testing audio processing without real audio files.
    
    Usage in tests:
        def test_audio_processing(sample_audio_bytes):
            result = process_audio(sample_audio_bytes)
            assert len(result) > 0
    """
    # 1 second of silence at 16kHz, 16-bit PCM
    audio_array = np.zeros(16000, dtype=np.int16)
    return audio_array.tobytes()


@pytest.fixture
def plex_webhook_payload():
    """
    Sample Plex webhook JSON payload.
    
    Returns a dictionary matching Plex's webhook format for
    a "library.new" event (new episode added).
    
    Usage in tests:
        def test_plex_webhook(test_client, plex_webhook_payload):
            import json
            response = test_client.post(
                "/plex",
                data={"payload": json.dumps(plex_webhook_payload)},
                headers={"User-Agent": "PlexMediaServer/1.0"}
            )
            assert response.status_code == 200
    """
    return {
        "event": "library.new",
        "Metadata": {
            "ratingKey": "12345",
            "type": "episode",
            "title": "Test Episode",
            "grandparentTitle": "Test Show",
            "Media": [
                {
                    "Part": [
                        {
                            "file": "/media/TV/Show/Season 01/S01E01.mkv"
                        }
                    ]
                }
            ]
        }
    }


@pytest.fixture
def jellyfin_webhook_payload():
    """
    Sample Jellyfin webhook payload.
    
    Returns form data matching Jellyfin's webhook format for
    an "ItemAdded" event.
    
    Usage in tests:
        def test_jellyfin_webhook(test_client, jellyfin_webhook_payload):
            response = test_client.post(
                "/jellyfin",
                data=jellyfin_webhook_payload,
                headers={"User-Agent": "Jellyfin-Server/1.0"}
            )
            assert response.status_code == 200
    """
    return {
        "NotificationType": "ItemAdded",
        "ItemId": "abc123def456",
        "file": "/media/TV/Show/Season 01/S01E01.mkv"
    }


@pytest.fixture
def temp_media_file(tmp_path):
    """
    Create a temporary media file for testing.
    
    Uses pytest's tmp_path fixture to create a temporary directory
    that's automatically cleaned up after the test.
    
    Args:
        tmp_path: pytest fixture providing temporary directory
    
    Returns:
        str: Absolute path to temporary file
    
    Usage in tests:
        def test_file_processing(temp_media_file):
            result = process_video(temp_media_file)
            assert result is not None
    """
    media_file = tmp_path / "test_video.mp4"
    media_file.write_bytes(b"fake video data")
    return str(media_file)


@pytest.fixture
def mock_language_code():
    """
    Returns a sample LanguageCode for testing.
    
    Usage in tests:
        def test_language_handling(mock_language_code):
            assert mock_language_code.to_iso_639_1() == "en"
    """
    return LanguageCode.ENGLISH


@pytest.fixture(autouse=True)
def reset_global_state():
    """
    Automatically runs before each test to reset global state.
    
    The autouse=True means this runs for EVERY test without
    explicitly requesting it.
    
    This prevents tests from affecting each other by:
    - Clearing the task queue
    - Clearing the task_results dictionary
    - Resetting any other global state
    
    Usage: Automatic, no need to request in test functions
    """
    # Setup: Clear state before test
    # (In future stories, add code here to clear task_queue, task_results, etc.)
    
    yield  # Test runs here
    
    # Teardown: Clean up after test
    # (In future stories, add cleanup code here)
```

**6. `tests/unit/test_example.py`** (Example test to prove pytest works)
- **Purpose**: Demonstrates that pytest infrastructure works
- **Location**: `/home/mikekao/personal/subgen/tests/unit/test_example.py`
- **Full contents**:

```python
"""
Example test file to verify pytest infrastructure is working.

This file contains simple tests that should pass immediately
to prove that pytest, fixtures, and mocks are configured correctly.
"""

import pytest
from language_code import LanguageCode


def test_pytest_is_working():
    """
    Simplest possible test - just checks that pytest runs.
    
    If this fails, pytest itself is broken.
    """
    assert True


def test_language_code_import():
    """
    Test that we can import LanguageCode from language_code.py.
    
    This verifies that the module is importable and the Enum works.
    """
    assert LanguageCode.ENGLISH is not None
    assert LanguageCode.ENGLISH.to_iso_639_1() == "en"


def test_mock_whisper_model_fixture(mock_whisper_model):
    """
    Test that mock Whisper model fixture works.
    
    Args:
        mock_whisper_model: Fixture from conftest.py
    
    This verifies that:
    1. The fixture loads
    2. The mock has transcribe() method
    3. The mock returns expected structure
    """
    result = mock_whisper_model.transcribe("fake_audio_data")
    
    assert result is not None
    assert result.language == "English"
    assert len(result.segments) == 2
    assert result.segments[0].text == "This is a test transcription"


def test_sample_audio_bytes_fixture(sample_audio_bytes):
    """
    Test that sample audio fixture generates correct data.
    
    Args:
        sample_audio_bytes: Fixture from conftest.py
    
    Verifies that audio data is correct length (1 second at 16kHz)
    """
    # 1 second at 16kHz, 16-bit (2 bytes per sample) = 32000 bytes
    expected_length = 16000 * 2
    assert len(sample_audio_bytes) == expected_length


def test_plex_webhook_payload_fixture(plex_webhook_payload):
    """
    Test that Plex webhook payload fixture has correct structure.
    
    Args:
        plex_webhook_payload: Fixture from conftest.py
    
    Verifies the payload matches Plex's webhook format.
    """
    assert "event" in plex_webhook_payload
    assert plex_webhook_payload["event"] == "library.new"
    assert "Metadata" in plex_webhook_payload
    assert "ratingKey" in plex_webhook_payload["Metadata"]
```

**7. Empty `__init__.py` files**
Create empty files at these locations (makes directories into Python packages):
- `/home/mikekao/personal/subgen/tests/__init__.py`
- `/home/mikekao/personal/subgen/tests/unit/__init__.py`
- `/home/mikekao/personal/subgen/tests/integration/__init__.py`
- `/home/mikekao/personal/subgen/tests/e2e/__init__.py`

---

### Integration Points

**Integration Point 1: language_code.py**
- **Location**: `/home/mikekao/personal/subgen/language_code.py`
- **Import statement**: `from language_code import LanguageCode`
- **Purpose**: Core enum used throughout the application for language handling
- **How Story Uses It**: Example test imports and tests LanguageCode.ENGLISH
- **Functions tested in this story**:
  - `LanguageCode.ENGLISH.to_iso_639_1()` - Returns `"en"`
  
**Integration Point 2: subgen.py FastAPI app**
- **Location**: `/home/mikekao/personal/subgen/subgen.py:2144` (entire file)
- **Import statement**: `from subgen import app`
- **Purpose**: Main FastAPI application with all webhook endpoints
- **How Story Uses It**: TestClient fixture wraps `app` for endpoint testing
- **Note**: In this story we only import it, we don't test endpoints yet (that's Story 05)

**Integration Point 3: pytest fixtures system**
- **Location**: `/home/mikekao/personal/subgen/tests/conftest.py`
- **Purpose**: Shared test data and mock objects
- **How Story Uses It**: All test files automatically have access to fixtures
- **Mechanism**: pytest discovers `conftest.py` and makes fixtures available

---

## Implementation Steps

Follow these steps **in exact order**:

### Step 1: Install pytest and dependencies

**Why**: Need testing tools before creating configuration files

**Commands**:
```bash
# Navigate to project root
cd /home/mikekao/personal/subgen

# Create requirements-dev.txt
cat > requirements-dev.txt << 'EOF'
# Testing Framework
pytest>=7.4.0
pytest-cov>=4.1.0
pytest-mock>=3.11.0
pytest-asyncio>=0.21.0

# HTTP Testing
httpx>=0.24.0
EOF

# Install development dependencies
pip install -r requirements-dev.txt

# Verify installation
pytest --version
```

**Expected output**: `pytest 7.4.x` or higher

### Step 2: Create pytest.ini configuration

**Why**: Tells pytest where to find tests and what options to use

**Commands**:
```bash
cat > pytest.ini << 'EOF'
[pytest]
testpaths = tests
python_files = test_*.py
python_classes = Test*
python_functions = test_*
addopts = 
    -v
    --strict-markers
    --cov=.
    --cov-report=term-missing
    --cov-report=html
    --cov-report=xml
    --tb=short
markers =
    slow: marks tests as slow (deselect with '-m "not slow"')
    integration: marks tests as integration tests
    e2e: marks tests as end-to-end tests
    requires_model: marks tests that need real Whisper model
EOF
```

### Step 3: Create .coveragerc configuration

**Why**: Configures code coverage measurement

**Commands**:
```bash
cat > .coveragerc << 'EOF'
[run]
source = .
omit = 
    tests/*
    launcher.py
    .venv/*
    venv/*
    */site-packages/*
    */dist-packages/*
    */__pycache__/*

[report]
exclude_lines =
    pragma: no cover
    def __repr__
    raise AssertionError
    raise NotImplementedError
    if __name__ == .__main__.:
    if TYPE_CHECKING:
    @abstractmethod
    @abc.abstractmethod
skip_covered = False
fail_under = 60
EOF
```

### Step 4: Create test directory structure

**Why**: Organizes tests into unit/integration/e2e categories

**Commands**:
```bash
# Create directories
mkdir -p tests/unit
mkdir -p tests/integration
mkdir -p tests/e2e
mkdir -p tests/fixtures/audio
mkdir -p tests/fixtures/video
mkdir -p tests/fixtures/webhooks

# Create __init__.py files (makes them Python packages)
touch tests/__init__.py
touch tests/unit/__init__.py
touch tests/integration/__init__.py
touch tests/e2e/__init__.py

# Verify structure
ls -R tests/
```

**Expected output**:
```
tests/:
__init__.py  conftest.py  e2e/  fixtures/  integration/  unit/

tests/e2e:
__init__.py

tests/fixtures:
audio/  video/  webhooks/

tests/integration:
__init__.py

tests/unit:
__init__.py  test_example.py
```

### Step 5: Create conftest.py with fixtures

**Why**: Provides reusable test fixtures to all test files

**Commands**:
```bash
# Copy the conftest.py content from "Files to Create" section above
# Save it to tests/conftest.py
# (Use the full code from the Technical Design section)
```

**Verification**:
```bash
# Check file exists and is valid Python
python3 -m py_compile tests/conftest.py
echo "✓ conftest.py syntax is valid"
```

### Step 6: Create example test file

**Why**: Proves pytest infrastructure works before writing real tests

**Commands**:
```bash
# Copy the test_example.py content from "Files to Create" section above
# Save it to tests/unit/test_example.py
```

**Verification**:
```bash
python3 -m py_compile tests/unit/test_example.py
echo "✓ test_example.py syntax is valid"
```

### Step 7: Run tests to verify infrastructure works

**Why**: Confirms everything is configured correctly

**Commands**:
```bash
# Run all tests with verbose output
pytest tests/ -v

# Run with coverage report
pytest tests/ --cov=. --cov-report=term-missing

# Run only unit tests
pytest tests/unit/ -v
```

**Expected output**:
```
tests/unit/test_example.py::test_pytest_is_working PASSED
tests/unit/test_example.py::test_language_code_import PASSED
tests/unit/test_example.py::test_mock_whisper_model_fixture PASSED
tests/unit/test_example.py::test_sample_audio_bytes_fixture PASSED
tests/unit/test_example.py::test_plex_webhook_payload_fixture PASSED

===================== 5 passed in 0.12s ======================
```

### Step 8: Verify coverage report generation

**Why**: Ensures coverage tracking works for future stories

**Commands**:
```bash
# Generate HTML coverage report
pytest tests/ --cov=. --cov-report=html

# Open in browser (optional)
open htmlcov/index.html  # macOS
xdg-open htmlcov/index.html  # Linux
```

**Expected result**: Coverage report shows language_code.py with some coverage

### Step 9: Update .gitignore

**Why**: Don't commit coverage reports and cache files

**Commands**:
```bash
# Add to .gitignore if not already present
cat >> .gitignore << 'EOF'

# Testing
htmlcov/
.coverage
.pytest_cache/
*.pyc
__pycache__/
.tox/
EOF
```

### Step 10: Create work log

**Why**: Documents what was done and any issues encountered

**Template**:
```markdown
# Work Log: EPIC_00 Story 01 - pytest Setup

**Date**: 2026-02-15
**Story**: EPIC_00 / STORY_01
**Time Spent**: X hours

## What Was Done

- Created `requirements-dev.txt` with pytest and 4 plugins
- Created `pytest.ini` with test discovery and coverage config
- Created `.coveragerc` with coverage rules
- Created test directory structure (unit/integration/e2e)
- Created `tests/conftest.py` with 8 fixtures
- Created example test file with 5 passing tests
- Verified all tests pass
- Generated coverage report

## Files Created

- `requirements-dev.txt` - 9 lines
- `pytest.ini` - 20 lines
- `.coveragerc` - 24 lines
- `tests/conftest.py` - 180 lines
- `tests/unit/test_example.py` - 75 lines
- `tests/__init__.py` - empty
- `tests/unit/__init__.py` - empty
- `tests/integration/__init__.py` - empty
- `tests/e2e/__init__.py` - empty

## Test Results

```
pytest tests/ -v
======================== 5 passed in 0.12s ========================
```

## Issues Encountered

- [None / or describe any issues]

## Next Steps

- Story 02: Add unit tests for language_code.py
- All 8 fixtures are ready to use
- Test infrastructure is ready for all future testing stories
```

---

## Definition of Done

- [ ] **All files created** as specified in Technical Design
- [ ] **5+ tests passing** when running `pytest tests/unit/test_example.py -v`
- [ ] **Coverage report generates** with `pytest --cov=. --cov-report=html`
- [ ] **No syntax errors** in any Python files (`python3 -m py_compile` succeeds)
- [ ] **pytest.ini** configures test discovery correctly
- [ ] **.coveragerc** excludes test files from coverage
- [ ] **conftest.py** contains 8 working fixtures
- [ ] **Work log created** at `docs/WORKLOGS/NNNN_2026-02-15_EPIC_00_story_01_pytest_setup.md`
- [ ] **Code committed** with message: "EPIC_00 STORY_01: Set up pytest infrastructure"
- [ ] **Next story ready**: Story 02 can now write tests using these fixtures

---

## Dependencies

**Depends On**: None (first story in epic)  
**Blocks**: All other EPIC_00 stories (02-10)

---

## Notes

### Troubleshooting

**Problem**: `pytest: command not found`
**Solution**: Run `pip install -r requirements-dev.txt`

**Problem**: `ImportError: cannot import name 'app' from 'subgen'`
**Solution**: Run pytest from project root: `cd /home/mikekao/personal/subgen && pytest`

**Problem**: Tests pass but coverage is 0%
**Solution**: Check `.coveragerc` source path is `.` (current directory)

### Tips for Fresh College Grads

1. **Always run pytest from project root**: `/home/mikekao/personal/subgen`
2. **Read fixture docstrings**: They explain what each fixture does
3. **Use `-v` flag**: Shows which tests pass/fail individually
4. **Use `--tb=short`**: Shorter error messages (already in pytest.ini)
5. **Test fixtures first**: Run `test_example.py` before writing new tests

### References

- pytest docs: https://docs.pytest.org/
- Fixtures guide: https://docs.pytest.org/en/stable/fixture.html
- Coverage.py: https://coverage.readthedocs.io/
- FastAPI testing: https://fastapi.tiangolo.com/tutorial/testing/

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15  
**Status**: Ready for Implementation
