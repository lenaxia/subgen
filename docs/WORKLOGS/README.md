# Work Logs Directory

This directory contains chronological work logs documenting all development sessions, feature implementations, bug fixes, and significant work on the Subgen project.

---

## 🚨 MANDATORY REQUIREMENT

**EVERY completed task, story, or significant work session MUST have a work log.**

**A task is NOT complete without a work log. No exceptions.**

---

## File Naming Convention

**Format**: `NNNN_YYYY-MM-DD_description.md`

- **NNNN**: 4-digit sequence number (0001, 0002, etc.)
- **YYYY-MM-DD**: ISO date when work was performed
- **description**: Brief descriptive name using snake_case

**Examples:**
```
0001_2026-02-15_testing_infrastructure_setup.md
0002_2026-02-15_webhook_validation_refactor.md
0003_2026-02-15_EPIC_01_story_01_completion.md
0004_2026-02-16_language_detection_bug_fix.md
```

---

## How to Create a Work Log

### 1. Get Next Sequence Number

```bash
cd docs/WORKLOGS
NEXT=$(printf "%04d" $(($(ls -1 [0-9][0-9][0-9][0-9]_*.md 2>/dev/null | sed 's/_.*//' | sort -n | tail -1) + 1)))
TODAY=$(date +%Y-%m-%d)
echo "Next work log: ${NEXT}_${TODAY}_your_description.md"
```

### 2. Create File with Template

See template in this README below or in parent README-LLM.md.

### 3. Fill in Details

Document:
- What was done (implementation details)
- Test results (pass/fail, coverage)
- Issues encountered and solutions
- Integration points identified
- Next steps

### 4. Commit with Code Changes

```bash
git add .
git commit -m "Implement feature X with tests and work log"
git push origin HEAD
```

---

## Work Log Template

```markdown
# Work Log: [Task/Story Name]

**Date**: YYYY-MM-DD
**Author**: [Agent Name/ID or Human Name]
**Epic/Story**: [Reference to docs/BACKLOG/EPIC_XX/] or N/A
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
1. [Change 1 with line numbers or code references]
2. [Change 2]
3. [Change 3]

### Design Decisions
- **Decision**: [What was decided]
- **Rationale**: [Why this approach was chosen]
- **Trade-offs**: [What alternatives were considered and rejected, and why]

---

## Testing

### Test Coverage
- Unit tests: X/Y passing (or N/A if no tests yet)
- Integration tests: X/Y passing
- Manual testing: [Results summary]

### Test Scenarios Covered
1. [Happy path scenario 1]
2. [Happy path scenario 2]
3. [Edge case 1]
4. [Error case 1]

### Testing Commands
```bash
pytest tests/test_feature.py -v
python -m mypy subgen.py
```

---

## Issues Encountered

### [Issue 1 Name]
- **Problem**: [Detailed description of what went wrong]
- **Solution**: [How it was resolved, with code references]
- **Prevention**: [How to avoid this in future]

### [Issue 2 Name]
[Same structure]

---

## Next Steps

1. [Next action item 1]
2. [Next action item 2]
3. [Dependencies that need to be completed first]

---

## Integration Points

- `function_name()` in `file.py:123` integrates with [component] via [mechanism]
- `class_name` called by [other component] when [event/condition]
- New endpoint `/endpoint` expects [payload format] from [caller]

---

## Validation Commands

```bash
# Commands used to validate the implementation
pytest tests/test_new_feature.py -v
curl -X POST http://localhost:9000/endpoint -d '{"test": "data"}'
docker logs subgen | grep "expected pattern"
```

---

## References

- Epic README: docs/BACKLOG/EPIC_XX/README.md
- Related work logs: NNNN_YYYY-MM-DD_related_task.md
- GitHub issues: #XXX
- Pull requests: #XXX
```

---

## Work Log Types

### Feature Implementation
Document new features, enhancements, or capabilities added to Subgen.

### Bug Fixes
Document issues resolved, including root cause analysis.

### Refactoring
Document code restructuring, performance improvements, or technical debt reduction.

### Epic/Story Completion
Document completion of entire epic or user story, summarizing all sub-tasks.

### Handoff Reports
Document current state when handing off work to another agent or developer.

---

## Tips for Quality Work Logs

**Be Specific**:
- Reference exact file paths and line numbers
- Include code snippets for key changes
- Link to related issues, PRs, or work logs

**Be Complete**:
- Document ALL files modified
- List ALL tests added
- Capture ALL issues encountered (even if resolved)

**Be Forward-Looking**:
- Identify follow-up work needed
- Note technical debt introduced (if any)
- Suggest future improvements

**Be Honest**:
- Document shortcuts taken and why
- Note incomplete testing or validation
- Identify known issues or limitations

---

## Finding Work Logs

### By Date
```bash
ls -1 *2026-02-15*.md
```

### By Topic
```bash
grep -l "webhook" *.md
grep -l "testing" *.md
grep -l "EPIC_01" *.md
```

### Recent Activity
```bash
ls -1 [0-9]*.md | tail -20  # Last 20 logs
```

### By Author (if tracked)
```bash
grep -l "Author: AgentName" *.md
```

---

## Maintenance

**Sequence Numbers**:
- Never reuse sequence numbers
- Always increment from highest existing number
- If collision occurs, the later work log should increment

**Archiving**:
- Work logs from previous years can be archived to `docs/WORKLOGS/archive/YYYY/`
- Keep current year logs in main directory
- Update archive index when archiving

---

**For questions about work logs, see README-LLM.md or ask the repository maintainer.**
