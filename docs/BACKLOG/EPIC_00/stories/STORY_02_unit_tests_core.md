# Story 02: Unit Tests for Core Utilities (language_code.py)

**Epic**: EPIC_00 - Testing Infrastructure  
**Status**: Not Started  
**Estimated Time**: 3-4 hours  
**Assignee**: TBD  
**Depends On**: STORY_01 (pytest infrastructure)

---

## User Story

As a **developer working on Subgen**,  
I want **comprehensive unit tests for the LanguageCode enum covering all 102 languages and conversion methods**,  
So that **I can safely refactor language handling, catch bugs before production, and ensure all 102 languages convert correctly between ISO 639-1, ISO 639-2/T, ISO 639-2/B, English names, and native names**.

---

## Context

The `language_code.py` module is **critical infrastructure** used everywhere in Subgen:
- **Where used**: `subgen.py` imports it 47 times across skip logic, subtitle naming, and language detection
- **Module size**: 199 lines, 102 language enum entries plus LanguageCode.NONE
- **Complexity**: 9 methods (5 static converters + 4 instance getters)
- **Current test coverage**: **0%** (no tests exist)

**Why this matters**: 
- Language code bugs affect subtitle file naming (could overwrite wrong files)
- Skip logic depends on accurate language matching (could skip files incorrectly)
- Webhook handlers parse language codes from Plex/Jellyfin metadata
- Whisper model expects specific language codes (ISO 639-1 only)

**What makes this an ideal first test target**:
1. **Pure logic** - No I/O, no network calls, no file system access
2. **No external dependencies** - Only uses Python standard library (enum)
3. **Well-defined behavior** - Clear inputs and outputs
4. **High ROI** - Critical path, used everywhere, easy to test

**Current state of language_code.py**:
- **Location**: `/home/mikekao/personal/subgen/language_code.py`
- **Line count**: 199 lines
- **Class**: `LanguageCode(Enum)` with 102 enum entries
- **Known bugs**: `from_name()` line 136 missing `return` statement (returns None instead of LanguageCode.NONE)

---

## Acceptance Criteria

- [ ] **70+ test cases** created covering all methods
- [ ] **100% code coverage** for language_code.py (verify with `pytest --cov=language_code`)
- [ ] **All 102 languages tested** for at least one conversion method
- [ ] **All conversion methods tested**: from_iso_639_1, from_iso_639_2, from_name, from_string, is_valid_language
- [ ] **All output methods tested**: to_iso_639_1, to_iso_639_2_t, to_iso_639_2_b, to_name
- [ ] **Special cases tested**: LanguageCode.NONE, empty strings, None values, case sensitivity
- [ ] **Equality tests**: __eq__ with LanguageCode, str, None
- [ ] **Boolean/string tests**: __bool__, __str__
- [ ] **Bug documented**: from_name() missing return statement found and documented
- [ ] **All tests passing**: `pytest tests/unit/test_language_code.py -v` shows 0 failures
- [ ] **Work log created** at `docs/WORKLOGS/NNNN_2026-02-15_EPIC_00_story_02_unit_tests_core.md`

---

## Technical Design

### Files to Create

**1. `tests/unit/test_language_code.py`** (Primary deliverable)
- **Location**: `/home/mikekao/personal/subgen/tests/unit/test_language_code.py`
- **Purpose**: Exhaustive unit tests for LanguageCode enum
- **Size**: ~600-700 lines
- **Test count**: 70+ test cases
- **Coverage target**: 100% of language_code.py

### Files to Modify

**None** - This story only creates new test files, no production code changes.

---

### Integration Points

**Integration Point 1: LanguageCode Enum**
- **Location**: `language_code.py:3`
- **Class definition**: `class LanguageCode(Enum):`
- **Purpose**: Enum containing all supported languages
- **Total entries**: 102 (101 languages + LanguageCode.NONE)
- **Structure**: Each enum entry is a tuple of 5 elements:
  ```python
  ENGLISH = ("en", "eng", "eng", "English", "English")
  #          ^^^^  ^^^^^  ^^^^^  ^^^^^^^^^  ^^^^^^^^^
  #          ISO    ISO    ISO    English    Native
  #          639-1  639-2T 639-2B  name      name
  ```
- **Special case**: `NONE = (None, None, None, None, None)` for unknown languages

**Integration Point 2: from_iso_639_1() method**
- **Location**: `language_code.py:117-121`
- **Function signature**: 
  ```python
  @staticmethod
  def from_iso_639_1(code) -> LanguageCode:
  ```
- **Purpose**: Convert ISO 639-1 code (2 letters) to LanguageCode
- **Input**: String like `"en"`, `"ja"`, `"es"`
- **Output**: LanguageCode enum or LanguageCode.NONE
- **Algorithm**: Linear search through all enum entries
- **Example usage**:
  ```python
  lang = LanguageCode.from_iso_639_1("en")
  assert lang == LanguageCode.ENGLISH
  ```

**Integration Point 3: from_iso_639_2() method**
- **Location**: `language_code.py:124-128`
- **Function signature**: 
  ```python
  @staticmethod
  def from_iso_639_2(code) -> LanguageCode:
  ```
