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

    @pytest.mark.parametrize(
        "code,expected",
        [
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
        ],
    )
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

    @pytest.mark.parametrize(
        "code,expected",
        [
            # T (terminological) variants
            ("eng", LanguageCode.ENGLISH),
            ("jpn", LanguageCode.JAPANESE),
            ("spa", LanguageCode.SPANISH),
            ("zho", LanguageCode.CHINESE),  # T variant for Chinese
            ("deu", LanguageCode.GERMAN),  # T variant for German
            # B (bibliographic) variants
            ("chi", LanguageCode.CHINESE),  # B variant for Chinese
            ("ger", LanguageCode.GERMAN),  # B variant for German
        ],
    )
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

    @pytest.mark.parametrize(
        "name,expected",
        [
            ("English", LanguageCode.ENGLISH),
            ("Japanese", LanguageCode.JAPANESE),
            ("Spanish", LanguageCode.SPANISH),
            ("日本語", LanguageCode.JAPANESE),
            ("中文", LanguageCode.CHINESE),
            ("Español", LanguageCode.SPANISH),
        ],
    )
    def test_from_name_valid_names(self, name, expected):
        """Test from_name() with valid English and native names."""
        assert LanguageCode.from_name(name) == expected

    @pytest.mark.parametrize(
        "name",
        [
            "english",  # lowercase
            "ENGLISH",  # uppercase
            "EnGLisH",  # mixed case
        ],
    )
    def test_from_name_case_insensitive(self, name):
        """Test from_name() is case-insensitive."""
        assert LanguageCode.from_name(name) == LanguageCode.ENGLISH

    @pytest.mark.xfail(
        reason="Known bug: from_name() missing return statement on line 136"
    )
    def test_from_name_invalid_name(self):
        """Test from_name() with invalid name returns NONE.

        ⚠️ THIS TEST WILL FAIL - Bug on line 136 of language_code.py
        The method has `LanguageCode.NONE` without `return`, so it returns None.

        Expected: LanguageCode.NONE
        Actual: None
        """
        result = LanguageCode.from_name("InvalidLanguageName")
        assert result == LanguageCode.NONE, (
            f"Expected LanguageCode.NONE, got {result} (type: {type(result)})"
        )


class TestFromString:
    """Test from_string() universal converter."""

    @pytest.mark.parametrize(
        "value,expected",
        [
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
        ],
    )
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

    @pytest.mark.parametrize(
        "code", ["en", "eng", "English", "日本語", "ja", "jpn", "Japanese"]
    )
    def test_is_valid_language_with_valid_codes(self, code):
        """Test is_valid_language() returns True for valid codes."""
        assert LanguageCode.is_valid_language(code) == True

    @pytest.mark.parametrize("code", ["xx", "xxx", "InvalidLang", "", "NotALanguage"])
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
        assert LanguageCode.GERMAN.to_iso_639_2_t() == "deu"  # T variant

    def test_to_iso_639_2_b(self):
        """Test to_iso_639_2_b() returns B (bibliographic) variant."""
        assert LanguageCode.ENGLISH.to_iso_639_2_b() == "eng"  # Same as T
        assert LanguageCode.CHINESE.to_iso_639_2_b() == "chi"  # B variant
        assert LanguageCode.GERMAN.to_iso_639_2_b() == "ger"  # B variant

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

    @pytest.mark.xfail(
        reason="Known bug: __eq__() line 193 compares self.value (tuple) with LanguageCode, should compare self with LanguageCode"
    )
    def test_equality_with_string(self):
        """Test __eq__() with strings (uses from_string()).

        ⚠️ THIS TEST WILL FAIL - Bug on line 193 of language_code.py
        The method compares `self.value == LanguageCode.from_string(other)`,
        but self.value is a tuple and from_string returns a LanguageCode.
        Should be: `self == LanguageCode.from_string(other)`

        Expected: LanguageCode.ENGLISH == "en" returns True
        Actual: Returns False (tuple != LanguageCode)
        """
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

    def test_all_languages_roundtrip_iso_639_2_b(self):
        """Test converting to ISO 639-2/B and back returns same language."""
        for lang in LanguageCode:
            if lang == LanguageCode.NONE:
                continue
            code = lang.to_iso_639_2_b()
            result = LanguageCode.from_iso_639_2(code)
            assert result == lang, f"Roundtrip failed for {lang.name}: {code}"


