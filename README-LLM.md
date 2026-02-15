# Subgen Documentation - Complete LLM Starting Point

**This is the ONLY document you need to read to start development on Subgen.**

All essential information is consolidated here. Additional docs are referenced for deep dives only.

---

## 🎯 Project Overview

**Subgen** is a Python-based automatic subtitle generation service that uses OpenAI's Whisper (via faster-whisper and stable-ts) to transcribe audio from media files into SRT/LRC subtitle files.

### Core Purpose
- Automatically generate subtitles for personal media libraries (Plex, Jellyfin, Emby)
- Support Bazarr as a Whisper provider for subtitle automation
- Transcribe or translate audio to English subtitles
- Handle multiple languages with configurable detection and forcing
- Support both GPU (CUDA) and CPU transcription

### Version
- Current version: 2026.02.9 (CalVer: YYYY.MM.commits_this_month)
- Version auto-updates via GitHub Actions on every commit

---

## 🚀 NEW: Hybrid Architecture Design Documents

**NEW ARCHITECTURE AVAILABLE:** Complete redesign documentation for hybrid Go/Python system.

### Quick Links

- **[00_HYBRID_ARCHITECTURE.md](./docs/DESIGN/00_HYBRID_ARCHITECTURE.md)** - System architecture overview
- **[01_GRPC_PROTOCOL.md](./docs/DESIGN/01_GRPC_PROTOCOL.md)** - gRPC communication protocol
- **[02_MEMORY_MANAGEMENT.md](./docs/DESIGN/02_MEMORY_MANAGEMENT.md)** - Memory leak prevention (CRITICAL)
- **[03_SCALING_STRATEGY.md](./docs/DESIGN/03_SCALING_STRATEGY.md)** - Phase 1 → Phase 2 scaling
- **[04_K8S_DEPLOYMENT.md](./docs/DESIGN/04_K8S_DEPLOYMENT.md)** - Kubernetes deployment with bjw-s

### Implementation Epics

- **[EPIC_01: Go Orchestrator Core](./docs/BACKLOG/EPIC_01/README.md)** - 56-72h, 8 stories
- **[EPIC_02: Python Worker Refactor](./docs/BACKLOG/EPIC_02/README.md)** - 34-44h, 5 stories
- **[EPIC_03: Integration & Testing](./docs/BACKLOG/EPIC_03/README.md)** - 34-44h, 5 stories
- **[EPIC_04: K8s Deployment (bjw-s)](./docs/BACKLOG/EPIC_04/README.md)** - 12-17h, 3 stories
- **[EPIC_05: Migration & Cutover](./docs/BACKLOG/EPIC_05/README.md)** - 18-26h, 4 stories

### Key Benefits

✅ **Memory Leaks Fixed** - All 3 confirmed leaks eliminated  
✅ **Horizontal Scaling** - Phase 1 (single pod) → Phase 2 (multi-worker) with zero code changes  
✅ **Production Ready** - Prometheus metrics, structured logging, health checks  
✅ **Kubernetes Native** - bjw-s app-template deployment (no custom Helm)  
✅ **Comprehensive Testing** - TDD approach, 70%+ coverage, memory leak tests

### Architecture Comparison

| Aspect | Current (subgen.py) | New (Hybrid Go/Python) |
|--------|---------------------|------------------------|
| **Language** | Python only | Go (orchestrator) + Python (worker) |
| **Lines of Code** | 2,144 (monolith) | ~800 (Go) + ~600 (Python) |
| **Memory Leaks** | 3 confirmed | 0 (fixed + tested) |
| **Scalability** | Single process | Horizontal (1-N workers) |
| **Deployment** | Docker only | Kubernetes (bjw-s chart) |
| **Observability** | Logs only | Prometheus + structured logs |
| **Testing** | None (0 tests) | TDD (70%+ coverage) |

---

## 📁 Repository Structure

```
subgen/
├── subgen.py           # Main application (2144 lines) - FastAPI server + transcription logic
├── launcher.py         # Bootstrap script (183 lines) - handles updates, setup, environment
├── language_code.py    # Language code management (199 lines) - enum-based language handling
├── requirements.txt    # Python dependencies (10 packages)
├── subgen.env          # Default environment variables
├── Dockerfile          # GPU-enabled Docker image (CUDA 12.3.2)
├── Dockerfile.cpu      # CPU-only Docker image (multi-stage build for smaller size)
├── docker-compose.yml  # Example Docker Compose configuration
├── entrypoint.sh       # Docker entrypoint with rootless support
├── subgen.xml          # Unraid template configuration
├── .github/workflows/  # CI/CD pipelines
│   ├── build_GPU.yml   # Auto-build GPU Docker image
│   ├── build_CPU.yml   # Auto-build CPU Docker image (multi-arch: amd64, arm64)
│   └── calver.yml      # Auto-update version on every commit
└── README.md           # User-facing documentation
```

---