- **Purpose**: Convert ISO 639-2/T or 639-2/B code (3 letters) to LanguageCode
- **Input**: String like `"eng"`, `"jpn"`, `"chi"`, `"zho"`
- **Output**: LanguageCode enum or LanguageCode.NONE
- **Special behavior**: Accepts both T (terminological) and B (bibliographic) variants
- **Example**: `"chi"` (639-2/B) and `"zho"` (639-2/T) both return CHINESE
- **Example usage**:
  ```python
  lang1 = LanguageCode.from_iso_639_2("eng")  # Returns ENGLISH
  lang2 = LanguageCode.from_iso_639_2("chi")  # Returns CHINESE (B variant)
  lang3 = LanguageCode.from_iso_639_2("zho")  # Returns CHINESE (T variant)
  ```

**Integration Point 4: from_name() method**
- **Location**: `language_code.py:131-136`
- **Function signature**: 
  ```python
  @staticmethod
  def from_name(name: str) -> LanguageCode:
  ```
- **Purpose**: Convert English or native language name to LanguageCode
- **Input**: String like `"English"`, `"日本語"`, `"Español"`
- **Output**: LanguageCode enum or LanguageCode.NONE
- **Case sensitivity**: Case-insensitive (uses `.lower()`)
- **⚠️ KNOWN BUG**: Line 136 missing `return` statement! Returns `None` instead of `LanguageCode.NONE`
- **Example usage**:
  ```python
  lang1 = LanguageCode.from_name("English")   # Returns ENGLISH
  lang2 = LanguageCode.from_name("日本語")    # Returns JAPANESE
  lang3 = LanguageCode.from_name("unknown")   # BUG: Returns None, should return NONE
  ```

**Integration Point 5: from_string() method**
- **Location**: `language_code.py:140-158`
- **Function signature**: 
  ```python
  @staticmethod
  def from_string(value: str) -> LanguageCode:
  ```
- **Purpose**: Universal converter - tries all formats (ISO codes, names)
- **Input**: Any string representation of a language
- **Output**: LanguageCode enum or LanguageCode.NONE
- **Algorithm**: 
  1. If value is None, return LanguageCode.NONE
  2. Strip whitespace and convert to lowercase
  3. Check against iso_639_1, iso_639_2_t, iso_639_2_b, name_en, name_native
  4. Return first match or LanguageCode.NONE
- **Example usage**:
  ```python
  lang = LanguageCode.from_string("en")        # ISO 639-1
  lang = LanguageCode.from_string("eng")       # ISO 639-2
  lang = LanguageCode.from_string("English")   # English name
  lang = LanguageCode.from_string("日本語")    # Native name
  lang = LanguageCode.from_string("   ja  ")   # Strips whitespace
  lang = LanguageCode.from_string(None)        # Returns NONE
  ```

**Integration Point 6: is_valid_language() method**
- **Location**: `language_code.py:162-163`
- **Function signature**: 
  ```python
  @staticmethod
  def is_valid_language(language: str) -> bool:
  ```
- **Purpose**: Check if a string is a valid language code
- **Implementation**: Wrapper around from_string()
- **Returns**: `True` if valid, `False` if not
- **Example usage**:
  ```python
  assert LanguageCode.is_valid_language("en") == True
  assert LanguageCode.is_valid_language("xxx") == False
  assert LanguageCode.is_valid_language("English") == True
  ```

**Integration Point 7: Output methods**
- **Locations**: Lines 165-175
- **Methods**: `to_iso_639_1()`, `to_iso_639_2_t()`, `to_iso_639_2_b()`, `to_name(in_english=True)`
- **Purpose**: Convert LanguageCode back to various string formats
- **Example usage**:
  ```python
  lang = LanguageCode.ENGLISH
  assert lang.to_iso_639_1() == "en"
  assert lang.to_iso_639_2_t() == "eng"
  assert lang.to_iso_639_2_b() == "eng"
  assert lang.to_name(in_english=True) == "English"
  assert lang.to_name(in_english=False) == "English"
  
  lang = LanguageCode.JAPANESE
  assert lang.to_name(in_english=True) == "Japanese"
  assert lang.to_name(in_english=False) == "日本語"
  ```

**Integration Point 8: Special methods**
- **Locations**: Lines 176-198
- **Methods**: `__str__`, `__bool__`, `__eq__`
- **Purpose**: Enable natural Python operations on LanguageCode
- **Example usage**:
  ```python
  lang = LanguageCode.ENGLISH
  
  # __str__: Convert to string
  assert str(lang) == "English"
  assert str(LanguageCode.NONE) == "Unknown"
  
  # __bool__: Use in if statements
  if lang:  # Truthy for real languages
      print("Valid language")
  if not LanguageCode.NONE:  # Falsy for NONE
      print("This won't print")
  
  # __eq__: Compare with different types
  assert lang == LanguageCode.ENGLISH
  assert lang == "en"  # Compares using from_string()
  assert lang == "English"
  assert LanguageCode.NONE == None
  ```

---

## Test Strategy

### Test Organization

Organize tests into 8 classes, one per major feature:

