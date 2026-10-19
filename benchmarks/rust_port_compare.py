"""Compare the serial D Rust token finder with frozen Go and tree-sitter Rust."""
import argparse, bisect, importlib.metadata, json, os, pathlib, platform, random, re, statistics, subprocess, sys, time
from js_ts_port_compare import digest, run
from brace_port_compare import json_rows, score

SOURCES=['brace_port_unicode.d','brace_port_cleaner.d','brace_port_model.d','rust_port.d','rust_port_probe.d']
HELPERS=['rust_port_compare.py','brace_port_compare.py','js_ts_port_compare.py']

def build(repo,work,mode):
    snapshot=work/'sources';snapshot.mkdir(exist_ok=True)
    for name in SOURCES+HELPERS:(snapshot/name).write_bytes((repo/'benchmarks'/name).read_bytes())
    flags=['-O3','-release','-mcpu=x86-64'] if mode=='release' else ['-O2','-boundscheck=on','-d-version=AuditRustTokens']
    if mode=='unit':flags+=['-unittest']
    binary=work/('rust-'+mode+'.exe')
    command=['F:/D/ldc-1.43.0/bin/ldc2.exe',*flags,'-I='+str(snapshot),'-of='+str(binary),*[snapshot/n for n in SOURCES]]
    bat=work/('compile-'+mode+'.bat')
    bat.write_text('@echo off\ncall "F:\\D\\ldcvars64.bat" >nul\nif errorlevel 1 exit /b 1\n'+subprocess.list2cmdline(list(map(str,command)))+'\nexit /b %errorlevel%\n')
    (work/('compile-'+mode+'.txt')).write_text(run(['cmd','/d','/c',bat],cwd=repo))
    return binary,list(map(str,command))

def reference(paths):
    from tree_sitter import Language,Parser
    import tree_sitter_rust
    parser=Parser(Language(tree_sitter_rust.language()));files=[]
    types={'struct_item','enum_item','union_item','trait_item','type_item','associated_type'}
    for path in paths:
        code=pathlib.Path(path).read_bytes();tree=parser.parse(code)
        starts=[0]+[m.end() for m in re.finditer(b'\n',code)]
        def line(offset):return bisect.bisect_right(starts,offset)
        row=dict(path=path,functions=[],types=[],errors=[],macro_definitions=0,macro_invocations=0,macro_spans=[])
        if tree.root_node.has_error:
            stack=[tree.root_node]
            while stack:
                node=stack.pop()
                if node.type=='ERROR' or node.is_missing:
                    row['errors'].append(dict(kind=node.type,start=line(node.start_byte),end=line(max(node.start_byte,node.end_byte-1)),text=code[node.start_byte:node.end_byte].decode('utf-8',errors='replace')[:200]))
                stack.extend(reversed(node.named_children))
            assert row['errors']
            command=['rustfmt','--emit','stdout','--edition','2021',path]
            syntax=subprocess.run(command,capture_output=True,text=True,encoding='utf-8',errors='replace')
            row['rustfmt_syntax_check']=dict(command=command,returncode=syntax.returncode,stderr=syntax.stderr)
        else:
            stack=[tree.root_node]
            while stack:
                node=stack.pop()
                if node.type in {'macro_definition','macro_invocation'}:
                    row['macro_definitions' if node.type=='macro_definition' else 'macro_invocations']+=1
                    row['macro_spans'].append(dict(kind=node.type,start=line(node.start_byte),end=line(node.end_byte-1)))
                    continue
                if node.type in {'function_item','function_signature_item',*types}:
                    name=node.child_by_field_name('name')
                    if name is not None:
                        item=dict(name=code[name.start_byte:name.end_byte].decode('utf-8'),start=line(node.start_byte),name_line=line(name.start_byte),end=line(node.end_byte-1))
                        row['types' if node.type in types else 'functions'].append(item)
                stack.extend(reversed(node.named_children))
        files.append(row)
    return files

def quality(files,rows):
    functions=score(files,rows,lambda f,line:False,True)
    type_files=[dict(path=f['path'],errors=f['errors'],functions=f['types']) for f in files]
    type_rows=[dict(path=r['path'],functions=[[n,s,e,'',s,n] for n,s,e in r['classes']]) for r in rows]
    return dict(functions=functions,types=score(type_files,type_rows,lambda f,line:False,True))