## 🏗️ Architecture Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    FastAPI Web Server                        │
│  Endpoints: /plex, /jellyfin, /emby, /tautulli, /asr,       │
│             /detect-language, /batch, /status                │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│              Deduplicated Priority Queue                     │
│  Priority: 0=Detect Language, 1=ASR, 2=Transcribe          │
│  Deduplication: By file path or audio hash                  │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│           Worker Threads (CONCURRENT_TRANSCRIPTIONS)         │
│  Each thread: fetch task → transcribe → cleanup             │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│         Whisper Model (faster-whisper + stable-ts)           │
│  Device: CPU or GPU (CUDA)                                   │
│  Models: tiny, base, small, medium, large, distil variants  │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                  Output: SRT or LRC Files                    │
│  Location: Same directory as source media                    │
│  Naming: {filename}.{model}.{language}.srt                   │
└─────────────────────────────────────────────────────────────┘
```

### Component Breakdown

#### 1. FastAPI Web Server (subgen.py:203-803)
- **Purpose**: Receives webhook events from media servers and ASR requests
- **Endpoints**:
  - `/plex`: Plex webhook handler (library.new, media.play events)
  - `/jellyfin`: Jellyfin webhook handler (ItemAdded, PlaybackStart events)
  - `/emby`: Emby webhook handler (library.new, playback.start events)
  - `/tautulli`: Tautulli webhook handler (added, played events)
  - `/asr`: Bazarr Whisper provider endpoint (blocking, returns SRT via streaming)
  - `/detect-language`: Language detection endpoint (blocking, returns JSON)
  - `/batch`: Manual transcription of files/folders
  - `/status`: Version and component info

#### 2. Deduplicated Priority Queue (subgen.py:272-324)
- **Class**: `DeduplicatedQueue(queue.PriorityQueue)`
- **Deduplication**: Tracks `_queued` and `_processing` sets by task ID (file path or audio hash)
- **Priority Levels**:
  - Priority 0: Language detection (highest priority)
  - Priority 1: ASR requests (time-sensitive, Bazarr)
  - Priority 2: Standard transcription (lowest priority)
- **Thread-safe**: Uses Lock for set operations
- **Status Tracking**: `is_idle()`, `is_active(task_id)`, `get_queued_tasks()`, `get_processing_tasks()`

#### 3. Worker Thread Pool (subgen.py:332-393)
- **Count**: Configured by `CONCURRENT_TRANSCRIPTIONS` (default: 2)
- **Behavior**: 
  - Daemon threads that run forever
  - Fetch tasks from queue with 1-second timeout
  - Execute transcription via `gen_subtitles()`, `asr_task_worker()`, or `detect_language_task()`
  - Refresh metadata after completion (Plex/Jellyfin)
  - Mark task done and schedule model cleanup
- **Logging**: Structured logs with WORKER START/FINISH, processing/queued counts, elapsed time

#### 4. Whisper Model Management (subgen.py:1143-1214)
- **Lazy Loading**: Model only loaded when needed via `start_model()`
- **Cleanup**: Delayed cleanup after `MODEL_CLEANUP_DELAY` seconds (default: 30s)
- **VRAM Management**: 
  - Unloads model when queue is idle
  - Clears CUDA cache if GPU enabled
  - Garbage collection and malloc_trim on Linux
- **Thread-safe**: Uses `model_cleanup_lock` to prevent race conditions
- **Models**: Loaded from `MODEL_PATH` (default: ./models)

#### 5. Language Code System (language_code.py)
- **Enum-based**: Each language has 5 attributes
  - `iso_639_1`: 2-letter code (e.g., "en")
  - `iso_639_2_t`: 3-letter terminological code (e.g., "eng")
  - `iso_639_2_b`: 3-letter bibliographic code (e.g., "eng")
  - `name_en`: English name (e.g., "English")
  - `name_native`: Native name (e.g., "English")
- **Conversion Methods**: 
  - `from_iso_639_1(code)`, `from_iso_639_2(code)`, `from_name(name)`, `from_string(value)`
  - `to_iso_639_1()`, `to_iso_639_2_t()`, `to_iso_639_2_b()`, `to_name(in_english=True)`
- **Special Values**: `LanguageCode.NONE` for unknown/undefined languages
- **Total Languages**: 104 languages supported

#### 6. Launcher System (launcher.py)
- **Purpose**: Bootstrap script for both standalone and Docker deployments
- **Features**:
  - Auto-update subgen.py and language_code.py from GitHub
  - Install/update dependencies from requirements.txt
  - Bazarr setup wizard (prompts for common configuration)
  - Environment variable management (CLI args → .env file → external env vars)
  - Branch selection support (default: main)
- **Arguments**:
  - `-d/--debug`: Enable debug mode
  - `-i/--install`: Install/update packages
  - `-a/--append`: Append "Transcribed by whisper" to subtitles
  - `-u/--update`: Update subgen.py from GitHub
  - `-x/--exit-early`: Exit without running subgen.py
  - `-s/--setup-bazarr`: Run Bazarr configuration wizard
  - `-b/--branch`: Specify GitHub branch (default: main)
  - `-l/--launcher-update`: Update launcher.py and re-launch

---

## 🔑 Critical Design Decisions

### 1. Webhook-Driven Architecture
**Decision**: Use webhooks from media servers instead of polling or filesystem watching
**Rationale**:
- Real-time response to new media or playback events
- Lower resource usage compared to polling
- Integrates seamlessly with existing media server workflows
**Trade-offs**:
- Requires configuration in media server
- Relies on media server sending correct webhooks

### 2. Deduplicated Priority Queue
**Decision**: Implement custom priority queue with deduplication and status tracking
**Rationale**:
- Prevents duplicate work when same file queued multiple times
- Language detection needs higher priority (fast, needed for transcription)
- ASR requests need immediate processing (Bazarr timeout constraints)
- Standard transcription can wait in queue
**Implementation Details**:
- Uses file path as task ID for file-based tasks
- Uses audio hash (SHA256) as task ID for ASR requests
- Tracks both queued and processing tasks separately
- Priority 0 > 1 > 2 (lower number = higher priority)

### 3. Lazy Model Loading with Delayed Cleanup
**Decision**: Load model on-demand and delay cleanup by 30 seconds
**Rationale**:
- Model loading is expensive (1-5 seconds)
- Consecutive requests often arrive in bursts
- 30-second delay allows model reuse without constant reload
- Prevents thrashing when processing multiple files
**VRAM Management**:
- Only clears VRAM when queue is completely idle
- Prevents premature cleanup during batch processing
- Configurable via `MODEL_CLEANUP_DELAY` and `CLEAR_VRAM_ON_COMPLETE`

### 4. Audio Hash-Based Deduplication for ASR
**Decision**: Use SHA256 hash of audio content + task + language for deduplication
**Rationale**:
- Bazarr may send identical audio multiple times
- File path not available for uploaded audio
- Hash ensures exact same audio + parameters = same task ID
**Implementation**: `generate_audio_hash(audio_content, task, language)` → 16-char hash

### 5. Blocking ASR Endpoint with Result Storage
**Decision**: ASR endpoint blocks until transcription completes, returns streaming response
**Rationale**:
- Bazarr expects synchronous response with subtitle content
- Cannot use async/callback pattern
- Must respect `CONCURRENT_TRANSCRIPTIONS` limit
**Implementation**:
- `TaskResult` class with `done` event for blocking
- `task_results` dict maps task_id → TaskResult
- Worker sets result when complete
- Endpoint waits with timeout (default: 5 hours)

### 6. Backwards Compatible Environment Variables
**Decision**: Support both old and new environment variable names
**Rationale**:
- Avoid breaking existing user configurations
- Migrate to cleaner naming convention over time
- New name takes precedence over old name
**Example Mappings**:
- `PLEX_TOKEN` → `PLEXTOKEN`
- `PROCESS_ADDED_MEDIA` → `PROCADDEDMEDIA`
- `SUBTITLE_LANGUAGE_NAME` → `NAMESUBLANG`

### 7. Multi-Stage Docker Build for CPU Image
**Decision**: Use 2-stage Docker build for CPU-only image
**Rationale**:
- Stage 1: Build dependencies (heavy build tools)
- Stage 2: Runtime-only dependencies (smaller image)
- CPU image reduced by ~50% compared to single-stage
- Faster pull times for users
**Trade-off**: Longer build time, but worth it for distribution

### 8. Rootless Docker Support
**Decision**: Support running container as non-root user via PUID/PGID
**Rationale**:
- Security: Principle of least privilege
- Permissions: Match host user for file access
- Compatibility: Works with Podman, Kubernetes, OpenShift
**Implementation**: `entrypoint.sh` with gosu for privilege dropping

### 9. CalVer Versioning via GitHub Actions
**Decision**: Auto-increment version on every commit using CalVer (YYYY.MM.commits_this_month)
**Rationale**:
- Eliminates manual version bumps
- Clear indication of how recent the code is
- Commit count shows activity level
**Implementation**: `calver.yml` workflow amends commit with new version

### 10. No Testing Infrastructure (Observation)
**Finding**: Repository has ZERO test files (no pytest, unittest, integration tests)
**Implications**:
- All testing is manual / in-production
- Risky for refactoring or adding features
- Heavy reliance on user bug reports
**Evidence**: `find . -name "*test*.py"` returns nothing

---

## ⚠️ CRITICAL RULES - READ BEFORE CODING

### 0. MANDATORY WORK LOGS (ALWAYS REQUIRED)

**EVERY task, story, or significant work session MUST create a work log before completion.**

**A task is NOT complete without a work log. No exceptions.**

- ✅ Create work log at end of EVERY task
- ✅ Create work log for delegated subtasks
- ✅ Create work log for feature implementations
- ✅ Create work log for bug fixes
- ✅ Document what was done, test results, any issues
- ✅ Commit work log with code changes

**Format**: `NNNN_YYYY-MM-DD_description.md` in `docs/WORKLOGS/`
**See**: "Work Log Directory" section below for complete requirements

### 1. Test-Driven Development (MANDATORY)

**Write tests BEFORE code, ALWAYS. No exceptions.**

```python
# 1. Write test FIRST (must fail initially)
def test_audio_hash_generation():
    audio = b"audio data"
    hash1 = generate_audio_hash(audio, "transcribe", "en")
    assert hash1 == generate_audio_hash(audio, "transcribe", "en")

# 2. Then implement to make test pass
def generate_audio_hash(audio_content: bytes, task: str = None, language: str = None) -> str:
    # implementation
```

**Requirements:**
- Multiple happy path tests (3-5 scenarios)
- Multiple unhappy path tests (error cases, edge cases)
- All tests must pass before task is complete
- Use pytest with appropriate fixtures

**CRITICAL: IMPLEMENT TEST INFRASTRUCTURE FIRST**

This project currently has ZERO tests. Before implementing ANY new features:
1. Set up pytest infrastructure
2. Add test runner to CI/CD
3. Create test fixtures for common scenarios
4. Add integration tests for webhooks

### 2. Type Safety (MANDATORY)

**ALWAYS use type hints for function signatures and complex data structures.**

```python
# ✅ REQUIRED
def gen_subtitles(file_path: str, transcription_type: str, force_language: LanguageCode = LanguageCode.NONE) -> None:
    pass

