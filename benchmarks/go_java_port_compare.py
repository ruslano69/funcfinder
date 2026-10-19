"""Compare D token finders for Go/Java with frozen Go and native/grammar references."""
import argparse,json,os,pathlib,platform,random,statistics,subprocess,sys,tarfile,time
from brace_port_compare import GO_PROBE,json_rows,score
from js_ts_port_compare import run,digest

SOURCES=['brace_port_unicode.d','brace_port_cleaner.d','brace_port_model.d','csharp_port.d','cpp_tokenizer.d','cpp_tokens_port.d','java_port.d','go_port.d','brace_port_probe.d']

def type_score(files,rows):
    converted=[dict(path=f['path'],errors=f.get('errors',[]),functions=f.get('classes',[])) for f in files]
    candidates=[dict(path=r['path'],functions=[[*c,'',0,''] for c in r['classes']]) for r in rows]
    return score(converted,candidates,lambda f,line:False)

def reference(rows,lang,repo,w,env):
    if lang=='go':
        source=w/'go_ast_oracle.go';source.write_bytes((repo/'benchmarks/go_ast_spans.go.txt').read_bytes());binary=w/'go-ast-oracle.exe';run(['go','build','-ldflags=-s -w','-o',binary,source],env=env)
        manifest=w/'go-oracle-inputs.json';manifest.write_text(json.dumps([r['path'] for r in rows]));data=json.loads(run([binary,manifest]));return data['files'],dict(engine='go/parser + go/ast',version=run(['go','version']).strip(),source_sha256=digest(source),binary_sha256=digest(binary),build_tags='not filtered')
    sys.path.insert(0,str(repo/'build/eval-runtime'))
    import tree_sitter,tree_sitter_java,importlib.metadata,bisect,re
    from scala_d_tree_eval import symbols
    parser=tree_sitter.Parser(tree_sitter.Language(tree_sitter_java.language()));files=[]
    for row in rows:
        code=pathlib.Path(row['path']).read_bytes();tree=parser.parse(code);classes=[];starts=[0]+[m.end() for m in re.finditer(b'\n',code)];stack=[tree.root_node]
        while stack:
            node=stack.pop()
            if node.type in ['class_declaration','interface_declaration','enum_declaration','record_declaration','annotation_type_declaration']:
                name=node.child_by_field_name('name');keyword=next((c for c in node.children if c.type in ['class','interface','enum','record','@interface','@']),node)
                if name is not None:classes.append(dict(name=code[name.start_byte:name.end_byte].decode(),start=bisect.bisect_right(starts,keyword.start_byte),name_line=bisect.bisect_right(starts,name.start_byte),end=bisect.bisect_right(starts,node.end_byte-1)))
            stack.extend(reversed(node.named_children))
        files.append(dict(path=row['path'],errors=['tree-sitter parse error'] if tree.root_node.has_error else [],functions=[] if tree.root_node.has_error else symbols(tree,code,'java'),classes=classes))
    return files,dict(engine='tree-sitter-java',tree_sitter=importlib.metadata.version('tree-sitter'),grammar=importlib.metadata.version('tree-sitter-java'))

