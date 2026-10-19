"""Freeze Go and compare the serial D C#/C++ finder/cleaner on tracked files."""
import argparse,collections,hashlib,json,os,pathlib,platform,random,statistics,subprocess,sys,tarfile,time
from js_ts_port_compare import GO_PROBE as BASE_PROBE,run,digest

def json_rows(text):return [json.loads(line) for line in text.split('\n') if line]

SOURCES=['brace_port_unicode.d','brace_port_cleaner.d','brace_port_model.d','csharp_port.d','cpp_tokenizer.d','cpp_tokens_port.d','java_port.d','go_port.d','brace_port_probe.d']
GO_PROBE=BASE_PROBE.replace('"encoding/json";','"encoding/json";"regexp";').replace('};json.NewEncoder(os.Stdout).Encode(out);return', '''};for k,p:=range map[string]*regexp.Regexp{"constructor_pattern":c.ConstructorRegex(),"member_declaration_pattern":c.MemberDeclarationRegex(),"conversion_operator_pattern":c.ConversionOperatorRegex()}{if p!=nil{out[k]=p.String()}};json.NewEncoder(os.Stdout).Encode(out);return''')
GO_PROBE=GO_PROBE.replace('enc.Encode(c.FuncSubmatch(s))','pattern:=c.FuncRegex();if p,ok:=dc["probe_pattern"].(string);ok{pattern=regexp.MustCompile(p)};enc.Encode(pattern.FindStringSubmatch(s))')
GO_PROBE=GO_PROBE.replace('"regexp";','"regexp";"unicode";').replace('out["lang"]=os.Args[2]','out["lang"]=os.Args[2];out["unicode_version"]=unicode.Version')

def score(files,rows,inactive,token_headers=False):
    counts=collections.Counter({k:0 for k in ['files','unparseable','preprocess_failed','truth','candidate','found','missing','false_positive','wrong_end','inactive_candidate']});findings=[]
    bypath={pathlib.Path(r['path']).resolve():r for r in rows}
    for f in files:
        path=pathlib.Path(f['path']).resolve()
        if f.get('errors'):
            counts['unparseable']+=1;findings.append(dict(path=str(path),kind='unparseable',errors=f['errors']));continue
        if f.get('preprocess_error'):
            counts['preprocess_failed']+=1;findings.append(dict(path=str(path),kind='preprocess_failed',error=f['preprocess_error']));continue
        candidate=[]
        for fn in bypath[path]['functions']:
            if inactive(f,fn[1]):counts['inactive_candidate']+=1
            else:candidate.append(fn)
        truth=f['functions'];counts.update(files=1,truth=len(truth),candidate=len(candidate));used=set()
        for node in truth:
            hits=[(i,c) for i,c in enumerate(candidate) if i not in used and c[0]==node['name'] and node['start']<=((c[4] or c[1]) if token_headers else c[1])<=node['name_line']]
            if not hits:counts['missing']+=1;findings.append(dict(path=str(path),kind='missing',node=node));continue
            i,c=min(hits,key=lambda item:abs(item[1][1]-node['start']));used.add(i);counts['found']+=1
            if c[2]!=node['end']:counts['wrong_end']+=1;findings.append(dict(path=str(path),kind='wrong_end',node=node,candidate=c))
        for i,c in enumerate(candidate):
            if i not in used:counts['false_positive']+=1;findings.append(dict(path=str(path),kind='false_positive',candidate=c))
    return dict(counts=dict(counts),findings=findings)

