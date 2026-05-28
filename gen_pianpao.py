# -*- coding: utf-8 -*-
"""
平抛运动题目生成 - 最终版

策略：
1. PDF 是 Chromium 生成的图像型，无法提取文字
2. 直接用 DeepSeek 的物理知识生成高质量题目
3. 生成有答案/无答案两个版本
4. 输出到 data/output/YYYY-MM-DD/
"""
import os
import json
import subprocess
import re
from datetime import datetime

# Load .env
_script_dir = os.path.dirname(os.path.abspath(__file__))
_env_path = os.path.join(_script_dir, ".env")
if os.path.exists(_env_path):
    with open(_env_path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            if "=" in line:
                k, v = line.split("=", 1)
                os.environ.setdefault(k.strip(), v.strip())

import requests
import zipfile

# ============ 配置 ============
DEEPSEEK_API_KEY = os.environ.get("DEEPSEEK_API_KEY", "")
DEEPSEEK_BASE_URL = os.environ.get("DEEPSEEK_BASE_URL", "https://api.deepseek.com")
DEEPSEEK_MODEL = os.environ.get("DEEPSEEK_MODEL", "deepseek-v4-flash")
OUTPUT_BASE = "data/output"
TODAY_DIR = datetime.now().strftime("%Y-%m-%d")


# ============ DeepSeek API ============

def call_deepseek(system_prompt, user_prompt, temperature=0.5, max_tokens=4000, json_mode=True):
    if not DEEPSEEK_API_KEY:
        print("ERROR: DEEPSEEK_API_KEY not set")
        return None

    url = f"{DEEPSEEK_BASE_URL}/chat/completions"
    headers = {
        "Authorization": f"Bearer {DEEPSEEK_API_KEY}",
        "Content-Type": "application/json"
    }

    body = {
        "model": DEEPSEEK_MODEL,
        "messages": [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt}
        ],
        "temperature": temperature,
        "max_tokens": max_tokens,
    }
    if json_mode:
        body["response_format"] = {"type": "json_object"}

    print(f"  Calling DeepSeek ({DEEPSEEK_MODEL})...")
    resp = requests.post(url, headers=headers, json=body, timeout=180)
    if resp.status_code != 200:
        print(f"  API error {resp.status_code}: {resp.text[:200]}")
        return None

    data = resp.json()
    if "choices" in data and len(data["choices"]) > 0:
        return data["choices"][0]["message"]["content"]
    return None


# ============ DeepSeek 提示词 ============

EXERCISE_PROMPT = """请为高中物理「平抛运动」章节生成一套完整的练习题。

生成要求：
1. 题型：选择题3道、填空题2道、计算题3道
2. 难度：基础题占60%，提升题占40%
3. 每道计算题要有详细解答过程（包含受力分析、公式、代入计算）
4. 选择题要有4个选项
5. 涵盖平抛运动的核心知识点：分解为水平匀速+竖直自由落体、速度公式、位移公式、轨迹方程、平抛临界问题

请用以下JSON格式输出（必须是合法JSON）：
{
  "title": "平抛运动练习题",
  "topic": "平抛运动",
  "grade": "高一物理",
  "knowledge_points": ["知识点1", "知识点2", "知识点3"],
  "questions": [
    {
      "type": "选择题",
      "difficulty": "基础",
      "question": "题干内容",
      "options": ["A. 选项内容", "B. 选项内容", "C. 选项内容", "D. 选项内容"],
      "answer": "B",
      "solution": "解析过程"
    },
    {
      "type": "填空题",
      "difficulty": "基础",
      "question": "题干内容",
      "answer": "填空答案",
      "solution": "解析过程"
    },
    {
      "type": "计算题",
      "difficulty": "提升",
      "question": "题干内容（包含具体数值）",
      "answer": "最终答案（带单位）",
      "solution": "详细解答过程，包含：1. 受力分析 2. 建立坐标系 3. 列出方程 4. 代入计算 5. 结果"
    }
  ]
}"""


# ============ XML 转义 ============

def esc_xml(s):
    s = str(s)
    s = s.replace("&", "&amp;")
    s = s.replace("<", "&lt;")
    s = s.replace(">", "&gt;")
    s = s.replace('"', "&quot;")
    s = s.replace("'", "&apos;")
    return s


# ============ DOCX 生成 ============