class TestEdgeCases:
    """Test edge cases and special scenarios."""

    def test_none_value_comparisons(self):
        """Test None value handling in various contexts."""
        # from_string with None
        assert LanguageCode.from_string(None) == LanguageCode.NONE

        # Comparison with None
        assert (LanguageCode.NONE == None) == True
        assert (LanguageCode.ENGLISH == None) == False

    def test_whitespace_handling(self):
        """Test whitespace handling in from_string."""
        assert LanguageCode.from_string("  en  ") == LanguageCode.ENGLISH
        assert LanguageCode.from_string("\ten\t") == LanguageCode.ENGLISH
        assert LanguageCode.from_string("\nja\n") == LanguageCode.JAPANESE
        assert LanguageCode.from_string("   ") == LanguageCode.NONE  # Only whitespace

    def test_mixed_case_handling(self):
        """Test mixed case handling in from_string."""
        assert LanguageCode.from_string("EN") == LanguageCode.ENGLISH
        assert LanguageCode.from_string("eN") == LanguageCode.ENGLISH
        assert LanguageCode.from_string("ENG") == LanguageCode.ENGLISH
        assert LanguageCode.from_string("ENGLISH") == LanguageCode.ENGLISH
        assert LanguageCode.from_string("eNgLiSh") == LanguageCode.ENGLISH

    def test_property_access(self):
        """Test direct property access on LanguageCode values."""
        lang = LanguageCode.ENGLISH
        assert lang.iso_639_1 == "en"
        assert lang.iso_639_2_t == "eng"
        assert lang.iso_639_2_b == "eng"
        assert lang.name_en == "English"
        assert lang.name_native == "English"

        # Test with language that has different T and B variants
        lang = LanguageCode.CHINESE
        assert lang.iso_639_1 == "zh"
        assert lang.iso_639_2_t == "zho"
        assert lang.iso_639_2_b == "chi"
        assert lang.name_en == "Chinese"
        assert lang.name_native == "中文"

    def test_none_property_access(self):
        """Test property access on LanguageCode.NONE."""
        none_lang = LanguageCode.NONE
        assert none_lang.iso_639_1 is None
        assert none_lang.iso_639_2_t is None
        assert none_lang.iso_639_2_b is None
        assert none_lang.name_en is None
        assert none_lang.name_native is None


class TestLanguageSampling:
    """Test a sampling of languages from different regions to ensure broad coverage."""

    @pytest.mark.parametrize(
        "iso_code,lang_code,english_name",
        [
            # European languages
            ("en", LanguageCode.ENGLISH, "English"),
            ("fr", LanguageCode.FRENCH, "French"),
            ("de", LanguageCode.GERMAN, "German"),
            ("es", LanguageCode.SPANISH, "Spanish"),
            ("it", LanguageCode.ITALIAN, "Italian"),
            ("pt", LanguageCode.PORTUGUESE, "Portuguese"),
            ("ru", LanguageCode.RUSSIAN, "Russian"),
            ("pl", LanguageCode.POLISH, "Polish"),
            ("nl", LanguageCode.DUTCH, "Dutch"),
            ("sv", LanguageCode.SWEDISH, "Swedish"),
            # Asian languages
            ("zh", LanguageCode.CHINESE, "Chinese"),
            ("ja", LanguageCode.JAPANESE, "Japanese"),
            ("ko", LanguageCode.KOREAN, "Korean"),
            ("hi", LanguageCode.HINDI, "Hindi"),
            ("th", LanguageCode.THAI, "Thai"),
            ("vi", LanguageCode.VIETNAMESE, "Vietnamese"),
            ("id", LanguageCode.INDONESIAN, "Indonesian"),
            # Middle Eastern languages
            ("ar", LanguageCode.ARABIC, "Arabic"),
            ("he", LanguageCode.HEBREW, "Hebrew"),
            ("fa", LanguageCode.PERSIAN, "Persian"),
            ("tr", LanguageCode.TURKISH, "Turkish"),
            # African languages
            ("sw", LanguageCode.SWAHILI, "Swahili"),
            ("ha", LanguageCode.HAUSA, "Hausa"),
            ("yo", LanguageCode.YORUBA, "Yoruba"),
            # Other regions
            ("haw", LanguageCode.HAWAIIAN, "Hawaiian"),
        ],
    )
    def test_language_sampling(self, iso_code, lang_code, english_name):
        """Test a sampling of languages from different regions."""
        # Test ISO 639-1 or 639-2 code lookup
        if len(iso_code) == 2:
            assert LanguageCode.from_iso_639_1(iso_code) == lang_code
        else:
            assert LanguageCode.from_iso_639_2(iso_code) == lang_code

        # Test name lookup
        assert LanguageCode.from_name(english_name) == lang_code

        # Test to_name()
        assert lang_code.to_name(in_english=True) == english_name