# ✅ REQUIRED
class DeduplicatedQueue(queue.PriorityQueue):
    def __init__(self) -> None:
        self._queued: set[str] = set()
        self._processing: set[str] = set()

# ❌ FORBIDDEN
def gen_subtitles(file_path, transcription_type, force_language=None):
    pass
```

**Rules:**
1. All function signatures must have type hints
2. All class attributes must have type hints
3. Use `Optional[T]` for nullable types
4. Use `Union[A, B]` sparingly, prefer specific types
5. Validate all inputs explicitly

### 3. Complete Implementation (MANDATORY)

**NO TODOs, NO stubs, NO placeholders.**

```python
# ❌ FORBIDDEN
limit_to_preferred_audio_languages = convert_to_bool(os.getenv('LIMIT_TO_PREFERRED_AUDIO_LANGUAGE', False))
#TODO: add support for this  # DON'T DO THIS!

# ✅ REQUIRED - Implement completely or remove the variable
```

If you can't implement something completely, document it in:
- `docs/BACKLOG/EPIC_XX/stories/STORY_XX.md`
- Create an issue on GitHub
- Add to roadmap

### 4. Ask Before Deciding (MANDATORY)

**Never assume deployment scenarios, performance requirements, or user preferences.**

When uncertain:
- State assumptions with confidence level (LOW/MEDIUM/HIGH)
- Cite specific evidence from code or docs
- Identify gaps in knowledge
- Ask user for confirmation

### 5. Code Quality Standards

- **Comprehensive docstrings** - All functions must have docstrings
- **No technical debt** - Implement full solutions
- **Consistent naming** - Use snake_case for Python (PEP 8)
- **Follow existing patterns** - Consistency over personal preference
- **Be neutral and factual** - No superlatives

### 6. Never Edit Production Code Without Tests

**Before modifying any existing function:**
1. Write tests that cover current behavior
2. Ensure tests pass with current implementation
3. Make your changes
4. Verify tests still pass (or update tests if behavior intentionally changed)

**This prevents regressions and documents expected behavior.**

---

## 🔄 Development Workflow Guide

### Overview

This section defines two distinct agent roles and their workflows for collaborative development on Subgen.

**IMPORTANT**: These workflows are MANDATORY when working on epics, stories, or multi-step tasks.

---

### Agent Role 1: Orchestrator Agent

**Purpose**: Coordinate multiple delegations to complete epics, stories, or complex multi-step tasks.

**When to Use**: 
- Working on Epic-level features
- User story implementation requiring multiple sub-tasks
- Complex refactoring or architectural changes
- Coordinating work across multiple code areas

#### Orchestrator Responsibilities

1. **Context Distribution**: Ensure ALL delegations have access to critical documentation
2. **Scope Definition**: Define clear boundaries, ownership, and integration points
3. **Quality Enforcement**: Validate work meets standards through code review and testing
4. **Gap Detection**: Identify and resolve integration gaps between sub-tasks
5. **Integration Validation**: Ensure all components work together end-to-end
6. **Testing Coordination**: Run comprehensive tests across entire codebase
7. **Work Log Management**: Create completion work logs documenting entire epic/story

#### Orchestrator Workflow (11-Step Process)

**CRITICAL**: Follow this workflow for ALL epic/story implementation tasks:

```
1. Context Setup
   └─> Delegate: "Read README-LLM.md, EPIC README, relevant design docs"
   └─> Include: Design constraints, architectural patterns, integration points
   └─> Define: Clear scope, ownership boundaries, expected deliverables

2. Implementation Delegation
   └─> Delegate: User story implementation with TDD requirements
   └─> Prompt Detail Level: "Fresh college grad seeing codebase for first time"
   └─> Include: Specific file references, pattern examples, testing requirements

3. Code Review Delegation
   └─> Delegate: Skeptical code reviewer to validate implementation
   └─> Focus: Integration points, test coverage, gap detection, code quality
   └─> Requirement: Only code + tests count as proof of work (NOT status updates)
   └─> Output: Detailed gap report with code references and fix recommendations

4. Gap Remediation
   └─> Delegate: Fix ALL gaps identified in review (no matter how minor)
   └─> Include: Specific gap descriptions, code locations, fix strategies
   └─> Validate: Each fix with targeted tests

5. Iterative Validation
   └─> Repeat Steps 2-4 until ZERO gaps remain
   └─> Acceptance Criteria: "Story complete in spirit AND letter"
   └─> No Compromises: All integration points validated, all tests passing

6. Manual Testing Validation
   └─> ALWAYS validate manually with real media files
   └─> Test with different media servers (Plex, Jellyfin, Emby)
   └─> Document test procedure and results
   └─> Capture evidence (logs, generated subtitle files)

7. Test Validation
   └─> Run ALL tests and fix ANY failures
   └─> Commands:
       - pytest tests/          # ALL tests must pass
       - python -m mypy subgen.py   # Type checking
   └─> NO TECH DEBT: Fix all failures regardless of relevance to current work
   └─> Zero Tolerance: No pre-existing failures acceptable

8. Commit and Push
   └─> git add .
   └─> git commit -m "Descriptive message referencing story/epic"
   └─> git push origin HEAD

9. Work Log Creation
   └─> Create work log in docs/WORKLOGS/ (see Work Log Directory section)
   └─> Format: NNNN_YYYY-MM-DD_story_completion_report.md
   └─> Content: Summary, implementation details, test results, next steps
   └─> Commit: Commit work log with code changes

10. Move to Next Story
    └─> Validate no implementation gaps between previous and current story
    └─> Common Pitfall: Previous story built/tested but never integrated into main code
    └─> If story file missing: Write it first before implementing
    └─> Repeat workflow from Step 1

11. Integration Gap Check
    └─> CRITICAL: Validate integration between stories
    └─> Ask: "Was previous story's code actually integrated into main codebase?"
    └─> Check: Import statements, function calls, initialization code
    └─> Test: End-to-end flow through new and existing code paths
```

#### Orchestrator Delegation Guidelines

**Prompt Quality Standards**:
- Detail level: "Instructions for fresh college grad seeing codebase for first time"
- Specificity: Include exact file paths, function names, pattern references
- Context: Provide architectural context, design decisions, trade-offs
- Boundaries: Clear scope limits, what's in/out of scope, integration points
- Examples: Reference similar implementations, established patterns

**Delegation Prompt Template**:
```
CONTEXT:
- Primary Doc: README-LLM.md (your bible)
- Epic/Story: [Reference to docs/BACKLOG/EPIC_XX/]
- Design Constraints: [TDD, type safety, modular design, etc.]

SCOPE:
- Objective: [Clear, specific goal]
- Boundaries: [What's included, what's excluded]
- Integration Points: [How this connects to existing code]
- Ownership: [Which files this delegation owns]

REQUIREMENTS:
- MUST read README-LLM.md
- MUST read [Epic/Story README]
- MUST follow TDD (tests first)
- MUST use type hints
- MUST validate integration points
- MUST create work log

DELIVERABLES:
1. [Specific deliverable 1 with acceptance criteria]
2. [Specific deliverable 2 with acceptance criteria]
3. [etc.]

