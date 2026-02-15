# Epic 00: Testing Infrastructure

**Status**: Not Started
**Priority**: Critical
**Estimated Effort**: 3-5 days
**Dependencies**: None
**Owner**: TBD

---

## Overview

Establish comprehensive testing infrastructure for Subgen to enable safe refactoring, feature additions, and regression detection. Currently, the project has ZERO tests, making changes risky and relying entirely on manual testing.

---

## Problem Statement

**Current State**:
- No test files exist in repository
- No pytest or testing framework configured
- No CI/CD testing pipeline
- All testing is manual or production-based
- Refactoring and feature additions are high-risk

**Desired State**:
- Pytest infrastructure established
- 60%+ code coverage for critical paths
- Integration tests for all webhooks
- CI/CD runs tests automatically
- Safe refactoring enabled

---

## Goals

1. **Set up pytest infrastructure** with fixtures and mocks
2. **Add unit tests** for critical utility functions (30+ tests)
3. **Add integration tests** for webhook endpoints (20+ tests)
4. **Add end-to-end tests** for transcription pipeline (10+ tests)
5. **Integrate tests into CI/CD** pipeline (GitHub Actions)
6. **Achieve 60%+ code coverage** for core functionality

---

## User Stories

### Completed
- None

### In Progress
- None

### Not Started
- **STORY_01**: Set up pytest infrastructure with fixtures
- **STORY_02**: Add unit tests for language_code.py module
- **STORY_03**: Add unit tests for queue deduplication logic
- **STORY_04**: Add unit tests for skip condition logic
- **STORY_05**: Add integration tests for webhook parsing
- **STORY_06**: Add integration tests for ASR endpoint
- **STORY_07**: Add end-to-end tests for transcription pipeline
- **STORY_08**: Add CI/CD test runner workflow
- **STORY_09**: Add code coverage reporting
- **STORY_10**: Document testing patterns and best practices

---

## Acceptance Criteria

### Technical Criteria
- [ ] pytest framework installed and configured
- [ ] Test directory structure created (`tests/`, `tests/unit/`, `tests/integration/`, `tests/e2e/`)
- [ ] Pytest fixtures for common test scenarios (mock model, mock media files, mock webhooks)
- [ ] Mock Whisper model to avoid loading real models in tests
- [ ] 60%+ code coverage for core functionality
- [ ] All tests passing in CI/CD
- [ ] Test runner added to GitHub Actions workflow
- [ ] Coverage reports generated and visible

### Functional Criteria
- [ ] Language code conversions tested (all 104 languages)
- [ ] Queue deduplication tested (duplicate detection, priority ordering)
- [ ] Skip condition logic tested (all 8 skip conditions)
- [ ] Webhook parsing tested (Plex, Jellyfin, Emby, Tautulli)
- [ ] ASR endpoint tested (blocking behavior, result storage)
- [ ] Model loading/cleanup tested
- [ ] Path mapping tested
- [ ] Audio hash generation tested

### Documentation Criteria
- [ ] Testing guide created in docs/ARCHITECTURE/TESTING_STRATEGY.md
- [ ] Pytest usage documented in README.md
- [ ] Work logs created for each story completion

---

## Implementation Approach

### Phase 1: Infrastructure (Story 01)
**Effort**: 4-6 hours

Set up pytest infrastructure with essential fixtures:
- Install pytest, pytest-cov, pytest-mock
- Create `tests/` directory structure
- Create `conftest.py` with fixtures (mock model, mock media files, mock FastAPI client)
- Create `pytest.ini` configuration
- Add `.coveragerc` for coverage configuration

**Deliverables**:
- `pytest.ini`
- `tests/conftest.py`
- `.coveragerc`
- Updated `requirements.txt` with test dependencies

### Phase 2: Unit Tests - Language Module (Story 02)
**Effort**: 2-3 hours

Test `language_code.py` exhaustively:
- All conversion methods (from_* and to_*)
- Special cases (NONE, unknown codes)
- Equality comparisons
- String representations

**Target**: 100% coverage of `language_code.py`

### Phase 3: Unit Tests - Queue (Story 03)
**Effort**: 3-4 hours

Test `DeduplicatedQueue` class:
- Deduplication by task ID
- Priority ordering (0 < 1 < 2)
- Status tracking (_queued, _processing)
- Thread safety (concurrent put/get operations)
- is_idle(), is_active(), mark_done()

**Target**: 100% coverage of queue implementation

### Phase 4: Unit Tests - Skip Logic (Story 04)
**Effort**: 4-5 hours

Test `should_skip_file()` and related functions:
- All 8 skip conditions
- Edge cases (missing files, malformed paths)
- Language preference logic
- Subtitle detection (internal and external)

**Target**: 90%+ coverage of skip logic

### Phase 5: Integration Tests - Webhooks (Story 05)
**Effort**: 5-6 hours