```python
class TestFromIso6391:
    """Test from_iso_639_1() converter"""
    # 15+ tests

class TestFromIso6392:
    """Test from_iso_639_2() converter with T and B variants"""
    # 12+ tests

class TestFromName:
    """Test from_name() converter with English and native names"""
    # 10+ tests (will discover the return bug)

class TestFromString:
    """Test from_string() universal converter"""
    # 15+ tests

class TestIsValidLanguage:
    """Test is_valid_language() validator"""
    # 6+ tests

class TestOutputMethods:
    """Test to_iso_639_1(), to_iso_639_2_t(), to_iso_639_2_b(), to_name()"""
    # 10+ tests

class TestSpecialMethods:
    """Test __str__, __bool__, __eq__"""
    # 8+ tests

class TestLanguageCodeNone:
    """Test LanguageCode.NONE special case"""
    # 6+ tests
```

### Unit Tests (Detailed Specification)

**Class 1: TestFromIso6391 (15 tests)**

Test the `from_iso_639_1()` static method:

1. `test_from_iso_639_1_valid_codes` - Parametrized test with 10 valid codes:
   - Input: `"en"` → Output: `LanguageCode.ENGLISH`
   - Input: `"ja"` → Output: `LanguageCode.JAPANESE`
   - Input: `"es"` → Output: `LanguageCode.SPANISH`
   - Input: `"fr"` → Output: `LanguageCode.FRENCH`
   - Input: `"de"` → Output: `LanguageCode.GERMAN`
   - Input: `"zh"` → Output: `LanguageCode.CHINESE`
   - Input: `"ru"` → Output: `LanguageCode.RUSSIAN`
   - Input: `"ar"` → Output: `LanguageCode.ARABIC`
   - Input: `"hi"` → Output: `LanguageCode.HINDI`
   - Input: `"ko"` → Output: `LanguageCode.KOREAN`

2. `test_from_iso_639_1_invalid_code` - Test with invalid code:
   - Input: `"xx"` → Output: `LanguageCode.NONE`

3. `test_from_iso_639_1_empty_string` - Test with empty string:
   - Input: `""` → Output: `LanguageCode.NONE`

4. `test_from_iso_639_1_three_letter_code` - Test with 3-letter code (wrong format):
   - Input: `"eng"` → Output: `LanguageCode.NONE` (should use from_iso_639_2)

5. `test_from_iso_639_1_case_sensitivity` - Test case sensitivity:
   - Input: `"EN"` → Output: `LanguageCode.NONE` (case-sensitive!)

**Class 2: TestFromIso6392 (12 tests)**

Test the `from_iso_639_2()` static method:

1. `test_from_iso_639_2_t_variant` - Test T (terminological) variants:
   - Input: `"eng"` → Output: `LanguageCode.ENGLISH`
   - Input: `"jpn"` → Output: `LanguageCode.JAPANESE`
   - Input: `"zho"` → Output: `LanguageCode.CHINESE` (T variant)

2. `test_from_iso_639_2_b_variant` - Test B (bibliographic) variants:
   - Input: `"eng"` → Output: `LanguageCode.ENGLISH` (same as T)
   - Input: `"chi"` → Output: `LanguageCode.CHINESE` (B variant)
   - Input: `"ger"` → Output: `LanguageCode.GERMAN` (B variant "ger" vs T variant "deu")

3. `test_from_iso_639_2_both_variants_work` - Verify both T and B work for Chinese:
   ```python
   def test_from_iso_639_2_both_variants_work():
       t_variant = LanguageCode.from_iso_639_2("zho")  # T
       b_variant = LanguageCode.from_iso_639_2("chi")  # B
       assert t_variant == b_variant == LanguageCode.CHINESE
   ```

4. `test_from_iso_639_2_invalid_code` - Test with invalid 3-letter code:
   - Input: `"xxx"` → Output: `LanguageCode.NONE`

5. `test_from_iso_639_2_two_letter_code` - Test with 2-letter code (wrong format):
   - Input: `"en"` → Output: `LanguageCode.NONE`

**Class 3: TestFromName (10 tests) - WILL DISCOVER BUG**

Test the `from_name()` static method:

1. `test_from_name_english_names` - Test with English names:
   - Input: `"English"` → Output: `LanguageCode.ENGLISH`
   - Input: `"Japanese"` → Output: `LanguageCode.JAPANESE`
   - Input: `"Spanish"` → Output: `LanguageCode.SPANISH`

2. `test_from_name_native_names` - Test with native names:
   - Input: `"日本語"` → Output: `LanguageCode.JAPANESE`
   - Input: `"中文"` → Output: `LanguageCode.CHINESE`
   - Input: `"Español"` → Output: `LanguageCode.SPANISH`

3. `test_from_name_case_insensitive` - Test case insensitivity:
   - Input: `"english"` → Output: `LanguageCode.ENGLISH`
   - Input: `"ENGLISH"` → Output: `LanguageCode.ENGLISH`
   - Input: `"EnGLisH"` → Output: `LanguageCode.ENGLISH`

4. `test_from_name_invalid_name` - **BUG WILL BE FOUND HERE**:
   ```python
   def test_from_name_invalid_name():
       result = LanguageCode.from_name("InvalidLanguage")
       # ⚠️ THIS TEST WILL FAIL - Bug on line 136
       # Expected: LanguageCode.NONE
       # Actual: None (missing return statement)
       assert result == LanguageCode.NONE, f"Expected LanguageCode.NONE, got {result}"
   ```