SUCCESS CRITERIA:
- All tests passing (unit + integration)
- Type checking passes (mypy)
- Integration points validated
- Code follows established patterns
- Work log created
```

#### Orchestrator Principles

**RESPECT OTHER AGENTS**:
- Multiple agents may work simultaneously in same repository
- NEVER perform destructive git operations (git checkout ., git clean -fd)
- Define clear ownership boundaries to avoid conflicts
- Coordinate on shared resources (configs, global state)

**THOROUGHNESS**:
- Proof of work = code + tests, NOT status updates
- Integration points MUST be identified and updated
- Sufficient tests for happy/unhappy paths
- NO gaps acceptable, no matter how minor

**QUALITY GATES**:
- Code review before merge
- ALL tests passing before next story
- Manual validation before marking complete
- Work log created before task closure

**DESIGN PRINCIPLES** (Always Enforce):
- Test-Driven Development (TDD)
- Type safety with comprehensive type hints
- Modular design (avoid monolithic functions)
- Explicit error handling (never silently fail)
- Structured logging (use logging module, not print)

**PROPER FIXES ONLY**:
- ALWAYS use the proper fix
- NEVER use workarounds, hacks, or shortcuts
- You have INFINITE time and INFINITE resources
- Cost, time, token limits are NOT valid excuses for cutting corners

---

### Agent Role 2: Delegation Agent

**Purpose**: Execute specific, well-scoped tasks as part of a larger epic or story.

**When to Use**:
- Implementing a specific feature or module
- Writing tests for a component
- Code review of another agent's work
- Fixing a specific bug or gap
- Integrating a component into main codebase

#### Delegation Agent Responsibilities

1. **Context Acquisition**: Read ALL assigned documentation (README-LLM.md, Epic README)
2. **Scope Adherence**: Stay within defined boundaries, ask orchestrator if unclear
3. **Pattern Following**: Use established patterns, check similar implementations
4. **TDD Compliance**: Write tests FIRST, ensure they fail, then implement
5. **Integration Awareness**: Identify and document integration points
6. **Quality Standards**: Follow type safety, error handling, logging standards
7. **Work Log Creation**: Document work performed in work log (if completing a task)

#### Delegation Agent Workflow

**Standard Implementation Task**:
```
1. Read Required Documentation
   - README-LLM.md (MANDATORY - your bible)
   - Epic/Story README from docs/BACKLOG/EPIC_XX/
   - Relevant design documents

2. Understand Context
   - Review delegation prompt carefully
   - Identify scope boundaries
   - Note integration points
   - Check similar implementations in codebase

3. Plan Implementation
   - Break down into sub-tasks
   - Identify test scenarios (happy + unhappy paths)
   - Note which patterns to follow
   - Identify dependencies

4. Write Tests FIRST (TDD)
   - Unit tests (happy paths)
   - Unit tests (unhappy paths)
   - Integration tests
   - Tests MUST fail initially

5. Implement
   - Follow established patterns
   - Use type hints for all functions
   - Handle errors explicitly
   - Use structured logging (logging module, not print)
   - Avoid deep nesting

6. Validate
   - All tests pass (pytest)
   - Type checking passes (mypy)
   - Integration points work
   - Follow-up questions documented

7. Create Work Log (if task complete)
   - Document what was done
   - Include test results
   - Note any issues or follow-up
   - See Work Log Directory section

8. Report Back to Orchestrator
   - Clear completion status
   - Any gaps or uncertainties
   - Integration point validation status
   - Recommendations for next steps
```

**Code Review Task**:
```
1. Read Code with Skeptical Mindset
   - Assume nothing works until proven
   - Check every integration point
   - Verify test coverage (happy + unhappy)
   - Look for edge cases

2. Validate Against Standards
   - README-LLM.md rules followed?
   - TDD practiced (tests first)?
   - Type hints present?
   - Patterns followed correctly?
   - Error handling comprehensive?
   - Logging standards met?

3. Integration Point Analysis
   - Are ALL integration points identified?
   - Are they properly tested?
   - Do end-to-end flows work?
   - Are there hidden dependencies?

4. Test Coverage Review
   - Unit tests sufficient?
   - Integration tests present?
   - End-to-end tests adequate?
   - Edge cases covered?
   - Unhappy paths tested?

5. Gap Identification
   - Document EVERY gap (no matter how minor)
   - Provide code references for each gap
   - Explain WHY it's a gap
   - Recommend HOW to fix it

6. Report Generation
   - Clear gap descriptions
   - Severity assessment
   - Fix recommendations with code examples
   - Test recommendations
   - NO APPROVAL until all gaps fixed
```

#### Delegation Agent Principles

**READ FIRST, ASK LATER**:
- ALWAYS read README-LLM.md before ANY work
- ALWAYS read Epic/Story README
- If information exists in docs, don't ask orchestrator

**FOLLOW PATTERNS**:
- Check similar implementations in subgen.py
- Use LanguageCode enum for language handling
- Use structured logging (logging module)
- Follow FastAPI patterns for endpoints
- Maintain backwards compatibility

**TEST-DRIVEN DEVELOPMENT**:
- Tests BEFORE code, always
- Tests must fail initially
- Happy AND unhappy paths
- Integration tests mandatory

**QUALITY STANDARDS**:
- Type hints (comprehensive)
- Error handling (never silently fail)
- Logging (structured, with context)
- No TODOs or placeholders
- Complete implementations only

**COMMUNICATION**:
- Report completion clearly
- Document gaps/uncertainties
- Ask questions when scope unclear
- Provide recommendations for next steps

---

### Workflow Integration Summary

| Critical Rule | Orchestrator Enforcement | Delegation Compliance |
|--------------|-------------------------|---------------------|
| 0. Work Logs | Creates completion work log | Creates work log if task complete |
| 1. TDD | Validates tests-first in delegation | Writes tests before code |
| 2. Type Safety | Reviews for missing type hints | Uses type hints throughout |
| 3. Complete Implementation | No TODOs check in review | No placeholders/stubs |
| 4. Integration | Validates integration points | Documents integration needs |
| 5. Quality | Enforces code quality standards | Follows PEP 8, best practices |

---

## 📝 Work Log Directory (docs/WORKLOGS/) - MANDATORY

### 🚨 CRITICAL: Work Logs Are MANDATORY

**EVERY task, story, or significant work session MUST create a work log before completion.**

**A task is NOT complete without a work log. No exceptions.**

---

### File Naming Convention

**Format**: `NNNN_YYYY-MM-DD_description.md`

- **NNNN**: 4-digit sequence number (sequential, zero-padded)
- **YYYY-MM-DD**: ISO date when work was performed
- **description**: Brief descriptive name (snake_case)

**Examples:**
```
0001_2026-02-15_testing_infrastructure_setup.md
0002_2026-02-15_webhook_validation_implementation.md
0003_2026-02-15_EPIC_01_story_01_completion_report.md
```

**Get Next Sequence Number:**
```bash
cd docs/WORKLOGS
NEXT=$(printf "%04d" $(($(ls -1 [0-9][0-9][0-9][0-9]_*.md 2>/dev/null | sed 's/_.*//' | sort -n | tail -1) + 1)))
echo "Next work log: ${NEXT}_$(date +%Y-%m-%d)_description.md"
```

---

### Work Log Template

```markdown
# Work Log: [Task/Story Name]

**Date**: YYYY-MM-DD
**Author**: [Agent Name/ID]
**Epic/Story**: [Reference to docs/BACKLOG/EPIC_XX/]
**Status**: Complete / In Progress / Blocked

---

## Summary

[2-3 sentence summary of what was accomplished]

---

## Implementation Details

### Files Created/Modified
- `path/to/file.py` - [Brief description of changes]
- `path/to/test_file.py` - [Brief description of tests added]

### Key Changes
1. [Change 1]
2. [Change 2]
3. [Change 3]

### Design Decisions
- **Decision**: [What was decided]
- **Rationale**: [Why this approach]
- **Trade-offs**: [What alternatives were rejected and why]

---

## Testing

### Test Coverage
- Unit tests: X/Y passing
- Integration tests: X/Y passing
- Manual testing: [Results summary]

### Test Scenarios Covered
1. [Happy path scenario 1]
2. [Happy path scenario 2]
3. [Edge case 1]
4. [Error case 1]

---

## Issues Encountered

### [Issue 1]
- **Problem**: [Description]
- **Solution**: [How it was resolved]
- **Prevention**: [How to avoid in future]

---

## Next Steps

1. [Next action item 1]
2. [Next action item 2]

---

## Integration Points

- [Function/module 1] integrates with [component X] via [mechanism]
- [Function/module 2] called by [component Y]

---

## Commands for Validation

```bash
# Run tests
pytest tests/test_feature.py -v

# Type checking
python -m mypy subgen.py

# Manual testing
curl -X POST http://localhost:9000/endpoint -d '...'
```

