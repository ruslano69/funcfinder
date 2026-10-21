"""Independent syntax-tree regression checks for Go/D accuracy fixes.

Requires tree-sitter 0.25.2 and C 0.24.1 / C++ 0.23.4 / Kotlin 1.1.0 / Ruby 0.23.1 grammar wheels.
Install those into --runtime; this script never installs packages itself.
"""
import argparse,sys,pathlib,json,subprocess,importlib.metadata
ap=argparse.ArgumentParser(description=__doc__)
ap.add_argument('--runtime',required=True)
ap.add_argument('--go',required=True)
ap.add_argument('--ffd',required=True)
args=ap.parse_args()
root=pathlib.Path(__file__).resolve().parent.parent
sys.path.insert(0,str(pathlib.Path(args.runtime).resolve()))
from tree_sitter import Language,Parser
import tree_sitter_c,tree_sitter_cpp,tree_sitter_kotlin,tree_sitter_ruby
fixtures={
'c':('struct S { int x; } __attribute__((aligned(16)));\n__attribute__((format(printf,1,2))) int logmsg(const char *fmt, ...);\nint f(void) __attribute__((unused));\n',['logmsg','f']),
'cpp':('void outer() {\n ss << uint(double(read) / delta);\n Event evt(Event::BEFORE, id,\n int64(size), timer.getCurrentTime());\n auto task = std::bind(std::forward<F>(f));\n}\nvoid valid(int x = timer.getCurrentTime());\n',['outer','valid']),
'kotlin':('class C {\n val state get() = flow.value\n fun f() {}\n val other get() = flow?.value\n suspend fun g() {}\n}\n',['f','g']),
'ruby':('def logger.warn(_event)\n sleep 30\nend\ndef self.[](key)\n key\nend\ndef Store::save(value)\n value\nend\n',['warn','[]','save'])}
mods={'c':tree_sitter_c,'cpp':tree_sitter_cpp,'kotlin':tree_sitter_kotlin,'ruby':tree_sitter_ruby}
out=root/'build/accuracy-fixed';out.mkdir(exist_ok=True)
summary={}
for lang,(source,wanted) in fixtures.items():
 data=source.encode();tree=Parser(Language(mods[lang].language())).parse(data)
 def walk(n):
  yield n
  for c in n.children:yield from walk(c)
 nodes=[n for n in walk(tree.root_node) if n.type in ('function_declarator','function_declaration','singleton_method')]
 print(lang,'errors',tree.root_node.has_error,[(n.type,n.start_point.row+1,source[n.start_byte:n.end_byte][:90]) for n in nodes])
 (out/f'{lang}-ast.txt').write_text(str(tree.root_node),encoding='utf-8')
 assert not tree.root_node.has_error,(lang,str(tree.root_node))
 names=[]
 starts=[]
 for n in nodes:
  child=n.child_by_field_name('declarator') or n.child_by_field_name('name')
  if child is None and lang=='kotlin':child=next(c for c in n.named_children if c.type=='simple_identifier' or c.type=='identifier')
  names.append(data[child.start_byte:child.end_byte].decode())
  starts.append(n.start_point.row+1)
 assert names==wanted,(lang,names,wanted)
 path=out/f'fixture.{lang}';path.write_bytes(data)
 exe=pathlib.Path(args.go if lang in ('cpp','ruby') else args.ffd).resolve()
 proc=subprocess.run([str(exe),'--inp',str(path),'--source',lang,'--map','--json'],capture_output=True)
 assert proc.returncode==0,(lang,proc.stderr)
 got=json.loads(proc.stdout)
 assert set(got)==set(wanted),(lang,got,wanted)
 if lang=='kotlin':assert [got[n]['start'] for n in wanted]==starts
 summary[lang]={'oracle_names':names,'oracle_lines':starts,'actual':got}
(out/'report.json').write_text(json.dumps({'fixtures':summary,'versions':{p:importlib.metadata.version(p) for p in ('tree-sitter','tree-sitter-c','tree-sitter-cpp','tree-sitter-kotlin','tree-sitter-ruby')}},ensure_ascii=False,indent=2),encoding='utf-8')
print('4 independent AST checks passed')