**Class 4: TestFromString (15 tests)**

Test the `from_string()` universal converter:

1. `test_from_string_iso_639_1` - Test with ISO 639-1 codes:
   - Input: `"en"` → Output: `LanguageCode.ENGLISH`

2. `test_from_string_iso_639_2` - Test with ISO 639-2 codes:
   - Input: `"eng"` → Output: `LanguageCode.ENGLISH`
   - Input: `"chi"` → Output: `LanguageCode.CHINESE`

3. `test_from_string_english_name` - Test with English names:
   - Input: `"English"` → Output: `LanguageCode.ENGLISH`

4. `test_from_string_native_name` - Test with native names:
   - Input: `"日本語"` → Output: `LanguageCode.JAPANESE`

5. `test_from_string_with_whitespace` - Test whitespace handling:
   - Input: `"  en  "` → Output: `LanguageCode.ENGLISH`
   - Input: `"\ten\n"` → Output: `LanguageCode.ENGLISH`

6. `test_from_string_case_insensitive` - Test case handling:
   - Input: `"EN"` → Output: `LanguageCode.ENGLISH`
   - Input: `"ENGLISH"` → Output: `LanguageCode.ENGLISH`

7. `test_from_string_none_input` - Test None input:
   - Input: `None` → Output: `LanguageCode.NONE`

8. `test_from_string_empty_string` - Test empty string:
   - Input: `""` → Output: `LanguageCode.NONE`

9. `test_from_string_invalid` - Test invalid string:
   - Input: `"xxxxx"` → Output: `LanguageCode.NONE`

**Class 5: TestIsValidLanguage (6 tests)**

Test the `is_valid_language()` boolean validator:

1. `test_is_valid_language_with_valid_codes`:
   ```python
   @pytest.mark.parametrize("code", ["en", "eng", "English", "日本語", "ja"])
   def test_is_valid_language_with_valid_codes(code):
       assert LanguageCode.is_valid_language(code) == True
   ```

2. `test_is_valid_language_with_invalid_codes`:
   ```python
   @pytest.mark.parametrize("code", ["xx", "xxx", "InvalidLang", ""])
   def test_is_valid_language_with_invalid_codes(code):
       assert LanguageCode.is_valid_language(code) == False
   ```

**Class 6: TestOutputMethods (10 tests)**

Test conversion from LanguageCode back to strings:

1. `test_to_iso_639_1`:
   ```python
   def test_to_iso_639_1():
       assert LanguageCode.ENGLISH.to_iso_639_1() == "en"
       assert LanguageCode.JAPANESE.to_iso_639_1() == "ja"
       assert LanguageCode.SPANISH.to_iso_639_1() == "es"
       assert LanguageCode.CHINESE.to_iso_639_1() == "zh"
       assert LanguageCode.CANTONESE.to_iso_639_1() == "yue"  # 3-letter code
   ```

2. `test_to_iso_639_2_t`:
   ```python
   def test_to_iso_639_2_t():
       assert LanguageCode.ENGLISH.to_iso_639_2_t() == "eng"
       assert LanguageCode.CHINESE.to_iso_639_2_t() == "zho"  # T variant
   ```

3. `test_to_iso_639_2_b`:
   ```python
   def test_to_iso_639_2_b():
       assert LanguageCode.ENGLISH.to_iso_639_2_b() == "eng"
       assert LanguageCode.CHINESE.to_iso_639_2_b() == "chi"  # B variant
       assert LanguageCode.GERMAN.to_iso_639_2_b() == "ger"   # B variant
   ```

4. `test_to_name_in_english`:
   ```python
   def test_to_name_in_english():
       assert LanguageCode.ENGLISH.to_name(in_english=True) == "English"
       assert LanguageCode.JAPANESE.to_name(in_english=True) == "Japanese"
       assert LanguageCode.CHINESE.to_name(in_english=True) == "Chinese"
   ```

5. `test_to_name_in_native`:
   ```python
   def test_to_name_in_native():
       assert LanguageCode.ENGLISH.to_name(in_english=False) == "English"
       assert LanguageCode.JAPANESE.to_name(in_english=False) == "日本語"
       assert LanguageCode.CHINESE.to_name(in_english=False) == "中文"
       assert LanguageCode.RUSSIAN.to_name(in_english=False) == "Русский"
   ```

**Class 7: TestSpecialMethods (8 tests)**

Test Python special methods (`__str__`, `__bool__`, `__eq__`):

1. `test_str_representation`:
   ```python
   def test_str_representation():
       assert str(LanguageCode.ENGLISH) == "English"
       assert str(LanguageCode.JAPANESE) == "Japanese"
       assert str(LanguageCode.NONE) == "Unknown"
   ```

2. `test_bool_conversion_truthy`:
   ```python
   def test_bool_conversion_truthy():
       assert LanguageCode.ENGLISH  # Truthy
       assert LanguageCode.JAPANESE
       if LanguageCode.ENGLISH:
           pass  # Should execute
       else:
           pytest.fail("LanguageCode.ENGLISH should be truthy")
   ```