---

## References

- Epic README: docs/BACKLOG/EPIC_XX/README.md
- Related work logs: 0XXX_YYYY-MM-DD_related_task.md
```

---

### For LLMs - Reading Work Logs

**Current Context** (Start here):
```bash
cd docs/WORKLOGS
ls -1 [0-9][0-9][0-9][0-9]_*.md | tail -10  # Last 10 logs
```

**Find Specific Topics**:
```bash
grep -l "webhook" [0-9][0-9][0-9][0-9]_*.md
grep -l "testing" [0-9][0-9][0-9][0-9]_*.md
grep -l "EPIC_01" [0-9][0-9][0-9][0-9]_*.md
```

---

## 📋 Backlog and Epic Structure (docs/BACKLOG/)

### Directory Structure

```
docs/
├── WORKLOGS/                    # Work logs (MANDATORY for all tasks)
│   ├── README.md               # Work log directory guide
│   └── NNNN_YYYY-MM-DD_*.md   # Individual work logs
├── BACKLOG/                    # Planning and epic tracking
│   ├── EPIC_00/                # Example: Testing infrastructure
│   │   ├── README.md          # Epic overview
│   │   └── stories/           # User stories for this epic
│   │       ├── STORY_01_pytest_setup.md
│   │       ├── STORY_02_unit_tests.md
│   │       └── STORY_03_integration_tests.md
│   ├── EPIC_01/                # Example: Refactor into modules
│   │   ├── README.md
│   │   └── stories/
│   └── [Future epics]
└── ARCHITECTURE/               # Architecture documentation (future)
```

### Epic Structure

Each epic directory (`EPIC_XX/`) must contain:

**1. README.md** (Epic Overview):
```markdown
# Epic XX: [Epic Name]

## Overview
[High-level description of what this epic achieves]

## Goals
1. [Goal 1]
2. [Goal 2]

## User Stories
- STORY_01: [Story name] - [Status: Not Started/In Progress/Complete]
- STORY_02: [Story name] - [Status]

## Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2

## Dependencies
- Requires: [Other epics or prerequisites]
- Blocks: [What depends on this]

## Timeline
- Estimated: X days/weeks
- Actual: [Fill when complete]
```

**2. stories/** (Individual User Stories):
```markdown
# Story XX: [Story Name]

**Epic**: EPIC_XX
**Status**: Not Started / In Progress / Complete
**Assignee**: [Agent/Person]

---

## User Story

As a [user type],
I want [feature],
So that [benefit].

---

## Acceptance Criteria

- [ ] Criterion 1
- [ ] Criterion 2
- [ ] All tests passing
- [ ] Work log created

---

## Technical Design

### Approach
[How this will be implemented]

### Files to Modify
- `path/to/file.py` - [Changes needed]

### Integration Points
- [Component X] via [mechanism]

---

## Testing Strategy

### Unit Tests
- Test case 1
- Test case 2

### Integration Tests
- Test case 1

### Manual Testing
- Test procedure 1
- Expected result

---

## Definition of Done

- [ ] Tests written and passing
- [ ] Code implemented
- [ ] Manual testing completed
- [ ] Integration validated
- [ ] Work log created
- [ ] Code committed and pushed
```

---

### Example Epic: EPIC_00 (Testing Infrastructure)

**Location**: `docs/BACKLOG/EPIC_00/`

**Structure**:
```
EPIC_00/
├── README.md                       # Epic overview
└── stories/
    ├── STORY_01_pytest_setup.md
    ├── STORY_02_unit_tests_core.md
    ├── STORY_03_webhook_tests.md
    ├── STORY_04_integration_tests.md
    └── STORY_05_ci_integration.md
```

**Epic Goals**:
1. Set up pytest infrastructure
2. Add unit tests for critical functions
3. Add integration tests for webhooks
4. Integrate tests into CI/CD pipeline
5. Achieve 60%+ code coverage

---

## 🔧 Major Components Deep Dive

### Transcription Pipeline

```
1. gen_subtitles_queue() - Entry point
   ├─> Check if already queued/processing
   ├─> Validate has_audio()
   ├─> choose_transcribe_language() - Determine language
   ├─> should_skip_file() - Check skip conditions (existing subs, languages, etc.)
   ├─> Queue detect_language task (if language unknown and detection enabled)
   └─> Queue transcribe task

2. transcription_worker() - Worker thread
   ├─> Fetch task from queue (blocking, 1s timeout)
   ├─> Log WORKER START with status
   ├─> Route to handler: detect_language, asr, or gen_subtitles
   ├─> Log WORKER FINISH with elapsed time
   ├─> Mark task done
   └─> Schedule model cleanup

3. gen_subtitles() - Actual transcription
   ├─> start_model() - Load Whisper model if needed
   ├─> handle_multiple_audio_tracks() - Extract specific audio track if multiple
   ├─> model.transcribe() - Perform transcription
   ├─> appendLine() - Add "Transcribed by whisperAI" footer (if enabled)
   ├─> write_lrc() or result.to_srt_vtt() - Write output file
   └─> delete_model() - Schedule cleanup

4. Model Cleanup
   ├─> delete_model() - Check if queue idle, schedule timer
   ├─> schedule_model_cleanup() - Cancel existing timer, start new 30s timer
   └─> perform_model_cleanup() - Unload model, clear CUDA, gc.collect()
```

### Skip Logic (should_skip_file)

Checks in order:
1. LRC exists for audio file
2. Unknown language + skip_unknown_language
3. Target language subtitle already exists + skip_if_to_transcribe_sub_already_exist
4. Internal subtitle in skipifinternalsublang language exists
5. External subtitle in namesublang language exists + skipifexternalsub
6. Subtitle language in skip_lang_codes_list
7. Audio track language in skip_if_audio_track_is_in_list
8. No preferred audio language found + limit_to_preferred_audio_languages

### Webhook Flow

**Plex**:
```
POST /plex
  ├─> Validate user_agent contains "PlexMediaServer"
  ├─> Parse JSON payload (multipart form)
  ├─> Check event: library.new (added) or media.play (played)
  ├─> Extract ratingKey from Metadata
  ├─> get_plex_file_name(ratingKey) - API call to get file path
  ├─> gen_subtitles_queue() - Queue transcription with plex_item_id
  ├─> Optional: Queue next episode (PLEX_QUEUE_NEXT_EPISODE)
  ├─> Optional: Queue season/series (PLEX_QUEUE_SEASON/SERIES)
  └─> refresh_plex_metadata() called after transcription in worker
```

**Jellyfin**:
```
POST /jellyfin
  ├─> Validate user_agent contains "Jellyfin-Server"
  ├─> Parse JSON body (NotificationType, ItemId)
  ├─> Check event: ItemAdded or PlaybackStart
  ├─> get_jellyfin_file_name(ItemId) - API call to get file path
  ├─> gen_subtitles_queue() - Queue transcription with jellyfin_item_id
  └─> refresh_jellyfin_metadata() called after transcription in worker
```

**Emby**:
```
POST /emby
  ├─> Parse multipart form data
  ├─> Check event: library.new or playback.start
  ├─> Extract file path directly from data_dict['Item']['Path']
  ├─> gen_subtitles_queue() - Queue transcription
  └─> No metadata refresh (not implemented for Emby)
```

**Tautulli**:
```
POST /tautulli
  ├─> Validate source header == "Tautulli"
  ├─> Parse JSON body (event, file)
  ├─> Check event: added or played
  ├─> Extract file path directly from body
  └─> gen_subtitles_queue() - Queue transcription
```

### ASR Endpoint (Bazarr Integration)

```
POST /asr
  ├─> Read audio file into memory
  ├─> Generate audio hash: SHA256(audio + task + language)
  ├─> task_id = "asr-{hash}"
  ├─> Create TaskResult for this task_id
  ├─> Queue ASR task (returns False if already queued)
  ├─> Block with task_result.wait(timeout=ASR_TIMEOUT)
  ├─> Return StreamingResponse with SRT content
  └─> Worker calls asr_task_worker() to process
      ├─> start_model()
      ├─> model.transcribe()
      ├─> result_container.set_result() - Unblock endpoint
      └─> delete_model()
