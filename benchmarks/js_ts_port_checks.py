"""Check JS/TS D-port fixtures, generated stress, raw cleanup and TS groups.

Requires the comparison harness binaries/configs and compile-audit.bat build.
"""
import argparse,hashlib,json,pathlib,random,subprocess

def main(a):
    repo=pathlib.Path(__file__).resolve().parent.parent;w=pathlib.Path(a.work_dir).resolve()
    def call(binary,lang,manifest,flags=()):
        r=subprocess.run([str(w/binary),str(w/f"{lang}-config.json"),str(manifest),"1",*flags],capture_output=True,text=True,encoding="utf-8",errors="replace")
        if r.returncode:raise RuntimeError(r.stderr[-4000:])
        return [json.loads(s) for s in r.stdout.splitlines()]
    checks={"fixtures":{},"corpora":{},"sources":{}}
    for lang in ["js","ts"]:
        suffixes=(".js",".jsx",".mjs",".cjs") if lang=="js" else (".ts",".tsx")
        paths=[p.resolve().as_posix() for folder in [repo/"test_examples",repo/"test_files"] for p in folder.rglob("*") if p.suffix in suffixes]
        source=w/f"stress.{lang}";lines=[]
        if lang=="js":
            lines=["class Box {", " constructor(){}", " ['computed'](){}", " 'literal'(){}", " #private(){}", " async method(x){return x;}","}"]
            for i in range(250):
                lines += [f"function outer{i}(){{", "const r=/[}\\/\"']+/g;", "const t=`text ${ {a: `nested ${'quoted'}`} }`;", f"function nested{i}(){{return 1;}}", "return t;}"]
            lines += ["let x=0;const arrow=(a)=>({value:a});function adjacent(){return 1;}","const f=function inner(){return inner;};const iife=(function hidden(){})();", "const obj={get value(){return 1;},async run(){},12(){}};", "function giant(){const s='"+"x"*131072+"';}"]
        else:
            lines=["interface Service {", "run(x: {id: string}): Promise<void>;", "}", "class Box {", "constructor(){}", "async method<T>(", " x:T", "):Promise<T> {return x;}", "}"]
            for i in range(250):
                lines += [f"export const typed{i}: (x: number) => number = (x) => {{", f"function child{i}(){{return x;}}", "return x;};"]
            lines += ["const value=(await query()).rows; const arrow=(x:number)=>x+1;", "const generic: React.FC<{", "children: string", "}> = ({children}) => {return children;};", "function giant(){const s='"+"x"*131072+"';}"]
        lines += ["/* nested /* ignored */ tail */", "const шаблон='Unicode';", "const jsx=<input value={x}/>;", "const broken=`${never", "still raw", "`;function recovered(){}"]
        # Mixed byte/rune coordinates: raw comments/literals have a different
        # UTF-8 length from their one-space-per-rune cleaned representations.
        if lang=='js':
            for i,name in enumerate(['é','ж','变量','𐐀','á','a\u200cb','a\u200db']):
                lines += [f"/* 一😀é */ const {name}{i} = (x) => {{return 'é😀\\\\';}};",f"const obj{i}={{'é😀'(){{return `𐐀 ${{'变量'}}`;}}}};"]
            lines += ["const whitespace='"+''.join(chr(c) for c in range(128) if c not in (10,13,39,92))+"';", "/* 😀 */ function byteBoundary(){return /[é😀]/u;} function afterBoundary(){}"]
        source.write_text("\n".join(lines)+"\n",encoding="utf-8");paths.append(source.as_posix())
        invalid=w/f"invalid-utf8.{lang}";invalid.write_bytes(b"const value='\xed\xa0\x80\xff';\nfunction afterInvalid(){return 1;}\n");paths.append(invalid.as_posix())
        manifest=w/f"{lang}-fixtures.json";manifest.write_text(json.dumps(paths),encoding="utf-8")
        for mode,flags in [("clean",["--clean"]),("raw_clean",["--clean","--raw"]),("find",[])]:
            baseline=call("go-probe.exe",lang,manifest,flags)
            for binary in ["d-probe.exe","d-audit-probe.exe"]:
                actual=call(binary,lang,manifest,flags)
                if actual!=baseline:
                    errors=[{"index":i,"go":x,"d":y} for i,(x,y) in enumerate(zip(baseline,actual)) if x!=y]
                    (w/f"{lang}-{mode}-fixture-errors.json").write_text(json.dumps(errors,ensure_ascii=False,indent=2),encoding="utf-8")
                    raise AssertionError((lang,mode,binary,errors[:1]))
            checks["fixtures"][lang+"-"+mode]={"files":len(paths),"parity":True}
    rng=random.Random(20261018)
    words=[' ','\t','\v','\n','\f','\r','\u00a0','function','export','default','async','const','let','var','public','private','protected','static','override','readonly','abstract','get','set','foo','bar','ж','١','é','_',':','=','=>','(',')','{','}','<','>',';',',','*','[',']']
    vectors=[''.join(rng.choice(words) for _ in range(rng.randrange(1,18))) for _ in range(50000)]
    for _ in range(50000):
        name=rng.choice(['name','ж','é','١','_']);t=rng.choice(['T','number','(x:T)=>U','A<{x:T}>',' ','A=>B','T=','{x:T;}']);prefix=rng.choice(['','export ','export default async ','public static ','async ']);body=rng.choice([' {',';',' => x','',':T {'])
        vectors.append(rng.choice([prefix+'function '+name+'<T>(',prefix+'const '+name+':'+t+' = async <T>(',prefix+name+'(x:T)'+body,prefix+name+'<T>(x:T):'+t+body]))
    manifest=w/'header-vectors.json';manifest.write_text(json.dumps(vectors,ensure_ascii=False),encoding='utf-8')
    assert call("go-probe.exe","ts",manifest,["--headers"])==call("d-probe.exe","ts",manifest,["--headers"])
    checks["ts_header_vectors"]={"cases":len(vectors),"all_groups_equal":True,"sha256":hashlib.sha256(manifest.read_bytes()).hexdigest()}
    report=json.loads((w/'report.json').read_text(encoding='utf-8'))
    for p in report['projects']:
        label=pathlib.Path(p['root']).name+'-'+p['language']; manifest=w/f'{label}-manifest.json'
        for mode,flags in [('clean',['--clean']),('find',[])]:
            expected=[json.loads(s) for s in (w/f'{label}-{mode}-go.jsonl').read_text(encoding='utf-8').splitlines()]
            assert call('d-audit-probe.exe',p['language'],manifest,flags)==expected,(label,mode)
        checks['corpora'][label]={'files':p['files'],'checked_parity':True}
    for name in report['sources']:
        actual=hashlib.sha256((repo/'benchmarks'/name).read_bytes()).hexdigest();assert actual==report['sources'][name],name;checks['sources'][name]=actual
    for lang in ['js','ts']:
        cfg=json.loads((w/f'{lang}-config.json').read_text());cfg['line_comment']='#';bad=w/f'{lang}-bad-config.json';bad.write_text(json.dumps(cfg))
        r=subprocess.run([str(w/'d-probe.exe'),str(bad),str(w/f'{lang}-fixtures.json'),'1'],capture_output=True,text=True)
        assert r.returncode!=0 and 'unsupported lexical configuration' in r.stderr
    (w/'verification.json').write_text(json.dumps(checks,indent=2),encoding='utf-8');print(json.dumps(checks),flush=True)

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--work-dir',default='build/js-ts-d-port-20261018');main(p.parse_args())