def checks(work,config,release,audit):
    fixtures=work/'fixtures';fixtures.mkdir(exist_ok=True)
    cases={
        'nested.rs':("pub trait T {type Item;fn req(&self);fn body(){fn inner(){}}}\nimpl T for S {type Item=fn(i32)->i32;pub unsafe extern \"C\" fn run<const N:usize>(&self)->[u8;N] where Self:Sized {let c='}';}}\n",['req','body','inner','run']),
        'raw.rs':('/* outer\n/* inner */ fn hidden(){} */\nfn real(){let a=r########"fn fake(){} }"########;let b=br##"{"##;let c=c"}";let d=b\'{\';let e=\'\\u{7d}\';}\nfn r#type(){}\n',['real','r#type']),
        'lifetime.rs':("fn borrow<'a>(x: &'a str)-> &'a str {'again: loop {break 'again;} x}\n",['borrow']),
        'macro.rs':('macro_rules! item {()=>{fn fake(){}}}\ncall!{fn fake2(){}}\n#[cfg(any())] fn actual(){}\n',['actual']),
        'generic.rs':('fn generic<T: Fn() -> Vec<u8>>() -> impl Fn() -> i32 {|| 1}\nstruct Tuple(pub fn() -> i32);struct Unit;type Alias=fn(i32)->i32;\n',['generic']),
        'never.rs':('impl T for ! {fn method(&self){}}\nfn opaque()->impl Fn(){fn nested(){} || ()}\n',['method','opaque','nested']),
        'unicode.rs':('fn café(){let s="ж";}\n',['café']),
        'long.rs':('/*'+'x'*131072+'*/ fn long_name(){}\n',['long_name']),
        'deep.rs':('fn outer(){'+'{'*128+';'+'}'*128+'fn inner(){}}\n',['outer','inner']),
        'incomplete.rs':('fn unfinished<T(\n',[]),
    }
    paths=[]
    for name,(source,expected) in cases.items():
        path=fixtures/name;path.write_bytes(source.encode());paths.append(path.as_posix())
    path=fixtures/'invalid.rs';path.write_bytes(b'/*\xff\xfe*/fn valid(){}\n');paths.append(path.as_posix())
    manifest=work/'fixtures-manifest.json';manifest.write_text(json.dumps(paths))
    maps={n:json_rows(run([binary,config,manifest,'1'])) for n,binary in [('d',release),('audit',audit)]}
    assert maps['d']==maps['audit']
    for name,row in zip([*cases,'invalid.rs'],maps['d']):
        expected=cases[name][1] if name in cases else ['valid']
        assert [f[0] for f in row['functions']]==expected,(name,row)
    truth=reference([p for p in paths if pathlib.Path(p).name not in ['incomplete.rs','invalid.rs']])
    data=quality(truth,maps['d'])
    assert not any(data[k]['counts'][issue] for k in ['functions','types'] for issue in ['missing','false_positive','wrong_end','unparseable']),data
    return dict(files=len(paths),input_sha256={p:digest(p) for p in paths},independent=data,
                grammar_exclusions=['incomplete.rs','invalid.rs'],release_audit_equal=True)

