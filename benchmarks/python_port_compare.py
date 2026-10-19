"""Compare the serial D Python indentation finder with frozen Go and native AST."""
import argparse,bisect,importlib.metadata,json,os,pathlib,platform,random,re,statistics,subprocess,sys,tarfile,time
from js_ts_port_compare import digest,run
from brace_port_compare import json_rows,score,GO_PROBE
SOURCES=['brace_port_unicode.d','brace_port_cleaner.d','brace_port_model.d','python_port.d','python_port_probe.d']
HELPERS=['python_port_compare.py','brace_port_compare.py','js_ts_port_compare.py']

def build_go(repo,work,revision):
    ref=work/'reference-src'
    if not ref.exists():
        archive=work/'reference.tar';run(['git','archive','--format=tar','-o',archive,revision]);ref.mkdir()
        with tarfile.open(archive) as t:t.extractall(ref,filter='data')
    original=ref/'internal/python_finder.go';source=original.read_text(encoding='utf-8')
    begin=source.index('\tlogical := pythonLogicalLines(lines, pf.config)');end=source.index('\n}\n',begin)+2
    adapter=source+'\n\nfunc (pf *PythonFinder) FindFunctionsInLinesForPort(lines []string) (*FindResult,error) {\n filename := ""\n'+source[begin:end]+'\n'
    overlay_source=work/'python_finder.overlay.go.txt';overlay_source.write_text(adapter,encoding='utf-8')
    overlay=work/'go-overlay.json';overlay.write_text(json.dumps(dict(Replace={str(original):str(overlay_source)})))
    main=ref/'build/python_probe.go';main.parent.mkdir(exist_ok=True)
    probe=GO_PROBE.replace('ff.NewFinder(c,nil,true,false,raw).FindFunctionsInLines(lines,1,"")','ff.NewPythonFinder(c,"","map",false).FindFunctionsInLinesForPort(lines)')
    reader='corpus[i],e=ff.ReadLines(p);if e!=nil{panic(e)}'
    assert reader in probe
    probe=probe.replace(reader,'data,e:=os.ReadFile(p);if e!=nil{panic(e)};corpus[i]=strings.Split(string(data),"\\n")')
    assert probe!=GO_PROBE;main.write_text(probe,encoding='utf-8')
    binary=work/'go-python-probe.exe';command=['go','build','-overlay='+str(overlay),'-ldflags=-s -w','-o',binary,main]
    env=dict(os.environ,GOPROXY='off',GOAMD64='v1',GOCACHE=str(pathlib.Path(os.environ['TEMP'])/'funcfinder-go-cache'))
    (work/'go-build.txt').write_text(run(command,cwd=ref,env=env))
    return binary,dict(command=list(map(str,command)),original=dict(path=str(original),sha256=digest(original)),overlay=dict(path=str(overlay_source),sha256=digest(overlay_source)),probe=dict(path=str(main),sha256=digest(main)),description='Unchanged production PythonFinder body after file read, exposed through an appended preloaded-lines adapter; original method retained.')

def build(repo,work,mode):
    snapshot=work/'sources';snapshot.mkdir(exist_ok=True)
    for name in SOURCES+HELPERS:(snapshot/name).write_bytes((repo/'benchmarks'/name).read_bytes())
    flags=['-O3','-release','-mcpu=x86-64'] if mode=='release' else ['-O2','-boundscheck=on','-d-version=AuditPythonTokens']
    if mode=='unit':flags+=['-unittest']
    binary=work/('python-'+mode+'.exe');command=['F:/D/ldc-1.43.0/bin/ldc2.exe',*flags,'-I='+str(snapshot),'-of='+str(binary),*[snapshot/n for n in SOURCES]]
    bat=work/('compile-'+mode+'.bat');bat.write_text('@echo off\ncall "F:\\D\\ldcvars64.bat" >nul\nif errorlevel 1 exit /b 1\n'+subprocess.list2cmdline(list(map(str,command)))+'\nexit /b %errorlevel%\n')
    (work/('compile-'+mode+'.txt')).write_text(run(['cmd','/d','/c',bat],cwd=repo));return binary,list(map(str,command))