def oracle(rows,lang,repo,work,root):
    if lang=='cs':
        from csharp_roslyn_eval import build_oracle
        binary,sdk=build_oracle(work/'oracle');manifest=work/(root.name+'-roslyn-inputs.json');manifest.write_text(json.dumps([r['path'] for r in rows]))
        data=json.loads(run(['dotnet',binary,manifest]));out=score(data['files'],rows,lambda f,line:any(s<=line<=e for s,e in f['inactive']))
        out['reference']={k:data[k] for k in ['roslyn_version','language_version','features']};out['reference'].update(sdk=sdk,defines=[],binary_sha256=digest(binary));return out
    sys.path.insert(0,str(repo/'build/cpp-eval-runtime'))
    from clang import cindex
    import importlib.metadata
    from cpp_tree_eval import active_lines,symbols
    gxx=pathlib.Path('C:/msys64/ucrt64/bin/g++.exe');gccroot=gxx.parent.parent
    gccver=run([gxx,'-dumpversion']).strip();base=gccroot/'lib/gcc/x86_64-w64-mingw32'/gccver
    cppinc=gccroot/'include/c++'/gccver
    incs=[cppinc,cppinc/'x86_64-w64-mingw32',cppinc/'backward',base/'include',base/'include-fixed',gccroot/'include']
    incs=[p for p in incs if p.exists()];include=[str(root/'include')] if (root/'include').exists() else [];args=['-x','c++','-std=c++17','-target','x86_64-w64-mingw32',*['-I'+p for p in include],*[x for p in incs for x in ['-isystem',str(p)]]]
    index=cindex.Index.create();files=[]
    for r in rows:
        path=pathlib.Path(r['path'])
        try:active=active_lines(path,gxx,'c++17',[],include)
        except RuntimeError as ex:files.append(dict(path=str(path),preprocess_error=str(ex)));continue
        unit=index.parse(str(path),args=args);errors=[d.spelling for d in unit.diagnostics if d.severity>=cindex.Diagnostic.Error]
        files.append(dict(path=str(path),errors=errors,functions=[] if errors else symbols(unit,path),active=sorted(active)))
    (work/(root.name+'-clang-truth.json')).write_text(json.dumps(files,indent=2))
    active_sets={f['path']:set(f.get('active',[])) for f in files};out=score(files,rows,lambda f,line:line not in active_sets[f['path']],True)
    out['match_profile']='C++ token-name/header anchor and end; declaration starts may include outer template clauses.'
    out['reference']=dict(libclang=importlib.metadata.version('libclang'),clang_library_sha256=digest(pathlib.Path(cindex.Config.library_path)/'libclang.dll'),gxx=str(gxx),gxx_sha256=digest(gxx),gxx_version=run([gxx,'--version']).splitlines()[0],args=args,defines=[],standard='c++17',target='x86_64-w64-mingw32');return out