```

---

## 📦 Dependencies

From `requirements.txt`:
```
numpy                    # Array operations for audio processing
stable-ts-whisperless    # Stable timestamps fork of Whisper (without OpenAI whisper)
fastapi                  # Web framework for REST API
requests                 # HTTP client for Plex/Jellyfin API calls
faster-whisper           # Faster Whisper implementation (CTranslate2)
uvicorn                  # ASGI server for FastAPI
python-multipart         # Form data parsing for webhooks
ffmpeg-python            # FFmpeg wrapper for audio extraction
watchdog                 # Filesystem monitoring (for MONITOR feature)
```

Additional dependencies (Docker-installed):
```
torch                    # PyTorch for Whisper model
torchaudio               # Audio I/O for PyTorch
av                       # PyAV for media file inspection
```

---

## 🐳 Docker Deployment

### Image Variants

**1. GPU Image (mccloud/subgen:latest)**
- Base: `nvidia/cuda:12.3.2-base-ubuntu22.04`
- Size: ~8GB (includes CUDA toolkit)
- Platforms: linux/amd64 only
- Use case: Users with NVIDIA GPU

**2. CPU Image (mccloud/subgen:cpu)**
- Base: `python:3.11-slim-bullseye`
- Size: ~4GB (multi-stage build, no CUDA)
- Platforms: linux/amd64, linux/arm64
- Use case: Users without GPU or ARM devices (Raspberry Pi, Mac M1/M2)

### Docker Features

**Rootless Support**:
- `PUID` and `PGID` environment variables
- `entrypoint.sh` creates user/group and drops privileges via gosu
- Podman `--userns=keep-id` compatibility
- Safety checks for already-non-root and PUID=0 scenarios

**Cache Management**:
- `/cache` directory for HuggingFace models, matplotlib
- Environment variables: `XDG_CACHE_HOME`, `HF_HOME`, `MPLCONFIGDIR`
- Prevents writes to root-owned `/.cache`

**Volume Mounts**:
- `/subgen/models` - Persistent model storage
- Media directories - Must match media server paths exactly (or use PATH_MAPPING)

---

## 🔄 CI/CD Pipeline

### GitHub Actions Workflows

**1. build_GPU.yml**
- Trigger: Push to `Dockerfile` or `requirements.txt`, or manual
- Runner: Self-hosted (likely has GPU for testing)
- Steps:
  1. Extract version from `subgen.py` (grep for subgen_version)
  2. Login to Docker Hub
  3. Build and push: `mccloud/subgen:latest` and `mccloud/subgen:{version}`

**2. build_CPU.yml**
- Trigger: Push to `Dockerfile.cpu` or `requirements.txt`, or manual
- Runner: GitHub-hosted ubuntu-latest
- Steps:
  1. Extract version from `subgen.py`
  2. Setup QEMU for multi-arch builds
  3. Setup Docker Buildx
  4. Login to Docker Hub
  5. Build and push: `mccloud/subgen:cpu` and `mccloud/subgen:{version}-cpu`
  6. Platforms: linux/amd64, linux/arm64

**3. calver.yml (Version Automation)**
- Trigger: Push to `subgen.py` on main branch, or manual
- Steps:
  1. Calculate version: `{YEAR}.{MONTH}.{commit_count_this_month}`
  2. Update `subgen_version` variable in subgen.py via sed
  3. Check if file actually changed (compare with HEAD)
  4. If changed: Amend last commit with new version
  5. Force push with `--force-with-lease` (safer than --force)

**Version Calculation Example**:
- Date: 2026-02-15
- Commits in February: 9
- Version: `2026.02.9`

---

## ⚙️ Configuration

### Environment Variables (Grouped by Category)

**Whisper Configuration**:
- `WHISPER_MODEL` (default: medium) - Model size: tiny, base, small, medium, large, distil-*
- `WHISPER_THREADS` (default: 4) - CPU threads for computation
- `TRANSCRIBE_DEVICE` (default: cpu) - Device: cpu, gpu, cuda
- `COMPUTE_TYPE` (default: auto) - Quantization: int8, int8_float16, float16, float32
- `MODEL_PATH` (default: ./models) - Model storage location

**Processing Control**:
- `CONCURRENT_TRANSCRIPTIONS` (default: 2) - Number of parallel transcription workers
- `PROCESS_ADDED_MEDIA` (default: True) - Process new media from webhooks
- `PROCESS_MEDIA_ON_PLAY` (default: True) - Process media when played
- `TRANSCRIBE_OR_TRANSLATE` (default: transcribe) - Mode: transcribe (same language) or translate (to English)

**Language Configuration**:
- `SUBTITLE_LANGUAGE_NAME` (default: aa) - Output subtitle language code
- `SUBTITLE_LANGUAGE_NAMING_TYPE` (default: ISO_639_2_B) - Format: ISO_639_1, ISO_639_2_T, ISO_639_2_B, NAME, NATIVE
- `FORCE_DETECTED_LANGUAGE_TO` (default: '') - Force language detection to specific code
- `PREFERRED_AUDIO_LANGUAGES` (default: eng) - Pipe-separated list of preferred audio track languages
- `DETECT_LANGUAGE_LENGTH` (default: 30) - Seconds of audio to use for language detection
- `DETECT_LANGUAGE_OFFSET` (default: 0) - Offset in seconds before detecting language

**Skip Conditions**:
- `SKIP_IF_INTERNAL_SUBTITLES_LANGUAGE` (default: eng) - Skip if internal subs in this language
- `SKIP_IF_EXTERNAL_SUBTITLES_EXIST` (default: False) - Skip if external subs exist
- `SKIP_IF_TARGET_SUBTITLES_EXIST` (default: True) - Skip if target language subs exist
- `SKIP_SUBTITLE_LANGUAGES` (default: '') - Pipe-separated list of languages to skip
- `SKIP_IF_AUDIO_LANGUAGES` (default: '') - Skip if audio track is in this language
- `SKIP_ONLY_SUBGEN_SUBTITLES` (default: False) - Only skip if subtitle has "subgen" in filename
- `SKIP_UNKNOWN_LANGUAGE` (default: False) - Skip if language detection fails
- `SKIP_IF_NO_LANGUAGE_BUT_SUBTITLES_EXIST` (default: False) - Skip if no language but subs exist

**Server Integration**:
- `PLEX_SERVER` (default: http://plex:32400) - Plex server URL
- `PLEX_TOKEN` (default: token here) - Plex authentication token
- `PLEX_QUEUE_NEXT_EPISODE` (default: False) - Auto-queue next episode in series
- `PLEX_QUEUE_SEASON` (default: False) - Auto-queue rest of season
- `PLEX_QUEUE_SERIES` (default: False) - Auto-queue entire series
- `JELLYFIN_SERVER` (default: http://jellyfin:8096) - Jellyfin server URL
- `JELLYFIN_TOKEN` (default: token here) - Jellyfin authentication token

**Path Mapping**:
- `USE_PATH_MAPPING` (default: False) - Enable path translation
- `PATH_MAPPING_FROM` (default: /tv) - Source path (as seen by media server)
- `PATH_MAPPING_TO` (default: /Volumes/TV) - Destination path (as seen by Subgen)

**Subtitle Formatting**:
- `WORD_LEVEL_HIGHLIGHT` (default: False) - Highlight words as spoken (karaoke-style)
- `CUSTOM_REGROUP` (default: cm_sl=84_sl=42++++++1) - Stable-TS regroup algorithm
- `LRC_FOR_AUDIO_FILES` (default: True) - Generate LRC instead of SRT for audio files
- `SHOW_IN_SUBNAME_SUBGEN` (default: True) - Add "subgen" to subtitle filename
- `SHOW_IN_SUBNAME_MODEL` (default: True) - Add model name to subtitle filename
- `APPEND` (default: False) - Append "Transcribed by whisperAI" footer

**Advanced**:
- `WEBHOOK_PORT` (default: 9000) - Port for webhook server
- `DEBUG` (default: True) - Enable debug logging
- `CLEAR_VRAM_ON_COMPLETE` (default: True) - Unload model when queue empty
- `MODEL_CLEANUP_DELAY` (default: 30) - Seconds to wait before cleaning up model
- `ASR_TIMEOUT` (default: 18000) - Timeout for ASR requests (5 hours)
- `MONITOR` (default: False) - Watch TRANSCRIBE_FOLDERS for new files
- `TRANSCRIBE_FOLDERS` (default: '') - Pipe-separated folders to transcribe on startup
- `UPDATE` (default: False) - Auto-update subgen.py from GitHub on start
- `SUBGEN_KWARGS` (default: {}) - Additional kwargs for model.transcribe() (advanced users)
- `USE_MODEL_PROMPT` (default: False) - Use prompt to force punctuation
- `CUSTOM_MODEL_PROMPT` (default: '') - Custom prompt for transcription
- `PUID` (default: 99) - User ID for rootless Docker
- `PGID` (default: 100) - Group ID for rootless Docker

---

## 🧪 Testing Infrastructure

**Finding**: NO testing infrastructure exists in this repository.

**Evidence**:
- No test files (`*test*.py`, `*spec*.py`)
- No test directories (`test/`, `tests/`)
- No testing frameworks in requirements.txt (no pytest, unittest)
- No CI testing workflows (workflows only build Docker images)

**Implications**:
- All code changes rely on manual testing
- No regression detection
- Refactoring is risky
- Heavy reliance on production user feedback

**Recommended Testing Strategy** (if implementing):
1. Unit tests for:
   - Language code conversions
   - Skip condition logic
   - Audio hash generation
   - Queue deduplication
   - Path mapping
2. Integration tests for:
   - Webhook parsing (Plex, Jellyfin, Emby, Tautulli)
   - ASR endpoint flow
   - Model loading/unloading
   - File output (SRT/LRC generation)
3. End-to-end tests:
   - Full transcription pipeline
   - Metadata refresh
   - Concurrent worker behavior

---

## 🚨 Known Architectural Issues

### 1. No Input Validation
**Issue**: Environment variables and webhook payloads not validated
**Risk**: Runtime errors from malformed input
**Examples**:
- Invalid language codes → LanguageCode.NONE silently
- Invalid file paths → logged but not handled gracefully
- Malformed JSON → uncaught exceptions

### 2. Global State Management
**Issue**: Heavy use of global variables (model, task_queue, task_results)
**Risk**: 
- Difficult to test in isolation
- Potential race conditions
- Unclear ownership

### 3. Tight Coupling
**Issue**: Transcription logic tightly coupled to FastAPI endpoints
**Example**: `gen_subtitles()` function called from multiple contexts (webhooks, ASR, batch) but has no abstraction layer
**Impact**: Difficult to refactor or add features

### 4. Error Handling Inconsistency
**Issue**: Some errors logged, some raised, some silently ignored
**Example**: 
- `has_audio()` returns False on exception (silent)
- `get_plex_file_name()` raises Exception on error
- `gen_subtitles()` logs exception and continues

### 5. Resource Leaks
**Issue**: File handles and audio file objects not always closed
**Example**: `audio_file.close()` in finally blocks, but some code paths may skip

### 6. Hardcoded Magic Values
**Issue**: Many magic numbers and strings throughout code
**Examples**:
- Priority levels (0, 1, 2) hardcoded in queue logic
- File extensions scattered across functions
- 30-second delay for file stability check

---

## 📝 Code Quality Observations

### Strengths
1. **Extensive logging**: Good visibility into system behavior
2. **Backwards compatibility**: Careful migration of environment variables
3. **Comprehensive README**: User documentation is excellent
4. **Multi-platform support**: CPU/GPU, Docker/standalone, multiple architectures
5. **Active maintenance**: Frequent commits, auto-versioning

### Weaknesses
1. **No tests**: Zero test coverage
2. **2144-line monolith**: All logic in single file (subgen.py)
3. **Inconsistent naming**: Mix of camelCase, snake_case, PascalCase
4. **Documentation comments**: Almost no inline comments explaining complex logic
5. **Type hints**: Inconsistent use (some functions typed, many not)
6. **Error messages**: Often too generic ("Error processing file")

### Code Smells
1. **Long functions**: `gen_subtitles()`, `should_skip_file()`, `get_next_plex_episode()` are 100+ lines
2. **Deep nesting**: Some functions have 4-5 levels of if/else
3. **Duplicate logic**: Path validation repeated across functions
4. **Magic booleans**: `recursion=True`, `only_skip_if_subgen_subtitle=False` parameters

---

## 🔮 Potential Improvements

### High Priority
1. **Add testing infrastructure**
   - Start with unit tests for utility functions
   - Add integration tests for webhook handling
   - Mock Whisper model to avoid slow tests

2. **Refactor into modules**
   - `webhooks.py` - Webhook handlers
   - `transcription.py` - Transcription logic
   - `queue_manager.py` - Queue and worker management
   - `media_server.py` - Plex/Jellyfin/Emby API interactions
   - `config.py` - Environment variable management

3. **Add input validation**
   - Validate environment variables on startup
   - Validate webhook payloads with Pydantic models
   - Return 400 Bad Request for invalid input

4. **Improve error handling**
   - Define custom exception types
   - Consistent error response format
   - Graceful degradation (continue processing other files on error)

### Medium Priority
5. **Add health check endpoint**
   - `/health` endpoint for Docker/k8s readiness/liveness
   - Check model loaded, queue responsive, disk space

6. **Implement metrics/observability**
   - Prometheus metrics (transcriptions completed, queue size, model load time)
   - Structured logging (JSON format for log aggregation)
   - Request tracing (correlation IDs)

7. **Add configuration validation**
   - Validate language codes on startup
   - Check file paths exist
   - Warn about conflicting settings

8. **Improve concurrency model**
   - Consider async/await for I/O-bound operations
   - Separate network I/O from CPU-bound transcription
   - Thread pool for blocking operations

### Low Priority
9. **Add CLI interface**
   - Standalone CLI tool for transcription without server
   - Useful for batch processing without webhooks

10. **Support additional subtitle formats**
    - WebVTT, SubRip, ASS/SSA, TTML
    - User-selectable via environment variable

---

## 🎓 Learning from This Codebase

### What This Project Does Well
1. **Single-purpose focus**: Does one thing (transcription) well
2. **User-centric**: Extensive documentation, backwards compatibility
3. **Production-ready**: Runs in production for many users
4. **Active community**: Frequent updates based on user feedback

### Anti-Patterns to Avoid
1. **No testing**: Makes refactoring and feature additions risky
2. **Monolithic structure**: All logic in one file (2144 lines)
3. **Global state**: Difficult to reason about and test
4. **Inconsistent patterns**: Mix of styles throughout codebase

### Lessons for LLM Development
1. **Start with tests**: TDD prevents technical debt
2. **Modular design**: Separate concerns early
3. **Type hints**: Make code self-documenting and catch errors
4. **Validation first**: Validate all inputs before processing
5. **Observability**: Logging, metrics, health checks from day one

---

## 🔍 Key Files Reference

### subgen.py (2144 lines)
**Main application file**

Key sections:
- Lines 1-210: Environment variables and configuration
- Lines 211-324: Deduplicated queue implementation
- Lines 332-393: Worker thread pool
- Lines 395-492: Logging configuration and progress handler
- Lines 513-685: Webhook endpoints (Plex, Jellyfin, Emby, Tautulli)
- Lines 687-803: ASR and detect-language endpoints
- Lines 806-858: ASR task worker
- Lines 1050-1098: Language detection worker
- Lines 1143-1214: Model management and cleanup
- Lines 1227-1274: Subtitle generation (gen_subtitles)
- Lines 1318-1350: Audio track handling
- Lines 1405-1444: Language selection logic
- Lines 1526-1632: Skip condition logic
- Lines 1634-1788: Subtitle checking utilities
- Lines 1790-1889: Plex next episode logic
- Lines 1891-2014: Plex/Jellyfin API interactions
- Lines 2016-2086: File validation and path mapping
- Lines 2087-2134: File system monitoring (watchdog)
- Lines 2136-2144: Main entry point (uvicorn)

### language_code.py (199 lines)
**Language code management**

Key sections:
- Lines 3-106: LanguageCode enum definition (104 languages)
- Lines 109-114: Enum initialization
- Lines 116-158: Conversion methods (from_* and to_*)
- Lines 160-199: Utility methods (__str__, __bool__, __eq__)

### launcher.py (183 lines)
**Bootstrap and update script**

Key sections:
- Lines 7-9: Boolean conversion utility
- Lines 11-20: Package installation
- Lines 22-33: GitHub file download
- Lines 35-58: Bazarr setup wizard
- Lines 60-73: Environment variable loading
- Lines 75-180: Main launcher logic (argument parsing, updates, execution)

---

## 🚀 Getting Started (for LLMs)

### Prerequisites
- Python 3.9-3.11
- FFmpeg installed
- NVIDIA drivers (if using GPU)
- Media server (Plex, Jellyfin, or Emby) OR Bazarr

### Quick Start (Standalone)
```bash
# Download launcher
wget https://raw.githubusercontent.com/McCloudS/subgen/main/launcher.py

