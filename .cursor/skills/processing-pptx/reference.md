# python-pptx API Reference

## Core Objects

| Object | Description |
|--------|-------------|
| `Presentation` | Top-level object, represents the .pptx file |
| `Slide` | A single slide |
| `SlideLayout` | Template for slide structure |
| `Shape` | Any object on a slide (text box, image, table, chart) |
| `TextFrame` | Text container within a shape |
| `Paragraph` | A paragraph within a TextFrame |
| `Run` | A run of text with uniform formatting |

## Common Slide Layouts (default template)

| Index | Name | Placeholders |
|-------|------|--------------|
| 0 | Title Slide | title, subtitle |
| 1 | Title and Content | title, body |
| 2 | Section Header | title, subtitle |
| 3 | Two Content | title, left body, right body |
| 4 | Comparison | title, left title, left body, right title, right body |
| 5 | Title Only | title |
| 6 | Blank | none |

## Units

```python
from pptx.util import Inches, Pt, Emu, Cm

Inches(1)   # 1 inch = 914400 EMU
Pt(12)      # 12 points (font size)
Cm(2.5)     # 2.5 centimeters
Emu(914400) # raw EMU value
```

## Text Formatting

```python
from pptx.util import Pt
from pptx.dml.color import RGBColor
from pptx.enum.text import PP_ALIGN

run = paragraph.add_run()
run.text = "Hello"
run.font.size = Pt(24)
run.font.bold = True
run.font.italic = True
run.font.color.rgb = RGBColor(0xFF, 0x00, 0x00)
run.font.name = "Arial"

paragraph.alignment = PP_ALIGN.CENTER  # LEFT, CENTER, RIGHT, JUSTIFY
```

## Shape Types

```python
from pptx.enum.shapes import MSO_SHAPE_TYPE

shape.shape_type == MSO_SHAPE_TYPE.PICTURE      # 13
shape.shape_type == MSO_SHAPE_TYPE.TABLE         # 19
shape.shape_type == MSO_SHAPE_TYPE.TEXT_BOX      # 17
shape.shape_type == MSO_SHAPE_TYPE.AUTO_SHAPE    # 1
shape.shape_type == MSO_SHAPE_TYPE.GROUP         # 6
shape.shape_type == MSO_SHAPE_TYPE.CHART         # 3
```

## Adding Shapes

```python
from pptx.util import Inches
from pptx.enum.shapes import MSO_SHAPE

# Text box
txBox = slide.shapes.add_textbox(Inches(1), Inches(1), Inches(5), Inches(1))
tf = txBox.text_frame
tf.text = "Hello World"

# Image
slide.shapes.add_picture("image.png", Inches(1), Inches(2), width=Inches(4))

# Auto shape
shape = slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, Inches(1), Inches(1), Inches(3), Inches(2))
shape.text = "Rectangle"

# Table
table_shape = slide.shapes.add_table(rows=3, cols=4, left=Inches(1), top=Inches(2), width=Inches(8), height=Inches(3))
table = table_shape.table
table.cell(0, 0).text = "Header"
```

## Working with Tables

```python
table = shape.table
table.rows[0].height = Inches(0.5)
table.columns[0].width = Inches(2)

for row_idx, row in enumerate(table.rows):
    for col_idx, cell in enumerate(row.cells):
        cell.text = f"R{row_idx}C{col_idx}"
        # Cell formatting
        cell.text_frame.paragraphs[0].font.size = Pt(10)
```

## Slide Transitions & Notes

```python
# Speaker notes
notes_slide = slide.notes_slide
notes_slide.notes_text_frame.text = "Speaker notes here"

# Reading notes
for slide in prs.slides:
    if slide.has_notes_slide:
        print(slide.notes_slide.notes_text_frame.text)
```

## Useful Patterns

### Iterate all text in presentation

```python
def all_text(prs):
    for slide in prs.slides:
        for shape in slide.shapes:
            if shape.has_text_frame:
                for para in shape.text_frame.paragraphs:
                    yield para.text
```

### Clone a slide (workaround - python-pptx has no native clone)

```python
import copy
from lxml import etree

def duplicate_slide(prs, slide_index):
    source = prs.slides[slide_index]
    layout = source.slide_layout
    new_slide = prs.slides.add_slide(layout)
    for shape in source.shapes:
        el = copy.deepcopy(shape._element)
        new_slide.shapes._spTree.append(el)
    return new_slide
```
