import sys, zipfile, xml.etree.ElementTree as ET
W='{http://schemas.openxmlformats.org/wordprocessingml/2006/main}'
z=zipfile.ZipFile(sys.argv[1])
root=ET.fromstring(z.read('word/document.xml'))
body=root.find(W+'body')
def ptext(p):
    out=[]
    for el in p.iter():
        if el.tag==W+'t' and el.text: out.append(el.text)
        elif el.tag==W+'tab': out.append('\t')
        elif el.tag in (W+'br',W+'cr'): out.append('\n')
        elif el.tag.endswith('}drawing') or el.tag.endswith('}pict'): out.append('[IMG]')
    return ''.join(out)
def pstyle(p):
    pp=p.find(W+'pPr')
    if pp is not None:
        s=pp.find(W+'pStyle')
        if s is not None: return s.get(W+'val')
        if pp.find(W+'numPr') is not None: return 'LIST'
    return ''
lines=[]
def tbl(t,depth=0):
    lines.append('[TABLE]')
    for tr in t.findall(W+'tr'):
        cells=[]
        for tc in tr.findall(W+'tc'):
            parts=[]
            for c in tc:
                if c.tag==W+'p': parts.append(ptext(c))
                elif c.tag==W+'tbl': parts.append('<nested table>'); 
            cells.append(' / '.join(x for x in parts if x.strip()).replace('\n',' '))
        lines.append('| '+' | '.join(cells)+' |')
    lines.append('[/TABLE]')
for el in body:
    if el.tag==W+'p':
        t=ptext(el); s=pstyle(el)
        if not t.strip(): continue
        if s and 'Heading' in s or (s and s.lower().startswith('heading')): lines.append('\n## ['+s+'] '+t)
        elif s=='LIST' or (s and 'List' in s): lines.append('  - '+t)
        else: lines.append(t)
    elif el.tag==W+'tbl': tbl(el)
open(sys.argv[2],'w',encoding='utf-8').write('\n'.join(lines))
print(len(lines))
print([n for n in z.namelist() if not n.startswith('word/media')])