def measure(a):
    repo=pathlib.Path(__file__).resolve().parent.parent;work=pathlib.Path(a.work_dir).resolve()
    if not work.is_relative_to(repo/'build'):raise ValueError('work directory must stay under build/')
    work.mkdir(parents=True,exist_ok=True);sys.path.insert(0,str(repo/'build/rust-eval-runtime'))
    old_path=repo/'build/go-java-d-port-20261018/report.json';old=json.loads(old_path.read_text());go=pathlib.Path(old['binaries']['go']['path'])
    assert digest(go)==old['binaries']['go']['sha256']
    unchanged={n:sha for n,sha in old['sources'].items() if n.endswith('.d')}
    c_report=json.loads((repo/'build/c-d-port-20261019/report.json').read_text())
    unchanged.update({n:sha for n,sha in c_report['sources'].items() if n.endswith('.d')})
    for name,sha in unchanged.items():assert digest(repo/'benchmarks'/name)==sha,name
    binaries={'go':go};commands={}
    for mode in ['release','audit','unit']:binaries[mode],commands[mode]=build(repo,work,mode)
    unit=subprocess.run([str(binaries['unit'])],capture_output=True,text=True,encoding='utf-8')
    (work/'unit.txt').write_text(unit.stdout+unit.stderr);assert unit.returncode==0,(unit.stdout,unit.stderr)
    config=work/'rust-config.json';config.write_text(run([go,'--config','rust']),encoding='utf-8')
    report=dict(scope='Serial Rust token finder; no macro expansion/cfg filtering/full CLI/file workers.',revision=old['revision'],
        frozen_go_report=dict(path=str(old_path),sha256=digest(old_path)),unchanged_prior_d_sources=unchanged,
        sources={n:digest(repo/'benchmarks'/n) for n in SOURCES+HELPERS},builds=commands,
        binaries={n:dict(path=str(b),sha256=digest(b),bytes=b.stat().st_size) for n,b in binaries.items()},
        python=platform.python_version(),platform=platform.platform(),go=old['go'],go_amd64='v1',
        ldc=run([commands['release'][0],'--version']).split('Registered Targets:')[0],ldc_sha256=digest(commands['release'][0]),
        tree_sitter=importlib.metadata.version('tree-sitter'),grammar=importlib.metadata.version('tree-sitter-rust'),
        rustfmt=run(['rustfmt','--version']).strip(),
        config_sha256=digest(config),samples=a.samples,repeats=a.repeats,seed=20261019,projects=[])
    report['checks']=checks(work,config,binaries['release'],binaries['audit'])
    output=work/'report.json'
    def save():output.write_text(json.dumps(report,indent=2))
    rng=random.Random(20261019)
    for label,root_text in (spec.split('=',1) for spec in a.project):
        root=pathlib.Path(root_text).resolve();git=['git','-c','safe.directory='+root.as_posix(),'-C',root]
        paths=[(root/p).as_posix() for p in run([*git,'-c','core.quotepath=false','ls-files','*.rs']).splitlines()]
        if label=='serde':paths=[p for p in paths if pathlib.Path(p).relative_to(root).parts[:2] in [('serde','src'),('serde_core','src')]]
        assert paths,('empty corpus',root)
        manifest=work/(label+'-manifest.json');manifest.write_text(json.dumps(paths))
        row=dict(label=label,root=str(root),pin=run([*git,'rev-parse','HEAD']).strip(),dirty=bool(run([*git,'status','--porcelain']).strip()),
            files=len(paths),bytes=sum(pathlib.Path(p).stat().st_size for p in paths),input_sha256={p:digest(p) for p in paths},manifest_sha256=digest(manifest),pathspec='serde/src and serde_core/src/**/*.rs' if label=='serde' else 'tracked *.rs')
        report['projects'].append(row)
        cmds={n:[b,config,manifest,'1'] for n,b in [('go',go),('d',binaries['release']),('audit',binaries['audit'])]}
        maps={n:json_rows(run(cmd)) for n,cmd in cmds.items()}
        assert maps['d']==maps['audit'] and all(len(r)==len(paths) for r in maps.values())
        for name,rows in maps.items():(work/(label+'-'+name+'.jsonl')).write_text('\n'.join(json.dumps(r,ensure_ascii=False) for r in rows)+'\n',encoding='utf-8')
        truth=reference(paths);(work/(label+'-truth.json')).write_text(json.dumps(truth,indent=2))
        row['quality']={n:quality(truth,maps[n]) for n in ['go','d']}
        row['macro_definitions']=sum(f['macro_definitions'] for f in truth);row['macro_invocations']=sum(f['macro_invocations'] for f in truth)
        bypath={str(pathlib.Path(f['path']).resolve()):f for f in truth}
        row['go_false_functions_in_macro_trees']=sum(any(s['start']<=f['candidate'][1]<=s['end'] for s in bypath[f['path']]['macro_spans']) for f in row['quality']['go']['functions']['findings'] if f['kind']=='false_positive')
        row['raw_counts']={n:dict(functions=sum(len(r['functions']) for r in rows),types=sum(len(r['classes']) for r in rows)) for n,rows in maps.items() if n!='audit'}
        save();print(label,'quality',json.dumps({n:{k:v['counts'] for k,v in q.items()} for n,q in row['quality'].items()}),flush=True)
        if any(row['quality']['d'][kind]['counts'][issue] for kind in ['functions','types'] for issue in ['missing','false_positive','wrong_end']):raise RuntimeError('Rust quality gate failed')
        samples={n:dict(parser_ms=[],process_ms=[]) for n in ['go','d']}
        for n in samples:run(cmds[n])
        for _ in range(a.samples):
            order=list(samples);rng.shuffle(order)
            for n in order:
                cmd=cmds[n];timed=json.loads(run(cmd[:-1]+[str(a.repeats),'--time']))
                expected=sum(len(r['functions'])+len(r['classes']) for r in maps[n])*a.repeats
                assert timed['checksum']==expected
                samples[n]['parser_ms'].append(timed['nanoseconds']/a.repeats/1e6)
                start=time.perf_counter();out=run(cmd);samples[n]['process_ms'].append((time.perf_counter()-start)*1e3)
                assert json_rows(out)==maps[n]
        row['timings']={n:{k:dict(median=statistics.median(v),min=min(v),max=max(v),samples=v) for k,v in kinds.items()} for n,kinds in samples.items()} if a.samples else {}
        assert row['input_sha256']=={p:digest(p) for p in paths};save();print(label,'timings',json.dumps(row['timings']),flush=True)
    for name,sha in report['sources'].items():assert digest(repo/'benchmarks'/name)==digest(work/'sources'/name)==sha,name
    for name,sha in unchanged.items():assert digest(repo/'benchmarks'/name)==sha,name
    for binary in report['binaries'].values():assert digest(binary['path'])==binary['sha256']
    for name,sha in report['checks']['input_sha256'].items():assert digest(name)==sha,name
    for row in report['projects']:
        assert row['input_sha256']=={p:digest(p) for p in row['input_sha256']}
        assert digest(work/(row['label']+'-manifest.json'))==row['manifest_sha256']
    assert digest(config)==report['config_sha256']
    report['postflight']='passed';save()

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--project',action='append',required=True,help='label=Git checkout; serde selects its two core source trees')
    p.add_argument('--work-dir',default='build/rust-d-port-20261019')
    p.add_argument('--samples',type=int,default=7);p.add_argument('--repeats',type=int,default=3)
    measure(p.parse_args())
