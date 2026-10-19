"""Audit Go/Java D token finders, lexical parity and C#/C++ regressions."""
import argparse,json,pathlib,random
from js_ts_port_compare import run,digest
from brace_port_compare import json_rows,score
from go_java_port_compare import reference,type_score

def main(a):
    import os
    repo=pathlib.Path(__file__).resolve().parent.parent;w=pathlib.Path(a.work_dir).resolve();report=json.loads((w/'report.json').read_text());audit=w/'d-audit-probe.exe';assert audit.exists();checks={}
    env=dict(os.environ,GOPROXY='off',GOAMD64='v1',GOCACHE=str(pathlib.Path(os.environ['TEMP'])/'funcfinder-go-cache'))
    fixed={
      'go':'''package p
type (
 S[T any] struct {Value T}
 I interface {Method()}
 Alias = []int
 F func(int) int
)
func Generic[T interface{~[]byte|string}](v T) T {return v}
func(s S[T]) Method() struct{Value T} {return struct{Value T}{s.Value}}
func Asm(x int) (n int)
func Outer() {
 type Local struct{X int}
 _=func() string{return `fake func hidden(){}`}
}
''',
      'java':'''@interface A {int[] value() default {1,2};}
record R(int x) {R {if(x<0)throw new Error();} int f(){return x;}}
class C {
 @A({1,2}) public <T> T[] f(T x) throws Exception {return null;}
 void $name() {}
 void outer() {class L {int inner(){return 1;}} Runnable r=new Runnable(){public void run(){}};}
}
'''}
    fixture_dir=w/'fixtures';fixture_dir.mkdir(exist_ok=True);rng=random.Random(20261018)
    for lang in ['go','java']:
        first=fixture_dir/('fixed.'+lang);first.write_text(fixed[lang],encoding='utf-8');paths=[first]
        for i in range(80):
            extra=rng.choice(['// ignored func fake() {}','/* nested /* inside */ outside */','// имя 名字 é','/* ` { \' " */'])
            text=extra+'\n'+fixed[lang]
            p=fixture_dir/(str(i)+'.'+lang);p.write_text(text,encoding='utf-8');paths.append(p)
        for label,data in [('long',('//'+('x'*131072)+'\n'+fixed[lang]).encode()),('invalid',b'// \xff\xc0\xaf\xed\xa0\x80\n'+fixed[lang].encode())]:
            p=fixture_dir/(label+'.'+lang);p.write_bytes(data);paths.append(p)
        paths.extend(p for folder in ['test_examples','test_files'] for p in (repo/folder).glob('*.'+lang));manifest=w/(lang+'-fixtures.json');manifest.write_text(json.dumps([p.resolve().as_posix() for p in paths]))
        cfg=w/(lang+'-config.json');commands={n:[w/binary,cfg,manifest,'1'] for n,binary in [('go','go-probe.exe'),('d','d-probe.exe'),('audit','d-audit-probe.exe')]}
        for mode,flags in [('clean',['--clean']),('raw-clean',['--clean','--raw']),('find',[])]:
            rows={n:json_rows(run(cmd+flags)) for n,cmd in commands.items()}
            assert rows['d']==rows['audit'],(lang,mode,'audit mismatch')
            if mode!='find':assert rows['d']==rows['go'],(lang,mode,'Go mismatch')
            elif lang=='java':assert all(x['functions']==y['functions'] and [c[:2] for c in x['classes']]==[c[:2] for c in y['classes']] for x,y in zip(rows['d'],rows['go'])),(lang,'function/header parity')
            checks[lang+'-'+mode]=dict(files=len(paths),audit_parity=True,go_parity=rows['d']==rows['go'])
            if mode=='find':
                files,meta=reference(rows['d'][:1],lang,repo,w,env);quality=score(files,rows['d'][:1],lambda f,line:False);assert not any(quality['counts'][k] for k in ['missing','false_positive','wrong_end']),(lang,quality)
                types=type_score(files,rows['d'][:1]);assert not any(types['counts'][k] for k in ['missing','false_positive','wrong_end']),(lang,types)
                checks[lang+'-fixed-oracle']=dict(functions=quality['counts'],types=types['counts'],reference=meta)
    for p in report['projects']:
        lang=p['language'];label=pathlib.Path(p['root']).name+'-'+lang
        for mode,flags in [('clean',['--clean']),('raw-clean',['--clean','--raw']),('find',[])]:
            expected=json_rows((w/(label+'-'+mode+'-d.jsonl')).read_text(encoding='utf-8'));actual=json_rows(run([audit,w/(lang+'-config.json'),w/(label+'-manifest.json'),'1',*flags]));assert expected==actual,(label,mode)
        checks[label]=dict(files=p['files'],audit_parity=True)
    oldpath=repo/'build/cpp-tokens-20261018/report.json';old=json.loads(oldpath.read_text())
    for p in old['projects']:
        label=pathlib.Path(p['root']).name+'-'+p['language'];cfg=oldpath.parent/(p['language']+'-config.json');manifest=oldpath.parent/(label+'-manifest.json')
        for path,sha in p['input_sha256'].items():assert digest(path)==sha
        for mode,flags in [('clean',['--clean']),('raw-clean',['--clean','--raw']),('find',[])]:
            expected=json_rows((oldpath.parent/(label+'-'+mode+'-d.jsonl')).read_text(encoding='utf-8'))
            for binary in [w/'d-probe.exe',audit]:assert json_rows(run([binary,cfg,manifest,'1',*flags]))==expected,(label,mode,binary)
        checks['regression-'+label]=dict(files=p['files'],full_map_and_cleanup_equal=True)
    for name,sha in report['sources'].items():assert digest(repo/'benchmarks'/name)==digest(w/'sources'/name)==sha
    checks['sources']=report['sources'];checks['audit_binary_sha256']=digest(audit)
    (w/'verification.json').write_text(json.dumps(checks,indent=2));print(json.dumps(checks),flush=True)

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--work-dir',default='build/go-java-d-port-20261018');main(p.parse_args())
