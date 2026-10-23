"""Score Goose named functions/types against a caller-built upstream Tree-sitter grammar.

Requires tree-sitter 0.25.2 in --runtime, and a shared library exporting
tree_sitter_goose (generated using Goose's pinned tree-sitter-cli 0.25.8).
No dependency installation, application execution or compiler specialization.
"""
import sys,pathlib,ctypes,json,subprocess,hashlib,re,collections,concurrent.futures,argparse,importlib.metadata
ap=argparse.ArgumentParser(description=__doc__)
for flag in ('root','binary','runtime','grammar-library','output'):ap.add_argument('--'+flag,required=True)
ap.add_argument('--jobs',type=int,default=8)
args=ap.parse_args()
r=pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0,str(pathlib.Path(args.runtime).resolve()))
from tree_sitter import Language,Parser
lib=ctypes.CDLL(str(pathlib.Path(args.grammar_library).resolve()));lib.tree_sitter_goose.restype=ctypes.c_void_p
caps=ctypes.pythonapi.PyCapsule_New;caps.restype=ctypes.py_object;caps.argtypes=[ctypes.c_void_p,ctypes.c_char_p,ctypes.c_void_p]
parser=Parser(Language(caps(lib.tree_sitter_goose(),b'tree_sitter.Language',None)))
root=pathlib.Path(args.root).resolve();exe=str(pathlib.Path(args.binary).resolve());counts=collections.Counter();files=[];tasks=[]
paths=subprocess.check_output(['git','-C',str(root),'ls-files','-z']).decode().split('\0')
for rel in paths:
 if not rel.endswith('.goose') or not rel.startswith(('stdlib/','samples/','bench/','test/')) or rel.startswith(('test/errors/','test/errors_tc/')):continue
 p=root/rel;data=p.read_bytes();tree=parser.parse(data);counts['input_files']+=1
 if tree.root_node.has_error:
  counts['excluded_files']+=1;files.append({'file':rel,'sha256':hashlib.sha256(data).hexdigest(),'excluded':'grammar_error'});continue
 truth=[]
 def visit(n):
  if n.type in ('function_declaration','extern_member','struct_declaration','enum_declaration','type_alias_declaration'):
   nn=n.child_by_field_name('name')
   if nn:
    name=re.sub(r'\s+','',data[nn.start_byte:nn.end_byte].decode())
    kind={'struct_declaration':'struct','enum_declaration':'enum','type_alias_declaration':'type'}.get(n.type,'function')
    truth.append({'name':name,'kind':kind,'start':n.start_point.row+1,'name_line':nn.start_point.row+1,'end':n.end_point.row+1})
  for c in n.named_children:visit(c)
 visit(tree.root_node)
 tasks.append((p,rel,truth,hashlib.sha256(data).hexdigest()))
def score(task):
 p,rel,truth,digest=task;findings=[];cnt=collections.Counter(accepted_files=1)
 for kind,flags in [('function',[]),('type',['--struct','--json'])]:
  proc=subprocess.run([exe,'--inp',str(p),'--source','goose','--map',*flags],capture_output=True,encoding='utf-8')
  if proc.returncode and 'No functions found' not in proc.stderr and 'No types found' not in proc.stderr:raise RuntimeError((rel,proc.stdout,proc.stderr))
  if kind=='type':got=json.loads(proc.stdout).get('types',[]) if proc.stdout.strip() else []
  else:got=[{'name':n.strip(),'start':int(s),'end':int(e)} for n,s,e in re.findall(r'([^;\n]+): (\d+)-(\d+)(?: \[one-line\])?;',proc.stdout)]
  targets=[t for t in truth if (t['kind']=='function')==(kind=='function')];cnt[kind+'_truth']+=len(targets);cnt[kind+'_candidate']+=len(got);used=set()
  for t in targets:
   matches=[i for i,g in enumerate(got) if i not in used and g['name']==t['name'] and t['start']<=g['start']<=t['name_line']]
   if not matches:cnt[kind+'_missing']+=1;findings.append({'category':'missing','truth':t});continue
   i=matches[0];used.add(i);cnt[kind+'_matched']+=1
   if t['end']!=got[i]['end']:cnt[kind+'_wrong_end']+=1;findings.append({'category':'wrong_end','truth':t,'candidate':got[i]})
  for i,g in enumerate(got):
   if i not in used:cnt[kind+'_extra']+=1;findings.append({'category':'extra','kind':kind,'candidate':g})
 return cnt,{'file':rel,'sha256':digest,'truth':truth,'findings':findings}
with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs) as pool:
 for cnt,item in pool.map(score,tasks):counts.update(cnt);files.append(item)
out=pathlib.Path(args.output);out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps({'counts':dict(counts),'tree_sitter':importlib.metadata.version('tree-sitter'),'grammar_library_sha256':hashlib.sha256(pathlib.Path(args.grammar_library).read_bytes()).hexdigest(),'scanner_sha256':hashlib.sha256((root/'tree-sitter/src/scanner.c').read_bytes()).hexdigest(),'goose_revision':subprocess.check_output(['git','-C',str(root),'rev-parse','HEAD']).decode().strip(),'binary_sha256':hashlib.sha256(pathlib.Path(exe).read_bytes()).hexdigest(),'grammar_sha256':hashlib.sha256((root/'tree-sitter/grammar.js').read_bytes()).hexdigest(),'files':files},indent=2),encoding='utf-8')
metadata=json.loads(out.read_text(encoding='utf-8'))
source_paths=sorted([p for base in ('internal','cmd') for p in (r/base).rglob('*.go') if not p.name.endswith('_test.go')]+[r/'internal/languages.json'])
metadata['funcfinder_revision']=subprocess.check_output(['git','-C',str(r),'rev-parse','HEAD']).decode().strip()
metadata['funcfinder_source_sha256']={p.relative_to(r).as_posix():hashlib.sha256(p.read_bytes()).hexdigest() for p in source_paths}
out.write_text(json.dumps(metadata,indent=2),encoding='utf-8')
print(json.dumps(dict(counts)))
for f in files:
 if f.get('findings'):print(json.dumps(f,ensure_ascii=False)[:1800])
