"""Check D C#/C++ fixtures, raw/Unicode cleanup and all configured header groups."""
import argparse,json,pathlib,random,subprocess,sys
from js_ts_port_compare import run,digest

def rows(text):return [json.loads(line) for line in text.split('\n') if line]

def main(a):
    repo=pathlib.Path(__file__).resolve().parent.parent;w=pathlib.Path(a.work_dir).resolve();stress=w/'fixtures';stress.mkdir(exist_ok=True)
    report=json.loads((w/'report.json').read_text());audit=w/'d-audit-probe.exe'
    binaries=[w/'go-probe.exe',w/'d-probe.exe',audit];assert all(p.exists() for p in binaries)
    fixed={
      'cs':'''namespace N;
[Serializable] public class C {
 public C(int x) : base() {}
 ~C() {}
 public static C operator +(C a, C b) => new C(1);
 public static implicit operator int(C c) => 1;
 public (int x, int y) Tuple<T>(T x) where T : class => (1,2);
 public int F() {
   int Local(int n) => n + 1;
   var s = @"text ""quoted"" { fake() }
   multiline";
   return Local(1);
 }
 public int P { get { return 1; } }
}
''',
      'cpp':'''class C {
public:
 C(int x)
   : member{x}, other(2)
 { }
 ~C() {}
 auto Trailing(int x) const -> int;
 virtual int Method(
    int x,
    int y
 ) const {return x+y;}
 int member,other;
};
auto C::Trailing(int x) const -> int {return x;}
template<class T> T Free(T value) {return value;}
'''}
    rng=random.Random(20261018);checks={}
    for lang in ['cs','cpp']:
        ext='.cs' if lang=='cs' else '.cpp';first=stress/('fixed'+ext);first.write_text(fixed[lang],encoding='utf-8')
        generated=[]
        for i in range(120):
            name=rng.choice(['f','имя','名字','\u1c89','\U0002ebf0','@class' if lang=='cs' else 'f_1']);sp=rng.choice([' ','\t','\u00a0','\v'])
            prefix='class C {\n' if lang=='cs' else 'struct C {\n'
            text=prefix+sp+'int '+name+'(int x) {\n'+rng.choice(['/* outer /* nested */ rest */','// ignored { fake()','var s = @"x ""y"" z";' if lang=='cs' else 'const char* s = "fake() {}";'])+'\nreturn x;\n}\n}\n'
            path=stress/(str(i)+ext);path.write_text(text,encoding='utf-8');generated.append(path)
        for label,text in [('long','//'+('x'*131072)+'\n'+fixed[lang]),('nested',('class C { int F() { '+('{ '*64)+'return 1; '+('} '*64)+'} }') if lang=='cs' else 'int F() { '+('{ '*64)+'return 1; '+('} '*64)+'}')]:
            path=stress/(label+ext);path.write_text(text);generated.append(path)
        for i,invalid in enumerate([b'\xff',b'\xc0\xaf',b'\xed\xa0\x80',b'\xf0\x80\x80\x80']):
            path=stress/('invalid'+str(i)+ext);path.write_bytes(b'// '+invalid+b'\n'+fixed[lang].encode());generated.append(path)
        fixtures=[str(p.resolve()) for folder in ['test_examples','test_files'] for p in (repo/folder).glob('*'+ext)]
        paths=[pathlib.Path(p).resolve().as_posix() for p in [first,*generated,*fixtures]];manifest=w/(lang+'-fixtures.json');manifest.write_text(json.dumps(paths))
        for label,extra in [('clean',['--clean']),('raw-clean',['--clean','--raw']),('find',[])]:
            expected=rows(run([binaries[0],w/(lang+'-config.json'),manifest,'1',*extra]))
            if lang=='cpp' and label=='find':expected=rows(run([binaries[1],w/(lang+'-config.json'),manifest,'1']))
            for binary in binaries[1:]:
                actual=rows(run([binary,w/(lang+'-config.json'),manifest,'1',*extra]))
                mismatches=[dict(path=paths[i],go=x,d=y) for i,(x,y) in enumerate(zip(expected,actual)) if x!=y]
                if mismatches:(w/(lang+'-fixture-mismatches.json')).write_text(json.dumps(mismatches[:10],indent=2,ensure_ascii=False),encoding='utf-8')
                assert not mismatches and len(actual)==len(expected)==len(paths),(lang,label,binary)
            if label=='find':
                names=[f[0] for f in expected[0]['functions']];required=['C','~C','F','Local','operator +','implicit operator int'] if lang=='cs' else ['C','~C','Trailing','Method','Free'];assert all(n in names for n in required),names
            checks[lang+'-'+label]=dict(files=len(paths),parity=True)
        cfg=json.loads((w/(lang+'-config.json')).read_text());vectors=[]
        words=[' ','\t','\n','\r','\f','\v','\u00a0','\u0085','void','int','char','bool','имя','é','名字','١','a','_','x','const','template','<','>','(',')','[',']','{','}','::',':',';','*','&','operator','new','=>','return','public','class','struct']
        for _ in range(10000):vectors.append(''.join(rng.choice(words) for _ in range(rng.randrange(1,22))))
        for _ in range(10000):
            name=rng.choice(['f','é','名字','\u1c89','\U0002ebf0','\U00010d40','_','١','operator +','operator new','~C']);space=rng.choice([' ','\t','\v','\u00a0']);arg=rng.choice(['','int x','const T& x','T','T*','inté','void']);tail=rng.choice([';',' {return 1;}',' const;',' noexcept -> T;',' = default;',''])
            vectors.append(rng.choice(['','template<class T> ','public '])+rng.choice(['void','int','T','std::vector<T>'])+space+name+'('+arg+')'+tail)
        manifest=w/(lang+'-header-vectors.json');manifest.write_text(json.dumps(vectors));keys=['func_pattern','class_pattern','header_open_pattern','constructor_pattern','declaration_pattern','member_declaration_pattern','conversion_operator_pattern'] if lang=='cpp' else ['func_pattern','class_pattern']
        for key in keys:
            if key not in cfg:continue
            probe=dict(cfg,probe_pattern=cfg[key]);config=w/(lang+'-'+key+'-config.json');config.write_text(json.dumps(probe))
            expected=rows(run([binaries[0],config,manifest,'1','--headers']))
            actual=rows(run([binaries[1],config,manifest,'1','--headers']))
            mismatches=[dict(text=v,go=x,d=y) for v,x,y in zip(vectors,expected,actual) if x!=y]
            if mismatches:(w/(lang+'-'+key+'-mismatches.json')).write_text(json.dumps(mismatches[:20],indent=2,ensure_ascii=False),encoding='utf-8')
            assert not mismatches,(lang,key,len(mismatches));checks[lang+'-'+key]=dict(cases=len(vectors),all_groups_equal=True)
        bad=dict(cfg,line_comment='#');path=w/(lang+'-bad-config.json');path.write_text(json.dumps(bad));r=subprocess.run([str(binaries[1]),str(path),str(manifest),'1'],capture_output=True,text=True);assert r.returncode!=0 and 'unsupported lexical configuration' in r.stderr
    for p in report['projects']:
        lang=p['language'];label=pathlib.Path(p['root']).name+'-'+lang
        for mode,extra in [('clean',['--clean']),('raw-clean',['--clean','--raw']),('find',[])]:
            variant='d' if lang=='cpp' and mode=='find' else 'go'
            expected=rows((w/(label+'-'+mode+'-'+variant+'.jsonl')).read_text(encoding='utf-8'))
            actual=rows(run([audit,w/(lang+'-config.json'),w/(label+'-manifest.json'),'1',*extra]));assert actual==expected,(label,mode)
        checks[label]=dict(files=p['files'],audit_parity=True)
    for name,sha in report['sources'].items():assert digest(repo/'benchmarks'/name)==digest(w/'sources'/name)==sha
    checks['sources']=report['sources'];checks['audit_binary_sha256']=digest(audit)
    (w/'verification.json').write_text(json.dumps(checks,indent=2));print(json.dumps(checks),flush=True)

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--work-dir',default='build/cpp-tokens-20261018');main(p.parse_args())