3. `test_bool_conversion_falsy`:
   ```python
   def test_bool_conversion_falsy():
       assert not LanguageCode.NONE  # Falsy
       if LanguageCode.NONE:
           pytest.fail("LanguageCode.NONE should be falsy")
   ```

4. `test_equality_with_language_code`:
   ```python
   def test_equality_with_language_code():
       assert LanguageCode.ENGLISH == LanguageCode.ENGLISH
       assert LanguageCode.ENGLISH != LanguageCode.SPANISH
       assert LanguageCode.JAPANESE == LanguageCode.JAPANESE
   ```

5. `test_equality_with_string`:
   ```python
   def test_equality_with_string():
       # __eq__ converts string using from_string()
       assert LanguageCode.ENGLISH == "en"
       assert LanguageCode.ENGLISH == "eng"
       assert LanguageCode.ENGLISH == "English"
       assert LanguageCode.JAPANESE == "ja"
       assert LanguageCode.JAPANESE == "日本語"
   ```

6. `test_equality_with_none`:
   ```python
   def test_equality_with_none():
       assert LanguageCode.NONE == None
       assert LanguageCode.ENGLISH != None
       assert not (LanguageCode.JAPANESE == None)
   ```

**Class 8: TestLanguageCodeNone (6 tests)**

Test the special `LanguageCode.NONE` case:

1. `test_none_to_iso_639_1_returns_none`:
   ```python
   def test_none_to_iso_639_1_returns_none():
       assert LanguageCode.NONE.to_iso_639_1() is None
   ```

2. `test_none_to_name_returns_none`:
   ```python
   def test_none_to_name_returns_none():
       assert LanguageCode.NONE.to_name() is None
   ```

3. `test_none_str_representation`:
   ```python
   def test_none_str_representation():
       assert str(LanguageCode.NONE) == "Unknown"
   ```

4. `test_none_is_falsy`:
   ```python
   def test_none_is_falsy():
       assert not LanguageCode.NONE
       assert bool(LanguageCode.NONE) == False
   ```

5. `test_none_equals_python_none`:
   ```python
   def test_none_equals_python_none():
       assert LanguageCode.NONE == None
   ```

6. `test_converters_return_none_for_invalid`:
   ```python
   def test_converters_return_none_for_invalid():
       assert LanguageCode.from_iso_639_1("xxx") == LanguageCode.NONE
       assert LanguageCode.from_iso_639_2("xxx") == LanguageCode.NONE
       assert LanguageCode.from_string("xxx") == LanguageCode.NONE
       assert LanguageCode.from_string(None) == LanguageCode.NONE
   ```

---

## Example Code

### Complete Test File Structure

