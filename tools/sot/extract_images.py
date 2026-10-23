import re, sys, zipfile, os
docx, out = sys.argv[1], sys.argv[2]
z = zipfile.ZipFile(docx)
doc = z.read("word/document.xml").decode("utf-8")
rels = z.read("word/_rels/document.xml.rels").decode("utf-8")
target = dict(re.findall(r'Id="(rId\d+)"[^>]*Target="([^"]+)"', rels))
# walk paragraphs in order; remember last image rId, map the next "Gambar N" caption to it
pending = []
mapping = {}
for para in re.findall(r"<w:p[ >].*?</w:p>", doc, flags=re.S):
    for rid in re.findall(r'r:embed="(rId\d+)"', para):
        pending.append(rid)
    text = "".join(re.findall(r"<w:t[^>]*>([^<]*)</w:t>", para))
    m = re.match(r"\s*Gambar\s+(\d+)", text)
    if m and pending:
        mapping[int(m.group(1))] = pending[-1]
        pending = []
for n, rid in sorted(mapping.items()):
    src = "word/" + target[rid]
    ext = os.path.splitext(src)[1]
    with open(os.path.join(out, f"gambar-{n:03d}{ext}"), "wb") as f:
        f.write(z.read(src))
print(len(mapping), "images")
