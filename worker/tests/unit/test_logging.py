"""Tests for structured logging setup (LOG_FORMAT wiring)."""

import logging

from utils.logging import setup_logging


def _root_formatter():
    return logging.getLogger().handlers[0].formatter


def test_default_format_is_json():
    setup_logging(debug=False)
    fmt = _root_formatter()._fmt or ""
    assert '"level"' in fmt and '"message"' in fmt


def test_text_format_selected():
    setup_logging(debug=False, log_format="text")
    fmt = _root_formatter()._fmt or ""
    assert "json" not in fmt.lower()
    assert "%(levelname)s" in fmt and "%(message)s" in fmt
    # restore default for other tests
    setup_logging(debug=False)


def test_invalid_format_falls_back_to_json():
    setup_logging(debug=False, log_format="xml")
    fmt = _root_formatter()._fmt or ""
    assert '"level"' in fmt  # anything non-"text" uses the JSON formatter
    setup_logging(debug=False)
