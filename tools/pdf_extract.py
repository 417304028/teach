#!/usr/bin/env python3
"""
PDF 文本提取工具 —— 使用 pdfplumber 解码 Chromium Web 打印生成的 PDF。
用法: python pdf_extract.py <input.pdf> [--max-pages N]
输出: JSON，包含 text 字段。
依赖: pip install pdfplumber
"""

import json
import sys
import os

def main():
    if len(sys.argv) < 2:
        print(json.dumps({"error": "Usage: pdf_extract.py <input.pdf> [--max-pages N]"}))
        sys.exit(1)

    pdf_path = sys.argv[1]
    max_pages = None

    i = 2
    while i < len(sys.argv):
        if sys.argv[i] == "--max-pages" and i + 1 < len(sys.argv):
            max_pages = int(sys.argv[i + 1])
            i += 2
        else:
            i += 1

    if not os.path.exists(pdf_path):
        print(json.dumps({"error": f"File not found: {pdf_path}"}))
        sys.exit(1)

    try:
        import pdfplumber
    except ImportError:
        print(json.dumps({
            "error": "pdfplumber not installed. Run: pip install pdfplumber",
            "fallback": True
        }))
        sys.exit(2)

    text_parts = []
    pages_processed = 0
    total_chars = 0
    good_chars = 0

    try:
        with pdfplumber.open(pdf_path) as pdf:
            for page in pdf.pages:
                if max_pages is not None and pages_processed >= max_pages:
                    break
                page_text = page.extract_text()
                if page_text:
                    cleaned = clean_text(page_text)
                    if cleaned:
                        text_parts.append(cleaned)
                        for ch in cleaned:
                            total_chars += 1
                            if is_readable(ch):
                                good_chars += 1
                pages_processed += 1

    except Exception as e:
        print(json.dumps({"error": str(e)}))
        sys.exit(1)

    full_text = "\n".join(text_parts)
    good_ratio = good_chars / max(total_chars, 1)

    if good_ratio < 0.2:
        print(json.dumps({
            "error": "Low readable ratio",
            "text": full_text,
            "ratio": round(good_ratio, 3),
            "pages": pages_processed,
            "fallback": True
        }))
        sys.exit(3)

    print(json.dumps({
        "text": full_text,
        "pages": pages_processed,
        "ratio": round(good_ratio, 3)
    }))


def clean_text(text):
    lines = []
    for line in text.split("\n"):
        line = line.strip()
        if len(line) < 3:
            continue
        if _is_noise(line):
            continue
        readable = sum(1 for ch in line if is_readable(ch))
        if readable / max(len(line), 1) < 0.3:
            continue
        lines.append(line)
    return "\n".join(lines)


def _is_noise(line):
    lower = line.lower()
    noise_words = ["chromium", "agpl", "adobereader", "http://", "https://"]
    for w in noise_words:
        if w in lower:
            return True
    return False


def is_readable(ch):
    o = ord(ch)
    return (
        (0x4E00 <= o <= 0x9FFF) or
        (0x3400 <= o <= 0x4DBF) or
        (0x3000 <= o <= 0x303F) or
        (0xFF00 <= o <= 0xFFEF) or
        (0x2000 <= o <= 0x206F) or
        (0x0020 <= o <= 0x007E) or
        (0x00A0 <= o <= 0x00FF) or
        (0x0100 <= o <= 0x024F) or
        (ord('A') <= o <= ord('Z')) or
        (ord('a') <= o <= ord('z')) or
        (ord('0') <= o <= ord('9')) or
        o in (ord(' '), ord('\t'), ord('\n'), ord('\r'))
    )


if __name__ == "__main__":
    main()