```python
"""
Unit tests for language_code.py module.

Tests all 102 languages and LanguageCode.NONE across all conversion methods.
Achieves 100% code coverage of language_code.py.

Run with:
    pytest tests/unit/test_language_code.py -v
    pytest tests/unit/test_language_code.py --cov=language_code --cov-report=term-missing
"""

import pytest
from language_code import LanguageCode


class TestFromIso6391:
    """Test from_iso_639_1() converter for 2-letter ISO codes."""
    
    @pytest.mark.parametrize("code,expected", [
        ("en", LanguageCode.ENGLISH),
        ("ja", LanguageCode.JAPANESE),
        ("es", LanguageCode.SPANISH),
        ("fr", LanguageCode.FRENCH),
        ("de", LanguageCode.GERMAN),
        ("zh", LanguageCode.CHINESE),
        ("ru", LanguageCode.RUSSIAN),
        ("ar", LanguageCode.ARABIC),
        ("hi", LanguageCode.HINDI),
        ("ko", LanguageCode.KOREAN),
    ])
    def test_from_iso_639_1_valid_codes(self, code, expected):
        """Test from_iso_639_1() with valid 2-letter ISO codes."""
        assert LanguageCode.from_iso_639_1(code) == expected
    
    def test_from_iso_639_1_invalid_code(self):
        """Test from_iso_639_1() with invalid code returns NONE."""
        assert LanguageCode.from_iso_639_1("xx") == LanguageCode.NONE
    
    def test_from_iso_639_1_empty_string(self):
        """Test from_iso_639_1() with empty string returns NONE."""
        assert LanguageCode.from_iso_639_1("") == LanguageCode.NONE
    
    def test_from_iso_639_1_three_letter_code(self):
        """Test from_iso_639_1() with 3-letter code returns NONE."""
        # This method only accepts 2-letter codes
        assert LanguageCode.from_iso_639_1("eng") == LanguageCode.NONE
    
    def test_from_iso_639_1_case_sensitivity(self):
        """Test from_iso_639_1() is case-sensitive (uppercase fails)."""
        # ISO 639-1 codes are lowercase only
        assert LanguageCode.from_iso_639_1("EN") == LanguageCode.NONE


class TestFromIso6392:
    """Test from_iso_639_2() converter for 3-letter ISO codes."""
    
    @pytest.mark.parametrize("code,expected", [
        # T (terminological) variants
        ("eng", LanguageCode.ENGLISH),
        ("jpn", LanguageCode.JAPANESE),
        ("spa", LanguageCode.SPANISH),
        ("zho", LanguageCode.CHINESE),  # T variant for Chinese
        ("deu", LanguageCode.GERMAN),   # T variant for German
        # B (bibliographic) variants
        ("chi", LanguageCode.CHINESE),  # B variant for Chinese
        ("ger", LanguageCode.GERMAN),   # B variant for German
    ])
    def test_from_iso_639_2_valid_codes(self, code, expected):
        """Test from_iso_639_2() with valid 3-letter ISO codes (T and B)."""
        assert LanguageCode.from_iso_639_2(code) == expected
    
    def test_from_iso_639_2_both_variants_return_same_language(self):
        """Test that both T and B variants return the same LanguageCode."""
        t_variant = LanguageCode.from_iso_639_2("zho")  # T for Chinese
        b_variant = LanguageCode.from_iso_639_2("chi")  # B for Chinese
        assert t_variant == b_variant == LanguageCode.CHINESE
    
    def test_from_iso_639_2_invalid_code(self):
        """Test from_iso_639_2() with invalid code returns NONE."""
        assert LanguageCode.from_iso_639_2("xxx") == LanguageCode.NONE
    
    def test_from_iso_639_2_two_letter_code(self):
        """Test from_iso_639_2() with 2-letter code returns NONE."""
        assert LanguageCode.from_iso_639_2("en") == LanguageCode.NONE


class TestFromName:
    """Test from_name() converter for English and native language names."""
    
    @pytest.mark.parametrize("name,expected", [
        ("English", LanguageCode.ENGLISH),
        ("Japanese", LanguageCode.JAPANESE),
        ("Spanish", LanguageCode.SPANISH),
        ("日本語", LanguageCode.JAPANESE),
        ("中文", LanguageCode.CHINESE),
        ("Español", LanguageCode.SPANISH),
    ])
    def test_from_name_valid_names(self, name, expected):
        """Test from_name() with valid English and native names."""
        assert LanguageCode.from_name(name) == expected
    
    @pytest.mark.parametrize("name", [
        "english",   # lowercase
        "ENGLISH",   # uppercase
        "EnGLisH",   # mixed case
    ])
    def test_from_name_case_insensitive(self, name):
        """Test from_name() is case-insensitive."""
        assert LanguageCode.from_name(name) == LanguageCode.ENGLISH
    
    @pytest.mark.xfail(reason="Known bug: from_name() missing return statement on line 136")
    def test_from_name_invalid_name(self):
        """Test from_name() with invalid name returns NONE.
        
        ⚠️ THIS TEST WILL FAIL - Bug on line 136 of language_code.py
        The method has `LanguageCode.NONE` without `return`, so it returns None.
        
        Expected: LanguageCode.NONE
        Actual: None
        """
        result = LanguageCode.from_name("InvalidLanguageName")
        assert result == LanguageCode.NONE, \
            f"Expected LanguageCode.NONE, got {result} (type: {type(result)})"


class TestFromString:
    """Test from_string() universal converter."""
    
    @pytest.mark.parametrize("value,expected", [
        # ISO 639-1 (2-letter)
        ("en", LanguageCode.ENGLISH),
        ("ja", LanguageCode.JAPANESE),
        # ISO 639-2 (3-letter T and B)
        ("eng", LanguageCode.ENGLISH),
        ("chi", LanguageCode.CHINESE),
        ("zho", LanguageCode.CHINESE),
        # English names
        ("English", LanguageCode.ENGLISH),
        ("Japanese", LanguageCode.JAPANESE),
        # Native names
        ("日本語", LanguageCode.JAPANESE),
        ("中文", LanguageCode.CHINESE),
        # With whitespace
        ("  en  ", LanguageCode.ENGLISH),
        ("\tja\n", LanguageCode.JAPANESE),
        # Case variations
        ("EN", LanguageCode.ENGLISH),
        ("ENGLISH", LanguageCode.ENGLISH),
    ])
    def test_from_string_comprehensive(self, value, expected):
        """Test from_string() with all input formats."""
        assert LanguageCode.from_string(value) == expected
    
    def test_from_string_none_input(self):
        """Test from_string() with None returns NONE."""
        assert LanguageCode.from_string(None) == LanguageCode.NONE
    
    def test_from_string_empty_string(self):
        """Test from_string() with empty string returns NONE."""
        assert LanguageCode.from_string("") == LanguageCode.NONE
    
    def test_from_string_invalid(self):
        """Test from_string() with invalid string returns NONE."""
        assert LanguageCode.from_string("xxxxx") == LanguageCode.NONE
        assert LanguageCode.from_string("NotALanguage") == LanguageCode.NONE


class TestIsValidLanguage:
    """Test is_valid_language() boolean validator."""
    
    @pytest.mark.parametrize("code", [
        "en", "eng", "English", "日本語", "ja", "jpn", "Japanese"
    ])
    def test_is_valid_language_with_valid_codes(self, code):
        """Test is_valid_language() returns True for valid codes."""
        assert LanguageCode.is_valid_language(code) == True
    
    @pytest.mark.parametrize("code", [
        "xx", "xxx", "InvalidLang", "", "NotALanguage"
    ])
    def test_is_valid_language_with_invalid_codes(self, code):
        """Test is_valid_language() returns False for invalid codes."""
        assert LanguageCode.is_valid_language(code) == False


class TestOutputMethods:
    """Test output conversion methods."""
    
    def test_to_iso_639_1(self):
        """Test to_iso_639_1() returns correct 2-letter codes."""
        assert LanguageCode.ENGLISH.to_iso_639_1() == "en"
        assert LanguageCode.JAPANESE.to_iso_639_1() == "ja"
        assert LanguageCode.SPANISH.to_iso_639_1() == "es"
        assert LanguageCode.CHINESE.to_iso_639_1() == "zh"
        assert LanguageCode.CANTONESE.to_iso_639_1() == "yue"  # 3-letter exception
    
    def test_to_iso_639_2_t(self):
        """Test to_iso_639_2_t() returns T (terminological) variant."""
        assert LanguageCode.ENGLISH.to_iso_639_2_t() == "eng"
        assert LanguageCode.CHINESE.to_iso_639_2_t() == "zho"  # T variant
        assert LanguageCode.GERMAN.to_iso_639_2_t() == "deu"   # T variant
    
    def test_to_iso_639_2_b(self):
        """Test to_iso_639_2_b() returns B (bibliographic) variant."""
        assert LanguageCode.ENGLISH.to_iso_639_2_b() == "eng"  # Same as T
        assert LanguageCode.CHINESE.to_iso_639_2_b() == "chi"  # B variant
        assert LanguageCode.GERMAN.to_iso_639_2_b() == "ger"   # B variant
    
    def test_to_name_in_english(self):
        """Test to_name(in_english=True) returns English names."""
        assert LanguageCode.ENGLISH.to_name(in_english=True) == "English"
        assert LanguageCode.JAPANESE.to_name(in_english=True) == "Japanese"
        assert LanguageCode.CHINESE.to_name(in_english=True) == "Chinese"
        assert LanguageCode.RUSSIAN.to_name(in_english=True) == "Russian"
    
    def test_to_name_in_native(self):
        """Test to_name(in_english=False) returns native names."""
        assert LanguageCode.ENGLISH.to_name(in_english=False) == "English"
        assert LanguageCode.JAPANESE.to_name(in_english=False) == "日本語"
        assert LanguageCode.CHINESE.to_name(in_english=False) == "中文"
        assert LanguageCode.RUSSIAN.to_name(in_english=False) == "Русский"
        assert LanguageCode.ARABIC.to_name(in_english=False) == "العربية"


class TestSpecialMethods:
    """Test Python special methods (__str__, __bool__, __eq__)."""
    
    def test_str_representation(self):
        """Test __str__() returns English name."""
        assert str(LanguageCode.ENGLISH) == "English"
        assert str(LanguageCode.JAPANESE) == "Japanese"
        assert str(LanguageCode.SPANISH) == "Spanish"
        assert str(LanguageCode.NONE) == "Unknown"
    
    def test_bool_conversion_truthy(self):
        """Test __bool__() returns True for valid languages."""
        assert LanguageCode.ENGLISH
        assert LanguageCode.JAPANESE
        assert bool(LanguageCode.SPANISH) == True
        
        # Should work in if statements
        if LanguageCode.ENGLISH:
            pass  # Should execute
        else:
            pytest.fail("LanguageCode.ENGLISH should be truthy")
    
    def test_bool_conversion_falsy(self):
        """Test __bool__() returns False for NONE."""
        assert not LanguageCode.NONE
        assert bool(LanguageCode.NONE) == False
        
        # Should work in if statements
        if LanguageCode.NONE:
            pytest.fail("LanguageCode.NONE should be falsy")
    
    def test_equality_with_language_code(self):
        """Test __eq__() with other LanguageCode instances."""
        assert LanguageCode.ENGLISH == LanguageCode.ENGLISH
        assert LanguageCode.ENGLISH != LanguageCode.SPANISH
        assert LanguageCode.JAPANESE == LanguageCode.JAPANESE
        assert not (LanguageCode.ENGLISH == LanguageCode.GERMAN)
    
    def test_equality_with_string(self):
        """Test __eq__() with strings (uses from_string())."""
        assert LanguageCode.ENGLISH == "en"
        assert LanguageCode.ENGLISH == "eng"
        assert LanguageCode.ENGLISH == "English"
        assert LanguageCode.JAPANESE == "ja"
        assert LanguageCode.JAPANESE == "日本語"
        assert LanguageCode.CHINESE == "chi"
        assert LanguageCode.CHINESE == "zho"
    
    def test_equality_with_none(self):
        """Test __eq__() with Python None."""
        assert LanguageCode.NONE == None
        assert LanguageCode.ENGLISH != None
        assert not (LanguageCode.JAPANESE == None)


class TestLanguageCodeNone:
    """Test LanguageCode.NONE special case."""
    
    def test_none_to_iso_639_1_returns_none(self):
        """Test NONE.to_iso_639_1() returns Python None."""
        assert LanguageCode.NONE.to_iso_639_1() is None
    
    def test_none_to_iso_639_2_returns_none(self):
        """Test NONE.to_iso_639_2_*() returns Python None."""
        assert LanguageCode.NONE.to_iso_639_2_t() is None
        assert LanguageCode.NONE.to_iso_639_2_b() is None
    
    def test_none_to_name_returns_none(self):
        """Test NONE.to_name() returns Python None."""
        assert LanguageCode.NONE.to_name() is None
        assert LanguageCode.NONE.to_name(in_english=True) is None
        assert LanguageCode.NONE.to_name(in_english=False) is None
    
    def test_none_str_representation(self):
        """Test str(NONE) returns 'Unknown'."""
        assert str(LanguageCode.NONE) == "Unknown"
    
    def test_none_is_falsy(self):
        """Test NONE is falsy in boolean context."""
        assert not LanguageCode.NONE
        assert bool(LanguageCode.NONE) == False
    
    def test_none_equals_python_none(self):
        """Test NONE == None (Python None)."""
        assert LanguageCode.NONE == None
        assert None == LanguageCode.NONE
    
    def test_converters_return_none_for_invalid(self):
        """Test all converters return NONE for invalid input."""
        assert LanguageCode.from_iso_639_1("xxx") == LanguageCode.NONE
        assert LanguageCode.from_iso_639_2("xxx") == LanguageCode.NONE
        assert LanguageCode.from_string("xxx") == LanguageCode.NONE
        assert LanguageCode.from_string(None) == LanguageCode.NONE
        assert LanguageCode.from_string("") == LanguageCode.NONE


class TestAllLanguages:
    """Test that all 101 languages (excluding NONE) can be converted."""
    
    def test_all_languages_have_iso_639_1(self):
        """Test all languages (except those with 3-letter codes) have iso_639_1."""
        for lang in LanguageCode:
            if lang == LanguageCode.NONE:
                continue
            # Some languages (like Cantonese) have 3-letter "ISO 639-1" codes
            assert lang.iso_639_1 is not None, f"{lang.name} missing iso_639_1"
    
    def test_all_languages_roundtrip_iso_639_1(self):
        """Test converting to ISO 639-1 and back returns same language."""
        for lang in LanguageCode:
            if lang == LanguageCode.NONE:
                continue
            code = lang.to_iso_639_1()
            if code and len(code) == 2:  # Only test 2-letter codes
                result = LanguageCode.from_iso_639_1(code)
                assert result == lang, f"Roundtrip failed for {lang.name}: {code}"
    
    def test_all_languages_roundtrip_iso_639_2_t(self):
        """Test converting to ISO 639-2/T and back returns same language."""
        for lang in LanguageCode:
            if lang == LanguageCode.NONE:
                continue
            code = lang.to_iso_639_2_t()
            result = LanguageCode.from_iso_639_2(code)
            assert result == lang, f"Roundtrip failed for {lang.name}: {code}"
```