def build_docx(title, sections):
    """构建 DOCX XML"""
    sections_xml = ""
    for sec_title, lines in sections:
        sections_xml += f"""
        <w:p>
          <w:pPr><w:pStyle w:val="Heading2"/></w:pPr>
          <w:r><w:t>{esc_xml(sec_title)}</w:t></w:r>
        </w:p>"""
        for line in lines:
            if line.strip():
                sections_xml += f"""
        <w:p>
          <w:r><w:t xml:space="preserve">{esc_xml(line)}</w:t></w:r>
        </w:p>"""

    doc_xml = f"""<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
  xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p>
      <w:pPr>
        <w:pStyle w:val="Heading1"/>
        <w:jc w:val="center"/>
      </w:pPr>
      <w:r><w:t>{esc_xml(title)}</w:t></w:r>
    </w:p>
{sections_xml}
    <w:sectPr/>
  </w:body>
</w:document>"""

    styles = """<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:rPr><w:rFonts w:eastAsia="宋体" w:ascii="宋体"/><w:sz w:val="24"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr><w:jc w:val="center"/><w:spacing w:before="480" w:after="240"/></w:pPr>
    <w:rPr><w:rFonts w:eastAsia="黑体" w:ascii="黑体" w:hAnsi="黑体"/><w:b/><w:sz w:val="36"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr><w:spacing w:before="360" w:after="120"/></w:pPr>
    <w:rPr><w:rFonts w:eastAsia="黑体" w:ascii="黑体" w:hAnsi="黑体"/><w:b/><w:sz w:val="28"/></w:rPr>
  </w:style>
</w:styles>"""

    content_types = """<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package/metadata/dc+xml"/>
</Types>"""

    rels = """<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="word/styles.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extendedProperties" Target="docProps/app.xml"/>
</Relationships>"""

    doc_rels = """<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>"""

    app_props = """<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>Hermesclaw</Application>
</Properties>"""

    core_props = f"""<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties">
  <dc:title xmlns:dc="http://purl.org/dc/elements/1.1/">{esc_xml(title)}</dc:title>
  <dcterms:created xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="dcterms:W3CDTF">{datetime.now().isoformat()}</dcterms:created>
</cp:coreProperties>"""

    return {
        "[Content_Types].xml": content_types,
        "_rels/.rels": rels,
        "docProps/app.xml": app_props,
        "docProps/core.xml": core_props,
        "word/document.xml": doc_xml,
        "word/styles.xml": styles,
        "word/_rels/document.xml.rels": doc_rels,
    }


def create_docx(output_path, title, sections):
    os.makedirs(os.path.dirname(output_path), exist_ok=True)
    with zipfile.ZipFile(output_path, 'w', zipfile.ZIP_DEFLATED) as zf:
        for name, content in build_docx(title, sections).items():
            zf.writestr(name, content.encode('utf-8'))
    print(f"  Created DOCX: {output_path}")
    return output_path


# ============ LibreOffice PDF 转换 ============

def find_libreoffice():
    paths = [
        "soffice", "libreoffice",
        "C:/Program Files/LibreOffice/program/soffice.exe",
        "C:/Program Files (x86)/LibreOffice/program/soffice.exe",
        "C:/Program Files/LibreOffice 24.8/program/soffice.exe",
        "C:/Program Files/LibreOffice 7/program/soffice.exe",
    ]
    for p in paths:
        try:
            r = subprocess.run([p, "--version"], capture_output=True, timeout=5)
            if r.returncode == 0:
                print(f"  Found LibreOffice: {p}")
                return p
        except Exception:
            pass
    return None


def convert_to_pdf(docx_path):
    soffice = find_libreoffice()
    if not soffice:
        print(f"  LibreOffice not found - skipping PDF conversion")
        return None

    out_dir = os.path.dirname(os.path.abspath(docx_path))
    try:
        env = {**os.environ, "HOME": "/tmp"}
        r = subprocess.run(
            [soffice, "--headless", "--convert-to", "pdf",
             "--outdir", out_dir, os.path.basename(docx_path)],
            cwd=out_dir, capture_output=True, timeout=60, env=env
        )
        pdf_path = os.path.splitext(docx_path)[0] + ".pdf"
        if os.path.exists(pdf_path):
            print(f"  Created PDF: {pdf_path}")
            return pdf_path
        else:
            print(f"  LibreOffice stderr: {r.stderr.decode('utf-8', errors='replace')[:200]}")
    except Exception as e:
        print(f"  LibreOffice failed: {e}")
    return None


# ============ 解析 JSON ============

def parse_questions(text):
    start = text.find('{')
    end = text.rfind('}') + 1
    if start == -1:
        return None
    s = text[start:end]
    try:
        return json.loads(s)
    except json.JSONDecodeError:
        pass
    # Fix common issues
    s = s.replace("「", '"').replace("」", '"')
    s = s.replace("‘", "'").replace("’", "'")
    s = s.replace("\n", " ")
    s = re.sub(r',\s*}', '}', s)
    s = re.sub(r',\s*]', ']', s)
    try:
        return json.loads(s)
    except:
        return None


# ============ 转为 DOCX Sections ============

