# SoT extraction tools

Regenerate the derived copies of `docs/FOS_KUK_SoT_v1.docx` after the document changes:

```sh
python -I tools/sot/docx2txt.py docs/FOS_KUK_SoT_v1.docx docs/sot/FOS_KUK_SoT_v1.txt
python -I tools/sot/extract_images.py docs/FOS_KUK_SoT_v1.docx docs/ui-reference
```

- `docs/sot/FOS_KUK_SoT_v1.txt`: the text, with headings as `## [HeadingN]`, tables as `| a | b |` rows and images as `[IMG]`. Grep it by ID (FR-, BR-, DB-RULE-, AC-, TBD-).
- `docs/ui-reference/gambar-NNN.png`: each evidence screenshot, named by its caption number ("Gambar NN").

The `.docx` stays the source of truth; these files are only for searching and viewing.