def reference(paths):
    import ast
    files=[]
    for path in paths:
        row=dict(path=path,functions=[],types=[],errors=[])
        try:tree=ast.parse(pathlib.Path(path).read_bytes(),filename=path)
        except (SyntaxError,UnicodeError,ValueError) as ex:row['errors']=[dict(kind=type(ex).__name__,detail=str(ex))]
        else:
            for node in ast.walk(tree):
                if isinstance(node,(ast.FunctionDef,ast.AsyncFunctionDef,ast.ClassDef)):
                    start=min([node.lineno]+[d.lineno for d in node.decorator_list])
                    row['types' if isinstance(node,ast.ClassDef) else 'functions'].append(dict(name=node.name,start=start,name_line=node.lineno,end=node.end_lineno))
            for kind in ['functions','types']:row[kind].sort(key=lambda n:(n['name_line'],n['start']))
        files.append(row)
    return files

def quality(files,rows):
    tf=[dict(path=f['path'],errors=f['errors'],functions=f['types']) for f in files]
    tr=[dict(path=r['path'],functions=[[n,s,e,'',s,n] for n,s,e in r['classes']]) for r in rows]
    return dict(functions=score(files,rows,lambda f,line:False,True),types=score(tf,tr,lambda f,line:False,True))

def checks(work,config,release,audit):
    fixtures=work/'fixtures';fixtures.mkdir(exist_ok=True)
    cases={
        'members.py':('@decorator(\n 1\n)\nasync def f[T](\n x:T\n)->T:\n # comment\n return x\n\nclass C:\n def inline(self): return 1; return 2\n def outer(self):\n  def inner():\n   pass\n  return inner\n# trailing\n',['f','inline','outer','inner']),
        'strings.py':('def text():\n s=f"{call("}")} def fake(): pass"\n s=r\'\'\'\ndef fake():pass\n\'\'\'\n return s\ndef after():pass\n',['text','after']),
        'continued.py':('def f():\n x = (\n  1,\n  2\n )\n return x\n\ndef g(): return 1 + \\\n 2\n',['f','g']),
        'types.py':('class A[T](\n Base[T]\n):\n class Inner: pass\n def f(self): pass\n',['f']),
        'tabs.py':('def f():\r\n\tif True:\r\n\t\tpass\r\n\treturn 1\r\n\r\ndef g(): pass\r',['f','g']),
        'fstrings.py':('def f():\n return f"{{literal}} { {"key": "value"} } {value:{width}}"\ndef g():return t"hello {name}"\n',['f','g']),
        'fstring-comments.py':('def f():\n return f"{(\n  1 # } quote\n )}"\ndef g():return rf"\\{call("}")}"\n',['f','g']),
        'bom.py':('\ufeffdef first():pass\n',['first']),
        'comments.py':('def f():\n # blank\n """doc\n def fake(): pass\n """\n # tail\n\n# outside\ndef g():pass\n',['f','g']),
        'control.py':('def f():\n try:\n  pass\n except E:\n  pass\n else:\n  pass\n finally:\n  pass\ndef g():pass\n',['f','g']),
        'unicode.py':('# \U0001f600\r\ndef café(): return "ж"\r\n',['café']),
        'long.py':('#'+'x'*131072+'\ndef longName():pass\n',['longName']),
        'deep.py':('def outer():\n'+''.join(' '*(i+1)+f'def n{i}():\n' for i in range(64))+' '*65+'pass\n',['outer']+[f'n{i}' for i in range(64)]),
        'incomplete.py':('def unfinished(\n',[]),
    }
    paths=[]
    for name,(source,_) in cases.items():path=fixtures/name;path.write_bytes(source.encode());paths.append(path.as_posix())
    path=fixtures/'invalid.py';path.write_bytes(b'#\xff\xfe\ndef valid():pass\n');paths.append(path.as_posix())
    manifest=work/'fixtures-manifest.json';manifest.write_text(json.dumps(paths))
    maps={n:json_rows(run([b,config,manifest,'1'])) for n,b in [('d',release),('audit',audit)]};assert maps['d']==maps['audit']
    for name,row in zip([*cases,'invalid.py'],maps['d']):
        expected=cases[name][1] if name in cases else ['valid'];assert [f[0] for f in row['functions']]==expected,(name,row)
    truth=reference([p for p in paths if pathlib.Path(p).name not in ['incomplete.py','invalid.py']]);data=quality(truth,maps['d'])
    assert not any(data[k]['counts'][i] for k in ['functions','types'] for i in ['missing','false_positive','wrong_end','unparseable']),data
    return dict(files=len(paths),input_sha256={p:digest(p) for p in paths},independent=data,grammar_exclusions=['incomplete.py','invalid.py'],release_audit_equal=True)