# Install dependencies and run
python3 launcher.py -i -u -s
```

### Quick Start (Docker)
```bash
# GPU version
docker run -d \
  -p 9000:9000 \
  -v /path/to/media:/media \
  -v /path/to/models:/subgen/models \
  -e WHISPER_MODEL=medium \
  -e TRANSCRIBE_DEVICE=gpu \
  --gpus all \
  mccloud/subgen:latest

# CPU version
docker run -d \
  -p 9000:9000 \
  -v /path/to/media:/media \
  -v /path/to/models:/subgen/models \
  -e WHISPER_MODEL=medium \
  -e TRANSCRIBE_DEVICE=cpu \
  mccloud/subgen:cpu
```

### Configuration Steps
1. Set up webhook in media server pointing to `http://subgen:9000/{plex,jellyfin,emby}`
2. Configure environment variables (see Configuration section)
3. Test with `/status` endpoint: `curl http://localhost:9000/status`
4. Monitor logs: `docker logs -f subgen`

---

## 📖 Documentation Structure

### Current Structure

```
subgen/
├── README.md           # User-facing documentation (comprehensive)
├── README-LLM.md       # This file - LLM development guide
└── docs/               # Development documentation (TO BE CREATED)
    ├── WORKLOGS/       # Mandatory work logs for all tasks
    │   ├── README.md   # Work log guide
    │   └── NNNN_YYYY-MM-DD_*.md
    ├── BACKLOG/        # Planning and epic tracking
    │   ├── EPIC_00/    # Testing infrastructure
    │   ├── EPIC_01/    # Modular refactoring
    │   └── [...]
    └── ARCHITECTURE/   # Architecture documentation (future)
        ├── TESTING_STRATEGY.md
        ├── MODULAR_DESIGN.md
        └── [...]
```