def measure(a):
    repo=pathlib.Path(__file__).resolve().parent.parent;w=pathlib.Path(a.work_dir).resolve();w.mkdir(parents=True,exist_ok=True)
    if not w.is_relative_to(repo/'build'):raise ValueError('work directory must stay under build/')
    revision=run(['git','rev-parse',a.revision+'^{commit}']).strip();ref=w/'reference-src';pin=w/'reference-revision.txt'
    if pin.exists() and pin.read_text()!=revision:raise ValueError('use a new directory for another revision')
    if not ref.exists():
        archive=w/'reference.tar';run(['git','archive','--format=tar','-o',archive,revision]);ref.mkdir()
        with tarfile.open(archive) as t:t.extractall(ref,filter='data')
        pin.write_text(revision)
    env=dict(os.environ,GOPROXY='off',GOAMD64='v1',GOCACHE=str(pathlib.Path(os.environ['TEMP'])/'funcfinder-go-cache'))
    source=ref/'build/brace_port_probe.go';source.parent.mkdir(exist_ok=True);source.write_text(GO_PROBE);go=w/'go-probe.exe';run(['go','build','-ldflags=-s -w','-o',go,source],cwd=ref,env=env)
    snapshot=w/'sources';snapshot.mkdir(exist_ok=True);helpers=['go_java_port_compare.py','go_java_port_checks.py','go_ast_spans.go.txt','brace_port_compare.py','js_ts_port_compare.py','scala_d_tree_eval.py']
    for name in SOURCES+helpers:(snapshot/name).write_bytes((repo/'benchmarks'/name).read_bytes())
    d=w/('d-audit-probe.exe' if a.audit else 'd-probe.exe');flags=['-O2','-boundscheck=on','-d-version=AuditBraceCleaner','-d-version=AuditCppTokens','-d-version=AuditBraceTokens'] if a.audit else ['-O3','-release','-mcpu=x86-64']
    command=['F:/D/ldc-1.43.0/bin/ldc2.exe',*flags,'-I='+str(snapshot),'-of='+str(d),*[snapshot/name for name in SOURCES]]
    bat=w/('compile-audit.bat' if a.audit else 'compile.bat');bat.write_text('@echo off\ncall "F:\\D\\ldcvars64.bat" >nul\nif errorlevel 1 exit /b 1\n'+subprocess.list2cmdline(list(map(str,command)))+'\nexit /b %errorlevel%\n');(w/('audit-build.txt' if a.audit else 'build.txt')).write_text(run(['cmd','/d','/c',bat]))
    report=dict(revision=revision,go=run(['go','version']).strip(),platform=platform.platform(),python=platform.python_version(),logical_cpus=os.cpu_count(),go_amd64='v1',ldc=run([command[0],'--version']).split('Registered Targets:')[0].strip(),build_command=list(map(str,command)),go_probe_sha256=digest(source),sources={n:digest(snapshot/n) for n in SOURCES+helpers},binaries={n:dict(path=str(p),bytes=p.stat().st_size,sha256=digest(p)) for n,p in [('go',go),('d',d)]},samples=a.samples,repeats=a.repeats,seed=20261018,audit=a.audit,scope='Serial Go/Java token finder API, not a complete CLI or file workers; generic Go versus specialized D.',projects=[])
    output=w/('audit-report.json' if a.audit else 'report.json')
    def save():output.write_text(json.dumps(report,indent=2))
    rng=random.Random(20261018)
    for spec in a.project:
        lang,path=spec.split('=',1);root=pathlib.Path(path).resolve();label=root.name+'-'+lang;git=['git','-c','safe.directory='+root.as_posix(),'-C',root];suffix='.go' if lang=='go' else '.java'
        isgit=subprocess.run([*map(str,git),'rev-parse','--show-toplevel'],capture_output=True).returncode==0
        if isgit:
            paths=[(root/p).as_posix() for p in run([*git,'-c','core.quotepath=false','ls-files','*'+suffix]).splitlines()];project_pin=run([*git,'rev-parse','HEAD']).strip();dirty=bool(run([*git,'status','--porcelain']).strip())
        else:
            paths=[p.as_posix() for p in sorted(root.rglob('*'+suffix))];project_pin='installed '+run(['go','version']).strip();dirty=None
        manifest=w/(label+'-manifest.json');manifest.write_text(json.dumps(paths));config=w/(lang+'-config.json');config.write_text(run([go,'--config',lang]),encoding='utf-8')
        p=dict(language=lang,root=str(root),pin=project_pin,dirty=dirty,files=len(paths),bytes=sum(pathlib.Path(f).stat().st_size for f in paths),input_sha256={f:digest(f) for f in paths});report['projects'].append(p);commands={n:[binary,config,manifest,'1'] for n,binary in [('go',go),('d',d)]}
        for mode,extra in [('clean',['--clean']),('raw-clean',['--clean','--raw']),('find',[])]:
            maps={n:json_rows(run(cmd+extra)) for n,cmd in commands.items()}
            for n,rows in maps.items():(w/(label+'-'+mode+'-'+n+'.jsonl')).write_text('\n'.join(json.dumps(r,ensure_ascii=False) for r in rows)+'\n',encoding='utf-8')
            mismatch=[dict(path=paths[i],go=x,d=y) for i,(x,y) in enumerate(zip(maps['go'],maps['d'])) if x!=y];p[mode+'_parity']=not mismatch and len(maps['go'])==len(maps['d'])==len(paths)
            if mismatch:(w/(label+'-'+mode+'-mismatches.json')).write_text(json.dumps(mismatch[:20],indent=2,ensure_ascii=False),encoding='utf-8')
            save();print(label,mode,'parity',p[mode+'_parity'],flush=True)
            if not p[mode+'_parity'] and mode!='find':raise RuntimeError('map mismatch: '+label+' '+mode)
            if mode=='find':
                p['function_parity']=all(x['functions']==y['functions'] for x,y in zip(maps['go'],maps['d']))
                p['type_header_parity']=all([c[:2] for c in x['classes']]==[c[:2] for c in y['classes']] for x,y in zip(maps['go'],maps['d']))
                if lang=='java':assert p['function_parity'] and p['type_header_parity'],'Java function/header parity regression'
                p.update(functions=sum(len(r['functions']) for r in maps['d']),classes=sum(len(r['classes']) for r in maps['d']))
                files,metadata=reference(maps['d'],lang,repo,w,env);(w/(label+'-truth.json')).write_text(json.dumps(files,indent=2));p['reference']=metadata
                p['oracle']=score(files,maps['d'],lambda f,line:False);p['go_oracle']=score(files,maps['go'],lambda f,line:False)
                p['types']=type_score(files,maps['d']);p['go_types']=type_score(files,maps['go']);save();print(label,'types',p['types']['counts'],flush=True)
                assert not any(p['types']['counts'][k] for k in ['missing','false_positive','wrong_end']),'type quality mismatch'
                issues=lambda d:{(f['path'],f['node']['name'],f['node']['start'],f['node']['end']) for f in d['findings'] if f['kind'] in ['missing','wrong_end']}
                p['quality_no_regression']=issues(p['oracle'])<=issues(p['go_oracle']) and all(p['oracle']['counts'][k]<=p['go_oracle']['counts'][k] for k in ['missing','false_positive','wrong_end']);save();print(label,'oracle',p['oracle']['counts'],'Go',p['go_oracle']['counts'],flush=True);assert p['quality_no_regression'],'function quality regression'
            if mode!='find' or not a.samples:continue
            samples={n:dict(parser_ms=[],process_ms=[]) for n in commands}
            for cmd in commands.values():run(cmd)
            for _ in range(a.samples):
                order=list(commands);rng.shuffle(order)
                for n in order:
                    cmd=commands[n];timed=json.loads(run(cmd[:-1]+[str(a.repeats),'--time']));expected=sum(len(r['functions'])+len(r['classes']) for r in maps[n]);assert timed['checksum']==expected*a.repeats;samples[n]['parser_ms'].append(timed['nanoseconds']/a.repeats/1e6)
                    start=time.perf_counter();out=run(cmd);samples[n]['process_ms'].append((time.perf_counter()-start)*1e3);assert json_rows(out)==maps[n]
            p['timings']={n:{k:dict(median=statistics.median(v),min=min(v),max=max(v),samples=v) for k,v in kinds.items()} for n,kinds in samples.items()};print(label,'timings',p['timings'],flush=True)
        assert p['input_sha256']=={f:digest(f) for f in paths};save()
    for n,sha in report['sources'].items():assert digest(repo/'benchmarks'/n)==sha

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--project',action='append',required=True);p.add_argument('--revision',default='bdbc81b');p.add_argument('--work-dir',default='build/go-java-d-port-20261018');p.add_argument('--samples',type=int,default=7);p.add_argument('--repeats',type=int,default=3);p.add_argument('--audit',action='store_true');measure(p.parse_args())