---

## Implementation Steps

### Step 1: Create test file

**Command**:
```bash
cd /home/mikekao/personal/subgen
touch tests/unit/test_language_code.py
```

### Step 2: Copy test code

Copy the complete test code from the "Example Code" section above into `tests/unit/test_language_code.py`.

### Step 3: Run tests and verify expected failure

**Command**:
```bash
pytest tests/unit/test_language_code.py -v
```

**Expected output**: 1 test should fail (test_from_name_invalid_name) due to known bug

### Step 4: Mark known bug test as xfail

The test file already has `@pytest.mark.xfail` on the bug test, so it won't fail the build.

### Step 5: Run with coverage

**Command**:
```bash
pytest tests/unit/test_language_code.py --cov=language_code --cov-report=term-missing
```

**Expected output**: Should show 100% coverage (or close to it)

### Step 6: Create work log

Document the bug found and all tests created.

---

## Definition of Done

- [ ] `tests/unit/test_language_code.py` created with 70+ tests
- [ ] All tests passing (except 1 xfail for known bug)
- [ ] 100% coverage of language_code.py achieved
- [ ] Bug documented in work log (from_name() line 136)
- [ ] Work log created at `docs/WORKLOGS/NNNN_2026-02-15_EPIC_00_story_02_unit_tests_core.md`
- [ ] Code committed with message: "EPIC_00 STORY_02: Add unit tests for language_code.py"

---

## Dependencies

**Depends On**: STORY_01 (pytest infrastructure)  
**Blocks**: None (other stories can proceed in parallel)

---

## Notes

### Known Bug Found

**Bug**: `language_code.py` line 136 missing `return` statement
**Impact**: `from_name("invalid")` returns `None` instead of `LanguageCode.NONE`
**Workaround**: Use `from_string()` instead, which works correctly
**Fix**: Add `return` before `LanguageCode.NONE` on line 136

### For Fresh College Grads

1. **Run one test at a time first**:
   ```bash
   pytest tests/unit/test_language_code.py::TestFromIso6391::test_from_iso_639_1_valid_codes -v
   ```

2. **Use `-k` to run tests matching a pattern**:
   ```bash
   pytest tests/unit/test_language_code.py -k "from_iso" -v
   ```

3. **Read parametrize test failures carefully** - They show which parameter failed

4. **The xfail marker** means "expected to fail" - Used for known bugs

---

**Created**: 2026-02-15  
**Last Updated**: 2026-02-15  
**Status**: Ready for Implementation
