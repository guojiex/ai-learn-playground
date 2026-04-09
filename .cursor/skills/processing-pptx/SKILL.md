---
name: processing-pptx
description: >-
  Read, create, and modify PowerPoint (.pptx) files using python-pptx.
  Use when the user mentions PPT, PowerPoint, PPTX, slides, presentations,
  or needs to extract content from or generate slide decks.
---

# PowerPoint (PPTX) Processing

## Prerequisites

Ensure `python-pptx` is installed:

```bash
pip install python-pptx Pillow
```

## Reading PPT Content

### Extract all text from a presentation

```python
from pptx import Presentation

def extract_text(pptx_path: str) -> list[dict]:
    """Extract text from all slides, returning list of {slide_num, shapes: [{name, text}]}."""
    prs = Presentation(pptx_path)
    result = []
    for i, slide in enumerate(prs.slides, 1):
        shapes = []
        for shape in slide.shapes:
            if shape.has_text_frame:
                shapes.append({"name": shape.name, "text": shape.text_frame.text})
            if shape.has_table:
                table = shape.table
                rows = []
                for row in table.rows:
                    rows.append([cell.text for cell in row.cells])
                shapes.append({"name": shape.name, "table": rows})
        result.append({"slide": i, "shapes": shapes})
    return result
```

### Extract images

```python
from pptx import Presentation
from pathlib import Path

def extract_images(pptx_path: str, output_dir: str = "extracted_images"):
    Path(output_dir).mkdir(exist_ok=True)
    prs = Presentation(pptx_path)
    count = 0
    for i, slide in enumerate(prs.slides, 1):
        for shape in slide.shapes:
            if shape.shape_type == 13:  # Picture
                image = shape.image
                ext = image.content_type.split("/")[-1]
                fname = f"slide{i}_img{count}.{ext}"
                with open(Path(output_dir) / fname, "wb") as f:
                    f.write(image.blob)
                count += 1
    return count
```

## Creating PPT Files

### Basic presentation

```python
from pptx import Presentation
from pptx.util import Inches, Pt
from pptx.enum.text import PP_ALIGN

def create_presentation(title: str, slides_data: list[dict], output_path: str):
    """
    slides_data: list of dicts with keys:
      - layout: "title", "title_content", "blank", "section"
      - title: str
      - content: str (bullet text, newline-separated for multiple bullets)
      - image_path: optional str
    """
    prs = Presentation()
    LAYOUTS = {
        "title": 0,        # Title Slide
        "title_content": 1, # Title and Content
        "section": 2,       # Section Header
        "blank": 6,         # Blank
    }

    # Title slide
    title_layout = prs.slide_layouts[0]
    slide = prs.slides.add_slide(title_layout)
    slide.shapes.title.text = title

    for data in slides_data:
        layout_idx = LAYOUTS.get(data.get("layout", "title_content"), 1)
        slide_layout = prs.slide_layouts[layout_idx]
        slide = prs.slides.add_slide(slide_layout)

        if slide.shapes.title and data.get("title"):
            slide.shapes.title.text = data["title"]

        if data.get("content") and len(slide.placeholders) > 1:
            tf = slide.placeholders[1].text_frame
            for line in data["content"].split("\n"):
                p = tf.add_paragraph()
                p.text = line
                p.font.size = Pt(18)

        if data.get("image_path"):
            slide.shapes.add_picture(
                data["image_path"], Inches(1), Inches(2), Inches(8), Inches(4)
            )

    prs.save(output_path)
```

## Modifying PPT Files

### Replace text in all slides

```python
from pptx import Presentation

def replace_text(pptx_path: str, output_path: str, replacements: dict[str, str]):
    """replacements: {old_text: new_text}"""
    prs = Presentation(pptx_path)
    for slide in prs.slides:
        for shape in slide.shapes:
            if shape.has_text_frame:
                for paragraph in shape.text_frame.paragraphs:
                    for run in paragraph.runs:
                        for old, new in replacements.items():
                            if old in run.text:
                                run.text = run.text.replace(old, new)
    prs.save(output_path)
```

### Add a slide to existing presentation

```python
from pptx import Presentation
from pptx.util import Pt

def add_slide(pptx_path: str, output_path: str, title: str, bullets: list[str]):
    prs = Presentation(pptx_path)
    layout = prs.slide_layouts[1]  # Title and Content
    slide = prs.slides.add_slide(layout)
    slide.shapes.title.text = title
    tf = slide.placeholders[1].text_frame
    for bullet in bullets:
        p = tf.add_paragraph()
        p.text = bullet
        p.font.size = Pt(18)
    prs.save(output_path)
```

## Utility Scripts

The `scripts/` directory contains ready-to-use tools:

- **`extract_pptx.py`**: Extract all text/tables from a PPTX file to JSON

```bash
python scripts/extract_pptx.py input.pptx
# Outputs: input_extracted.json
```

- **`pptx_summary.py`**: Print a human-readable summary of slide contents

```bash
python scripts/pptx_summary.py input.pptx
```

## Tips

- Always work on a **copy** of the original file when modifying.
- `python-pptx` cannot render slides to images; use LibreOffice CLI for that:
  ```bash
  libreoffice --headless --convert-to png --outdir output/ input.pptx
  ```
- For detailed API reference, see [reference.md](reference.md).