def questions_to_sections(data, include_answers=True, for_homework=False):
    sections = []
    title = data.get("title", "平抛运动练习题")

    # 知识点回顾
    kp = data.get("knowledge_points", [])
    if kp:
        kp_lines = [f"{i+1}. {k}" for i, k in enumerate(kp)]
        sections.append(("核心知识点", kp_lines))

    # 练习题（无答案版 - 作业用）
    qs = data.get("questions", [])
    q_lines = []
    for i, q in enumerate(qs, 1):
        qtype = q.get("type", "选择题")
        diff = q.get("difficulty", "")
        tag = f"[{diff}] " if diff else ""
        q_lines.append("")
        q_lines.append(f"{i}. {tag}{q.get('question', '')}")
        if qtype == "选择题":
            for opt in q.get("options", []):
                q_lines.append(f"   {opt}")

    sections.append(("练习题", q_lines))

    # 参考答案（仅在有答案版）
    if include_answers:
        a_lines = []
        for i, q in enumerate(qs, 1):
            qtype = q.get("type", "")
            ans = q.get("answer", "")
            sol = q.get("solution", "")
            a_lines.append("")
            a_lines.append(f"第{i}题 [{qtype}]")
            if for_homework:
                a_lines.append(f"   答案: {ans}")
                if sol:
                    for sl in sol.split("\n"):
                        sl = sl.strip()
                        if sl:
                            a_lines.append(f"   {sl}")
            else:
                if ans:
                    a_lines.append(f"   答案: {ans}")
                if sol:
                    for sl in sol.split("\n"):
                        sl = sl.strip()
                        if sl:
                            a_lines.append(f"   {sl}")
        sections.append(("参考答案与解析", a_lines))

    return title, sections


# ============ 主流程 ============

def main():
    print("=" * 60)
    print("  平抛运动练习题生成")
    print("  时间: " + datetime.now().strftime("%Y-%m-%d %H:%M:%S"))
    print("=" * 60)

    # Step 1: DeepSeek 生成题目
    print("\n[Step 1] 调用 DeepSeek API 生成题目...")
    result_text = call_deepseek(
        "你是一位专业的高中物理教师，擅长设计练习题和编写详细解答。你的任务是生成高质量的物理练习题。",
        EXERCISE_PROMPT,
        temperature=0.5,
        max_tokens=4000,
        json_mode=True
    )

    if not result_text:
        print("DeepSeek API 调用失败！")
        return

    print(f"\n  DeepSeek 响应 ({len(result_text)} chars)")

    # 保存原始输出
    out_dir = os.path.join(OUTPUT_BASE, TODAY_DIR)
    os.makedirs(out_dir, exist_ok=True)
    raw_path = os.path.join(out_dir, "平抛运动_DeepSeek_raw.txt")
    with open(raw_path, "w", encoding="utf-8") as f:
        f.write(result_text)
    print(f"  保存原始响应: {raw_path}")

    # Step 2: 解析 JSON
    print("\n[Step 2] 解析题目...")
    data = parse_questions(result_text)
    if not data:
        print("  JSON 解析失败，保存原始结果")
        data = {"title": "平抛运动练习题", "knowledge_points": [], "questions": []}
    else:
        print(f"  解析成功：{len(data.get('questions', []))} 道题目")

    # 保存结构化 JSON
    json_path = os.path.join(out_dir, "平抛运动_练习题.json")
    with open(json_path, "w", encoding="utf-8") as f:
        f.write(json.dumps(data, ensure_ascii=False, indent=2))
    print(f"  保存 JSON: {json_path}")

    # 预览题目
    qs = data.get("questions", [])
    for i, q in enumerate(qs[:3], 1):
        print(f"  {i}. [{q.get('type','')}][{q.get('difficulty','')}] {q.get('question','')[:50]}...")

    # Step 3: 生成有答案版（练习题含答案）
    print("\n[Step 3] 生成练习题（含答案）...")
    title, sections = questions_to_sections(data, include_answers=True, for_homework=False)
    docx1 = os.path.join(out_dir, "平抛运动_练习题_含答案.docx")
    create_docx(docx1, title, sections)
    pdf1 = convert_to_pdf(docx1)

    # Step 4: 生成无答案版（课后作业）
    print("\n[Step 4] 生成课后作业（无答案）...")
    title2, sections2 = questions_to_sections(data, include_answers=True, for_homework=True)
    docx2 = os.path.join(out_dir, "平抛运动_课后作业_无答案.docx")
    create_docx(docx2, title2, sections2)
    pdf2 = convert_to_pdf(docx2)

    # Step 5: 生成纯作业版（仅题目，无答案区域）
    print("\n[Step 5] 生成纯作业版（仅题目）...")
    title3, sections3 = questions_to_sections(data, include_answers=False)
    docx3 = os.path.join(out_dir, "平抛运动_课后作业_纯题目.docx")
    create_docx(docx3, title3, sections3)
    pdf3 = convert_to_pdf(docx3)

    # 汇总
    print("\n" + "=" * 60)
    print("  生成完成！")
    print("=" * 60)
    print(f"\n  输出目录: {os.path.abspath(out_dir)}\n")
    for f in sorted(os.listdir(out_dir)):
        fp = os.path.join(out_dir, f)
        size = os.path.getsize(fp)
        ext = os.path.splitext(f)[1].upper()
        print(f"  {ext:8s} {f} ({size:,} bytes)")

    print(f"\n  注意: PDF 文件需要 LibreOffice 才能生成")
    if not find_libreoffice():
        print("  当前未检测到 LibreOffice，请安装后重新运行以生成 PDF")


if __name__ == "__main__":
    main()