def measure(a):
    repo=pathlib.Path(__file__).resolve().parent.parent;work=pathlib.Path(a.work_dir).resolve();work.mkdir(parents=True,exist_ok=True)
    if not work.is_relative_to(repo/'build'):raise ValueError('work directory must stay under build/')
    baseline=None;baseline_path=None
    if a.baseline_report:
        baseline_path=pathlib.Path(a.baseline_report).resolve()
        if baseline_path.parent==work:raise ValueError('candidate directory must differ from baseline directory')
        baseline=json.loads(baseline_path.read_text())
        for binary in baseline['binaries'].values():assert digest(binary['path'])==binary['sha256'],'baseline binary changed'
    revision=run(['git','rev-parse',a.revision+'^{commit}']).strip();ref=work/'reference-src';pin=work/'reference-revision.txt'
    if pin.exists() and pin.read_text()!=revision:raise ValueError('use a new directory for another revision')
    if not ref.exists():
        archive=work/'reference.tar';run(['git','archive','--format=tar','-o',archive,revision]);ref.mkdir()
        with tarfile.open(archive) as t:t.extractall(ref,filter='data')
        pin.write_text(revision)
    source=ref/'build/brace_port_probe.go';source.parent.mkdir(exist_ok=True);source.write_text(GO_PROBE)
    env=dict(os.environ,GOPROXY='off',GOAMD64='v1',GOCACHE=str(pathlib.Path(os.environ['TEMP'])/'funcfinder-go-cache'))
    go=work/'go-probe.exe';d=work/'d-probe.exe';run(['go','build','-ldflags=-s -w','-o',go,source],cwd=ref,env=env)
    snapshot=work/'sources';snapshot.mkdir(exist_ok=True)
    helpers=['brace_port_compare.py','brace_port_checks.py','js_ts_port_compare.py','csharp_roslyn_eval.py','csharp_oracle.cs','cpp_tree_eval.py']
    for name in SOURCES+helpers:(snapshot/name).write_bytes((repo/'benchmarks'/name).read_bytes())
    flags=['-O2','-boundscheck=on','-d-version=AuditBraceCleaner','-d-version=AuditCppTokens'] if a.audit else ['-O3','-release','-mcpu=x86-64']
    d=work/('d-audit-probe.exe' if a.audit else 'd-probe.exe')
    command=['F:/D/ldc-1.43.0/bin/ldc2.exe',*flags,'-I='+str(snapshot),'-of='+str(d),*[snapshot/s for s in SOURCES]]
    bat=work/('compile-audit.bat' if a.audit else 'compile.bat');bat.write_text('@echo off\ncall "F:\\D\\ldcvars64.bat" >nul\nif errorlevel 1 exit /b 1\n'+subprocess.list2cmdline(list(map(str,command)))+'\nexit /b %errorlevel%\n')
    (work/('d-audit-build.txt' if a.audit else 'd-build.txt')).write_text(run(['cmd','/d','/c',bat],cwd=repo))
    report=dict(revision=revision,go=run(['go','version']).strip(),ldc=run([command[0],'--version']).split('Registered Targets:')[0].strip(),platform=platform.platform(),python=platform.python_version(),logical_cpus=os.cpu_count(),go_amd64='v1',build_command=list(map(str,command)),go_probe_sha256=digest(source),sources={n:digest(snapshot/n) for n in SOURCES+helpers},binaries={n:dict(path=str(p),bytes=p.stat().st_size,sha256=digest(p)) for n,p in [('go',go),('d',d)]},samples=a.samples,repeats=a.repeats,seed=20261018,audit=a.audit,scope='Serial C#/C++ finder and cleanup API; generic Go versus specialized D. No complete CLI/parallel/extraction claim.',projects=[])
    if baseline:
        assert baseline['revision']==revision and baseline['go_probe_sha256']==report['go_probe_sha256'],'baseline reference differs'
        report['baseline_report']=dict(path=str(baseline_path),sha256=digest(baseline_path));report['binaries']['d_regex']=baseline['binaries']['d']
    output=work/('audit-report.json' if a.audit else 'report.json')
    def save():output.write_text(json.dumps(report,indent=2))
    rng=random.Random(20261018)
    for spec in a.project:
        lang,path=spec.split('=',1);root=pathlib.Path(path).resolve();label=root.name+'-'+lang;git=['git','-c','safe.directory='+root.as_posix(),'-C',root]
        suffix=['*.cs','*.csx'] if lang=='cs' else ['*.cc','*.cpp','*.cxx','*.hpp','*.h']
        paths=[(root/p).as_posix() for p in run([*git,'-c','core.quotepath=false','ls-files',*suffix]).splitlines()]
        if lang=='cpp' and root.name=='fmt':paths=[p for p in paths if pathlib.Path(p).relative_to(root).parts[0] in ['include','src']]
        if lang=='cpp' and root.name=='tinyxml2':paths=[p for p in paths if pathlib.Path(p).name in ['tinyxml2.cpp','tinyxml2.h']]
        manifest=work/(label+'-manifest.json');manifest.write_text(json.dumps(paths));config=work/(lang+'-config.json');config.write_text(run([go,'--config',lang]),encoding='utf-8')
        p=dict(language=lang,root=str(root),pin=run([*git,'rev-parse','HEAD']).strip(),dirty=bool(run([*git,'status','--porcelain']).strip()),files=len(paths),bytes=sum(pathlib.Path(s).stat().st_size for s in paths),input_sha256={s:digest(s) for s in paths});report['projects'].append(p)
        commands={n:[b,config,manifest,'1'] for n,b in [('go',go),('d',d)]}
        if baseline:
            old=next(p for p in baseline['projects'] if p['root']==str(root) and p['language']==lang)
            assert p['input_sha256']==old['input_sha256'],'baseline corpus differs'
            assert json.loads(config.read_text())==json.loads((baseline_path.parent/(lang+'-config.json')).read_text()),'baseline config differs'
            commands['d_regex']=[pathlib.Path(baseline['binaries']['d']['path']),config,manifest,'1']
        for mode,extra in [('clean',['--clean']),('raw-clean',['--clean','--raw']),('find',[])]:
            maps={n:json_rows(run(cmd+extra)) for n,cmd in commands.items()}
            for n,rows in maps.items():(work/(label+'-'+mode+'-'+n+'.jsonl')).write_text('\n'.join(json.dumps(r,ensure_ascii=False) for r in rows)+'\n',encoding='utf-8')
            mismatch=[dict(index=i,path=paths[i],go=x,d=y) for i,(x,y) in enumerate(zip(maps['go'],maps['d'])) if x!=y];p[mode+'_parity']=not mismatch and len(maps['go'])==len(maps['d'])==len(paths)
            if mismatch:(work/(label+'-'+mode+'-mismatches.json')).write_text(json.dumps(mismatch[:20],indent=2,ensure_ascii=False),encoding='utf-8')
            save();print(label,mode,'parity',p[mode+'_parity'],flush=True)
            if not p[mode+'_parity'] and (lang!='cpp' or mode!='find'):raise RuntimeError('map mismatch: '+label+' '+mode)
            if baseline:assert maps['d_regex']==maps['go'],'baseline no longer matches frozen Go'
            if mode=='find':
                p.update(functions=sum(len(r['functions']) for r in maps['d']),classes=sum(len(r['classes']) for r in maps['d']))
                if not a.skip_oracle:p['oracle']=oracle(maps['d'],lang,repo,work,root);print(label,'oracle',p['oracle']['counts'],flush=True);save()
                if lang=='cpp' and not a.skip_oracle:
                    files=json.loads((work/(root.name+'-clang-truth.json')).read_text());active={f['path']:set(f.get('active',[])) for f in files}
                    p['go_oracle']=score(files,maps['go'],lambda f,line:line not in active[f['path']],True)
                    def issues(data):return {(f['path'],f['node']['name'],f['node']['start'],f['node']['end']) for f in data['findings'] if f['kind'] in ['missing','wrong_end']}
                    p['quality_no_regression']=issues(p['oracle'])<=issues(p['go_oracle']) and all(p['oracle']['counts'][key]<=p['go_oracle']['counts'][key] for key in ['missing','false_positive','wrong_end'])
                    save();assert p['quality_no_regression'],'C++ quality regression'
            if not a.samples or mode!='find':continue
            samples={n:dict(parser_ms=[],process_ms=[]) for n in commands}
            for cmd in commands.values():run(cmd)
            for _ in range(a.samples):
                order=list(commands);rng.shuffle(order)
                for n in order:
                    cmd=commands[n];timed=json.loads(run(cmd[:-1]+[str(a.repeats),'--time']));expected=sum(len(r['functions'])+len(r['classes']) for r in maps[n]);assert timed['checksum']==expected*a.repeats
                    samples[n]['parser_ms'].append(timed['nanoseconds']/a.repeats/1e6);start=time.perf_counter();out=run(cmd);samples[n]['process_ms'].append((time.perf_counter()-start)*1e3);assert json_rows(out)==maps[n]
            p['timings']={n:{k:dict(median=statistics.median(v),min=min(v),max=max(v),samples=v) for k,v in kinds.items()} for n,kinds in samples.items()};print(label,'timings',p['timings'],flush=True)
        assert p['input_sha256']=={s:digest(s) for s in paths};save()
    for name,sha in report['sources'].items():assert digest(repo/'benchmarks'/name)==sha

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--project',action='append',required=True);p.add_argument('--revision',default='bdbc81b');p.add_argument('--work-dir',default='build/cpp-tokens-20261018');p.add_argument('--baseline-report');p.add_argument('--samples',type=int,default=7);p.add_argument('--repeats',type=int,default=1);p.add_argument('--audit',action='store_true');p.add_argument('--skip-oracle',action='store_true');measure(p.parse_args())