### Documentation Standards

**Work Logs** (MANDATORY):
- MUST be created for every completed task
- MUST follow naming convention: `NNNN_YYYY-MM-DD_description.md`
- MUST include: summary, implementation details, test results, next steps
- MUST be committed with code changes

**Epic READMEs**:
- Overview of epic goals and scope
- List of user stories with status
- Acceptance criteria
- Dependencies and blockers
- Timeline estimates

**User Stories**:
- Clear user story format (As a..., I want..., So that...)
- Acceptance criteria (testable)
- Technical design approach
- Integration points
- Definition of done

---

## 🚫 Common Mistakes to Avoid

### 1. Skipping Tests
❌ **Wrong**: Implement feature without tests
✅ **Right**: Write tests FIRST, watch them fail, then implement

### 2. Using Print for Logging
❌ **Wrong**: `print(f"Processing {file_path}")`
✅ **Right**: `logging.info(f"Processing {file_path}")`

**Why**: Logging module provides levels, formatting, filtering, and can be captured in tests

### 3. Ignoring Type Hints
❌ **Wrong**: `def process_file(path, lang):`
✅ **Right**: `def process_file(path: str, lang: LanguageCode) -> bool:`

**Why**: Type hints catch errors early, document intent, enable IDE assistance

### 4. Silently Failing
❌ **Wrong**: 
```python
try:
    result = risky_operation()
except Exception:
    pass  # Silent failure
```
✅ **Right**:
```python
try:
    result = risky_operation()
except Exception as e:
    logging.error(f"Failed to perform operation: {e}", exc_info=True)
    raise  # or return error indicator
```

### 5. Adding to Monolith
❌ **Wrong**: Add more functions to subgen.py (already 2144 lines)
✅ **Right**: Create new module, import into subgen.py

**Why**: Monolithic files become unmaintainable. Break into logical modules.

### 6. Global State Without Thread Safety
❌ **Wrong**: 
```python
task_results = {}  # No lock
task_results[task_id] = result  # Race condition
```
✅ **Right**:
```python
task_results_lock = Lock()
with task_results_lock:
    task_results[task_id] = result
```

### 7. Hardcoding Configuration
❌ **Wrong**: `TIMEOUT = 18000  # 5 hours`
✅ **Right**: `asr_timeout = int(os.getenv('ASR_TIMEOUT', 18000))`

**Why**: Configuration should be externalized for different environments

### 8. Skipping Work Logs
❌ **Wrong**: Complete task and move on
✅ **Right**: Create work log documenting what was done, then mark complete

**Why**: Work logs provide context for future developers and track project history

---

## 🎯 Quick Reference

### Essential Commands

```bash
# Run application (standalone)
python3 launcher.py -u -i          # Update and install dependencies
python3 launcher.py -s             # Setup Bazarr wizard
python3 launcher.py -d             # Run in debug mode

# Run application (Docker)
docker-compose up -d               # Start container
docker logs -f subgen              # View logs
docker exec -it subgen bash        # Access container

# Testing (when implemented)
pytest tests/ -v                   # Run all tests
pytest tests/test_webhooks.py -v  # Run specific test file
python -m mypy subgen.py          # Type checking

# Work log management
cd docs/WORKLOGS
ls -1 *.md | tail -10             # View recent logs
NEXT=$(printf "%04d" $(($(ls -1 [0-9][0-9][0-9][0-9]_*.md 2>/dev/null | sed 's/_.*//' | sort -n | tail -1) + 1)))
echo "${NEXT}_$(date +%Y-%m-%d)_description.md"
```

### Essential Files to Check Before Implementing

| File | Purpose | When to Check |
|------|---------|---------------|
| `README-LLM.md` | Development guide | ALWAYS before any work |
| `subgen.py:1-210` | Environment variable definitions | Before adding configuration |
| `language_code.py` | Language handling patterns | Before language-related changes |
| `subgen.py:272-324` | Queue implementation | Before queue-related changes |
| `subgen.py:332-393` | Worker thread pattern | Before worker-related changes |
| `requirements.txt` | Dependencies | Before adding new libraries |
| `docs/BACKLOG/EPIC_XX/` | Epic and story definitions | Before implementing features |
| `docs/WORKLOGS/` | Recent work logs | To understand recent changes |

---

## 📚 Additional Resources

- **GitHub Repository**: https://github.com/McCloudS/subgen
- **Docker Hub**: https://hub.docker.com/r/mccloud/subgen
- **OpenAI Whisper**: https://github.com/openai/whisper
- **faster-whisper**: https://github.com/guillaumekln/faster-whisper
- **stable-ts**: https://github.com/jianfch/stable-ts
- **Bazarr**: https://www.bazarr.media/
- **FastAPI**: https://fastapi.tiangolo.com/
- **pytest**: https://docs.pytest.org/

---

**End of README-LLM.md**

This document captures everything an LLM needs to understand and work with the Subgen codebase. For user-facing documentation, see README.md.