def measure(a):
    repo=pathlib.Path(__file__).resolve().parent.parent;work=pathlib.Path(a.work_dir).resolve()
    if not work.is_relative_to(repo/'build'):raise ValueError('work directory must stay under build/')
    work.mkdir(parents=True,exist_ok=True);sys.path.insert(0,str(repo/'build/eval-runtime'))
    import ast
    old_path=repo/'build/scala-d-port-final-20261019/report.json';old=json.loads(old_path.read_text());generic=pathlib.Path(old['binaries']['go']['path']);assert digest(generic)==old['binaries']['go']['sha256']
    go,go_adapter=build_go(repo,work,old['revision'])
    unchanged=dict(old['unchanged_prior_d_sources']);unchanged.update({n:sha for n,sha in old['sources'].items() if n.endswith('.d')})
    for name,sha in unchanged.items():assert digest(repo/'benchmarks'/name)==sha,name
    binaries={'go':go,'go_generic':generic};commands={}
    for mode in ['release','audit','unit']:binaries[mode],commands[mode]=build(repo,work,mode)
    unit=subprocess.run([str(binaries['unit'])],capture_output=True,text=True,encoding='utf-8');(work/'unit.txt').write_text(unit.stdout+unit.stderr);assert unit.returncode==0,(unit.stdout,unit.stderr)
    config=work/'python-config.json';config.write_text(run([go,'--config','py']),encoding='utf-8')
    grammar_binary=pathlib.Path(ast.__file__)
    report=dict(scope='Serial Python logical-statement/indentation finder; named def/async def and classes; native AST reference; no full CLI.',revision=old['revision'],frozen_go_report=dict(path=str(old_path),sha256=digest(old_path)),unchanged_prior_d_sources=unchanged,
        sources={n:digest(repo/'benchmarks'/n) for n in SOURCES+HELPERS},builds=commands,binaries={n:dict(path=str(b),sha256=digest(b),bytes=b.stat().st_size) for n,b in binaries.items()},python=platform.python_version(),platform=platform.platform(),go=old['go'],go_amd64='v1',ldc=run([commands['release'][0],'--version']).split('Registered Targets:')[0],ldc_sha256=digest(commands['release'][0]),tree_sitter=None,grammar='CPython ast '+platform.python_version(),python_executable=dict(path=sys.executable,sha256=digest(sys.executable)),grammar_binary=dict(path=str(grammar_binary),sha256=digest(grammar_binary)),reference_newlines='CRLF normalized to LF in memory; original line numbers preserved',config_sha256=digest(config),samples=a.samples,repeats=a.repeats,seed=20261019,projects=[])
    report['go_adapter']=go_adapter;report['checks']=checks(work,config,binaries['release'],binaries['audit']);output=work/'report.json'
    def save():output.write_text(json.dumps(report,indent=2))
    rng=random.Random(20261019)
    for label,root_text in (s.split('=',1) for s in a.project):
        root=pathlib.Path(root_text).resolve();git=['git','-c','safe.directory='+root.as_posix(),'-C',root]
        paths=[(root/p).as_posix() for p in run([*git,'-c','core.quotepath=false','ls-files','*.py']).splitlines()];assert paths
        manifest=work/(label+'-manifest.json');manifest.write_text(json.dumps(paths));row=dict(label=label,root=str(root),pin=run([*git,'rev-parse','HEAD']).strip(),dirty=bool(run([*git,'status','--porcelain']).strip()),files=len(paths),input_sha256={p:digest(p) for p in paths},manifest_sha256=digest(manifest));report['projects'].append(row)
        cmds={n:[b,config,manifest,'1'] for n,b in [('go',go),('d',binaries['release']),('audit',binaries['audit'])]};maps={n:json_rows(run(cmd)) for n,cmd in cmds.items()};assert maps['d']==maps['audit'] and all(len(r)==len(paths) for r in maps.values())
        for name,rows in maps.items():(work/(label+'-'+name+'.jsonl')).write_text('\n'.join(json.dumps(r,ensure_ascii=False) for r in rows)+'\n',encoding='utf-8')
        truth=reference(paths);(work/(label+'-truth.json')).write_text(json.dumps(truth,indent=2));row['quality']={n:quality(truth,maps[n]) for n in ['go','d']};row['raw_counts']={n:dict(functions=sum(len(r['functions']) for r in rows),types=sum(len(r['classes']) for r in rows)) for n,rows in maps.items() if n!='audit'}
        save();print(label,'quality',json.dumps({n:{k:v['counts'] for k,v in q.items()} for n,q in row['quality'].items()}),flush=True)
        assert row['quality']['d']['functions']['counts']['truth']>0,'no accepted function truth'
        if any(row['quality']['d'][kind]['counts'][issue] for kind in ['functions','types'] for issue in ['missing','false_positive','wrong_end']):raise RuntimeError('Python quality gate failed')
        samples={n:dict(parser_ms=[],process_ms=[]) for n in ['go','d']}
        for n in samples:run(cmds[n])
        for _ in range(a.samples):
            order=list(samples);rng.shuffle(order)
            for n in order:
                cmd=cmds[n];timed=json.loads(run(cmd[:-1]+[str(a.repeats),'--time']));expected=sum(len(r['functions'])+len(r['classes']) for r in maps[n])*a.repeats;assert timed['checksum']==expected
                samples[n]['parser_ms'].append(timed['nanoseconds']/a.repeats/1e6);start=time.perf_counter();out=run(cmd);samples[n]['process_ms'].append((time.perf_counter()-start)*1e3);assert json_rows(out)==maps[n]
        row['timings']={n:{k:dict(median=statistics.median(v),min=min(v),max=max(v),samples=v) for k,v in kinds.items()} for n,kinds in samples.items()} if a.samples else {};save();print(label,'timings',json.dumps(row['timings']),flush=True)
    for name,sha in report['sources'].items():assert digest(repo/'benchmarks'/name)==digest(work/'sources'/name)==sha,name
    for name,sha in unchanged.items():assert digest(repo/'benchmarks'/name)==sha,name
    for binary in report['binaries'].values():assert digest(binary['path'])==binary['sha256']
    for row in report['projects']:
        for path,sha in row['input_sha256'].items():assert digest(path)==sha,path
        assert digest(work/(row['label']+'-manifest.json'))==row['manifest_sha256']
    for path,sha in report['checks']['input_sha256'].items():assert digest(path)==sha,path
    assert digest(config)==report['config_sha256'] and digest(grammar_binary)==report['grammar_binary']['sha256'];report['postflight']='passed';save()

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--project',action='append',required=True,help='label=Git checkout');p.add_argument('--work-dir',default='build/python-d-port-20261019');p.add_argument('--samples',type=int,default=7);p.add_argument('--repeats',type=int,default=3);measure(p.parse_args())