Test webhook endpoints with mocked FastAPI TestClient:
- Plex webhook parsing and queuing
- Jellyfin webhook parsing and queuing
- Emby webhook parsing and queuing
- Tautulli webhook parsing and queuing
- Invalid webhook rejection
- Queue integration validation

**Target**: All webhook endpoints covered

### Phase 6: Integration Tests - ASR (Story 06)
**Effort**: 4-5 hours

Test ASR endpoint with mocked Whisper model:
- Audio hash generation
- Blocking behavior
- Result storage and retrieval
- Timeout handling
- Concurrent request deduplication

**Target**: ASR endpoint fully covered

### Phase 7: End-to-End Tests (Story 07)
**Effort**: 6-8 hours

Test complete transcription pipeline:
- Webhook → Queue → Worker → Model → Output
- Use small test audio files
- Verify SRT/LRC file generation
- Test metadata refresh
- Test error scenarios

**Target**: Critical paths covered end-to-end

### Phase 8: CI/CD Integration (Story 08)
**Effort**: 2-3 hours

Add test runner to GitHub Actions:
- Create `test.yml` workflow
- Run on every push and PR
- Fail build if tests fail
- Matrix testing (Python 3.9, 3.10, 3.11)

**Deliverables**:
- `.github/workflows/test.yml`

### Phase 9: Coverage Reporting (Story 09)
**Effort**: 2-3 hours

Add coverage reporting:
- Generate coverage reports in CI
- Upload to codecov.io or similar
- Add coverage badge to README.md
- Set coverage threshold (60% minimum)

**Deliverables**:
- Coverage configuration
- CI integration
- README badge

### Phase 10: Documentation (Story 10)
**Effort**: 2-3 hours

Document testing patterns:
- Create `docs/ARCHITECTURE/TESTING_STRATEGY.md`
- Document how to write tests
- Document how to run tests
- Document how to debug test failures

**Deliverables**:
- `docs/ARCHITECTURE/TESTING_STRATEGY.md`
- Updated README.md with testing section

---

## Dependencies

**None** - This epic has no dependencies and should be completed before other epics.

---

## Blocks

This epic blocks:
- **EPIC_01**: Modular Refactoring (needs tests before safe refactoring)
- **EPIC_02**: Input Validation (needs test infrastructure)
- **EPIC_03**: Error Handling Improvements (needs tests to verify behavior)

---

## Timeline

**Estimated Total Effort**: 3-5 days (30-40 hours)

**Breakdown by Story**:
- Story 01 (Infrastructure): 4-6 hours
- Story 02 (Language tests): 2-3 hours
- Story 03 (Queue tests): 3-4 hours
- Story 04 (Skip logic tests): 4-5 hours
- Story 05 (Webhook tests): 5-6 hours
- Story 06 (ASR tests): 4-5 hours
- Story 07 (E2E tests): 6-8 hours
- Story 08 (CI integration): 2-3 hours
- Story 09 (Coverage): 2-3 hours
- Story 10 (Docs): 2-3 hours

**Critical Path**: Stories 01 → 02-06 (parallel) → 07 → 08-09 (parallel) → 10

---

## Success Metrics

**Code Coverage**:
- Overall: 60%+ coverage
- Core utilities: 90%+ coverage
- Webhook handlers: 80%+ coverage
- Queue system: 100% coverage

**Test Counts**:
- Unit tests: 80+ tests
- Integration tests: 30+ tests
- End-to-end tests: 10+ tests
- Total: 120+ tests

**CI/CD**:
- Tests run on every push
- Tests run on every PR
- Build fails if tests fail
- Coverage report generated

---

## Risks and Mitigation

### Risk 1: Mocking Whisper Model Complexity
**Risk**: Difficult to mock Whisper model behavior accurately
**Mitigation**: 
- Use small real models for E2E tests (tiny model)
- Mock only for unit/integration tests
- Document limitations of mocks

### Risk 2: Test Execution Time
**Risk**: Tests with real models may be slow
**Mitigation**:
- Use pytest markers for slow tests
- Run fast tests in CI, slow tests nightly
- Mock model for unit/integration tests

### Risk 3: Breaking Changes During Implementation
**Risk**: Adding tests may reveal bugs in existing code
**Mitigation**:
- Document bugs found as separate issues
- Fix critical bugs, defer minor bugs to future epics
- Track all findings in work logs

---

## References

- Testing best practices: https://docs.pytest.org/en/stable/goodpractices.html
- FastAPI testing: https://fastapi.tiangolo.com/tutorial/testing/
- pytest fixtures: https://docs.pytest.org/en/stable/fixture.html
- pytest-mock: https://pytest-mock.readthedocs.io/

---

**Created**: 2026-02-15
**Last Updated**: 2026-02-15
**Status**: Planning Phase
