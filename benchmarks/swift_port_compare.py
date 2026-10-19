"""Compare the serial D Swift token finder with frozen Go and tree-sitter Swift."""
import argparse,bisect,importlib.metadata,json,pathlib,platform,random,re,statistics,subprocess,sys,time
from js_ts_port_compare import digest,run
from brace_port_compare import json_rows,score
SOURCES=['brace_port_unicode.d','brace_port_cleaner.d','brace_port_model.d','swift_port.d','swift_port_probe.d']
HELPERS=['swift_port_compare.py','brace_port_compare.py','js_ts_port_compare.py']

def build(repo,work,mode):
    snapshot=work/'sources';snapshot.mkdir(exist_ok=True)
    for name in SOURCES+HELPERS:(snapshot/name).write_bytes((repo/'benchmarks'/name).read_bytes())
    flags=['-O3','-release','-mcpu=x86-64'] if mode=='release' else ['-O2','-boundscheck=on','-d-version=AuditSwiftTokens']
    if mode=='unit':flags+=['-unittest']
    binary=work/('swift-'+mode+'.exe');command=['F:/D/ldc-1.43.0/bin/ldc2.exe',*flags,'-I='+str(snapshot),'-of='+str(binary),*[snapshot/n for n in SOURCES]]
    bat=work/('compile-'+mode+'.bat');bat.write_text('@echo off\ncall "F:\\D\\ldcvars64.bat" >nul\nif errorlevel 1 exit /b 1\n'+subprocess.list2cmdline(list(map(str,command)))+'\nexit /b %errorlevel%\n')
    (work/('compile-'+mode+'.txt')).write_text(run(['cmd','/d','/c',bat],cwd=repo));return binary,list(map(str,command))

def reference(paths):
    from tree_sitter import Language,Parser
    import tree_sitter_swift
    parser=Parser(Language(tree_sitter_swift.language()));files=[]
    for path in paths:
        code=pathlib.Path(path).read_bytes().replace(b'\r\n',b'\n');tree=parser.parse(code);starts=[0]+[m.end() for m in re.finditer(b'\n',code)]
        def line(offset):return bisect.bisect_right(starts,offset)
        def last(node):
            while node.children:
                children=[c for c in node.children if not c.is_extra]
                if not children:break
                node=children[-1]
            return node.end_byte-1
        row=dict(path=path,functions=[],types=[],errors=[]);stack=[tree.root_node]
        if tree.root_node.has_error:
            while stack:
                node=stack.pop()
                if node.type=='ERROR' or node.is_missing:row['errors'].append(dict(kind=node.type,start=line(node.start_byte),end=line(max(node.start_byte,node.end_byte-1)),text=code[node.start_byte:node.end_byte].decode('utf-8',errors='replace')[:200]))
                stack.extend(reversed(node.children))
            assert row['errors']
        else:
            while stack:
                node=stack.pop()
                if node.type in ['line_string_literal','multi_line_string_literal','raw_string_literal','regex_literal','raw_regex_literal']:continue
                if node.type in ['function_declaration','protocol_function_declaration','class_declaration','protocol_declaration','typealias_declaration','associatedtype_declaration']:
                    keywords=['func'] if node.type in ['function_declaration','protocol_function_declaration'] else ['class','struct','enum','actor','protocol','typealias','associatedtype']
                    children=[c for c in node.children if not c.is_extra];name=None
                    for i,child in enumerate(children[:-1]):
                        if child.type in keywords:name=children[i+1];break
                    if name is not None:
                        item=dict(name=code[name.start_byte:name.end_byte].decode('utf-8'),start=line(node.start_byte),name_line=line(name.start_byte),end=line(last(node)))
                        row['functions' if keywords==['func'] else 'types'].append(item)
                stack.extend(reversed(node.named_children))
        files.append(row)
    return files

def quality(files,rows):
    tf=[dict(path=f['path'],errors=f['errors'],functions=f['types']) for f in files]
    tr=[dict(path=r['path'],functions=[[n,s,e,'',s,n] for n,s,e in r['classes']]) for r in rows]
    return dict(functions=score(files,rows,lambda f,line:False,True),types=score(tf,tr,lambda f,line:False,True))

def checks(work,config,release,audit):
    fixtures=work/'fixtures';fixtures.mkdir(exist_ok=True)
    cases={
        'members.swift':('protocol P {\nfunc f<T>(_ x:T) async throws -> Int where T:Equatable\nfunc g()\n}\nstruct S {\nstatic func ==(a:S,b:S)->Bool {true}\nfunc outer(){func local(){}}\n}\nextension S {func ext(){}}\ntypealias Alias = (Int) -> String\n',['f','g','==','outer','local','ext']),
        'raw.swift':('/* outer /* nested */ */\nfunc `repeat`() {let a = ##"} func fake(){} \\##(foo("}"))"##;let b = #/ } func fake2(){} /#}\nfunc 🚀() {}\n',['`repeat`','🚀']),
        'kinds.swift':('class C {class func f(){};init(){};deinit{};var p:Int {0}}\n#if X\nfunc left(){}\n#else\nfunc right(){}\n#endif\n',['f','left','right']),
        'generic.swift':('func generic<T: P>(_ x:T) -> Result<T, E>\n where T.Value == Int { x() }\nprotocol P {associatedtype Value: Equatable}\n',['generic']),
        'multiline.swift':('func text() -> String {\n """\n } func fake() {}\n """\n}\n',['text']),
        'regex.swift':('func pattern() {let r = /[{}]func fake/;let x = a / b;use(r,x)}\n',['pattern']),
        'annotations.swift':('protocol P {\n @A func first(\n _:Int\n )\n @B func second()\n}\n',['first','second']),
        'crlf.swift':('func multiline() -> String {\r\n """\r\n a \\\r\n b\r\n """\r\n}\r\n',['multiline']),
        'unicode.swift':('// 😀\r\nfunc café() {let s="ж"}\r\n',['café']),
        'long.swift':('/*'+'x'*131072+'*/func longName(){}\n',['longName']),
        'deep.swift':('func outer(){\n'+'if true {\n'*128+'foo()\n'+'}\n'*128+'func inner(){}\n}\n',['outer','inner']),
        'incomplete.swift':('func unfinished<T(\n',[]),
    }
    paths=[]
    for name,(source,_) in cases.items():path=fixtures/name;path.write_bytes(source.encode());paths.append(path.as_posix())
    path=fixtures/'invalid.swift';path.write_bytes(b'/*\xff\xfe*/func valid(){}\n');paths.append(path.as_posix())
    manifest=work/'fixtures-manifest.json';manifest.write_text(json.dumps(paths))
    maps={n:json_rows(run([b,config,manifest,'1'])) for n,b in [('d',release),('audit',audit)]};assert maps['d']==maps['audit']
    for name,row in zip([*cases,'invalid.swift'],maps['d']):
        expected=cases[name][1] if name in cases else ['valid'];assert [f[0] for f in row['functions']]==expected,(name,row)
    truth=reference([p for p in paths if pathlib.Path(p).name not in ['incomplete.swift','invalid.swift']]);data=quality(truth,maps['d'])
    assert not any(data[k]['counts'][i] for k in ['functions','types'] for i in ['missing','false_positive','wrong_end','unparseable']),data
    return dict(files=len(paths),input_sha256={p:digest(p) for p in paths},independent=data,grammar_exclusions=['incomplete.swift','invalid.swift'],release_audit_equal=True)

def measure(a):
    repo=pathlib.Path(__file__).resolve().parent.parent;work=pathlib.Path(a.work_dir).resolve()
    if not work.is_relative_to(repo/'build'):raise ValueError('work directory must stay under build/')
    work.mkdir(parents=True,exist_ok=True);sys.path.insert(0,str(repo/'build/swift-eval-runtime'))
    import tree_sitter_swift
    old_path=repo/'build/kotlin-d-port-20261019/report.json';old=json.loads(old_path.read_text());go=pathlib.Path(old['binaries']['go']['path']);assert digest(go)==old['binaries']['go']['sha256']
    unchanged=dict(old['unchanged_prior_d_sources']);unchanged.update({n:sha for n,sha in old['sources'].items() if n.endswith('.d')})
    for name,sha in unchanged.items():assert digest(repo/'benchmarks'/name)==sha,name
    binaries={'go':go};commands={}
    for mode in ['release','audit','unit']:binaries[mode],commands[mode]=build(repo,work,mode)
    unit=subprocess.run([str(binaries['unit'])],capture_output=True,text=True,encoding='utf-8');(work/'unit.txt').write_text(unit.stdout+unit.stderr);assert unit.returncode==0,(unit.stdout,unit.stderr)
    config=work/'swift-config.json';config.write_text(run([go,'--config','swift']),encoding='utf-8')
    grammar_binary=next(pathlib.Path(tree_sitter_swift.__file__).parent.glob('*.pyd'))
    report=dict(scope='Serial Swift token finder; named func and nominal/alias types; no constructors/accessors/extensions as new types/full CLI.',revision=old['revision'],frozen_go_report=dict(path=str(old_path),sha256=digest(old_path)),unchanged_prior_d_sources=unchanged,
        sources={n:digest(repo/'benchmarks'/n) for n in SOURCES+HELPERS},builds=commands,binaries={n:dict(path=str(b),sha256=digest(b),bytes=b.stat().st_size) for n,b in binaries.items()},python=platform.python_version(),platform=platform.platform(),go=old['go'],go_amd64='v1',ldc=run([commands['release'][0],'--version']).split('Registered Targets:')[0],ldc_sha256=digest(commands['release'][0]),tree_sitter=importlib.metadata.version('tree-sitter'),grammar=importlib.metadata.version('tree-sitter-swift'),grammar_binary=dict(path=str(grammar_binary),sha256=digest(grammar_binary)),reference_newlines='CRLF normalized to LF in memory; original line numbers preserved',config_sha256=digest(config),samples=a.samples,repeats=a.repeats,seed=20261019,projects=[])
    report['checks']=checks(work,config,binaries['release'],binaries['audit']);output=work/'report.json'
    def save():output.write_text(json.dumps(report,indent=2))
    rng=random.Random(20261019)
    for label,root_text in (s.split('=',1) for s in a.project):
        root=pathlib.Path(root_text).resolve();git=['git','-c','safe.directory='+root.as_posix(),'-C',root]
        paths=[(root/p).as_posix() for p in run([*git,'-c','core.quotepath=false','ls-files','*.swift']).splitlines()];assert paths
        manifest=work/(label+'-manifest.json');manifest.write_text(json.dumps(paths));row=dict(label=label,root=str(root),pin=run([*git,'rev-parse','HEAD']).strip(),dirty=bool(run([*git,'status','--porcelain']).strip()),files=len(paths),input_sha256={p:digest(p) for p in paths},manifest_sha256=digest(manifest));report['projects'].append(row)
        cmds={n:[b,config,manifest,'1'] for n,b in [('go',go),('d',binaries['release']),('audit',binaries['audit'])]};maps={n:json_rows(run(cmd)) for n,cmd in cmds.items()};assert maps['d']==maps['audit'] and all(len(r)==len(paths) for r in maps.values())
        for name,rows in maps.items():(work/(label+'-'+name+'.jsonl')).write_text('\n'.join(json.dumps(r,ensure_ascii=False) for r in rows)+'\n',encoding='utf-8')
        truth=reference(paths);(work/(label+'-truth.json')).write_text(json.dumps(truth,indent=2));row['quality']={n:quality(truth,maps[n]) for n in ['go','d']};row['raw_counts']={n:dict(functions=sum(len(r['functions']) for r in rows),types=sum(len(r['classes']) for r in rows)) for n,rows in maps.items() if n!='audit'}
        save();print(label,'quality',json.dumps({n:{k:v['counts'] for k,v in q.items()} for n,q in row['quality'].items()}),flush=True)
        assert row['quality']['d']['functions']['counts']['truth']>0,'no accepted function truth'
        if any(row['quality']['d'][kind]['counts'][issue] for kind in ['functions','types'] for issue in ['missing','false_positive','wrong_end']):raise RuntimeError('Swift quality gate failed')
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
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--project',action='append',required=True,help='label=Git checkout');p.add_argument('--work-dir',default='build/swift-d-port-20261019');p.add_argument('--samples',type=int,default=7);p.add_argument('--repeats',type=int,default=3);measure(p.parse_args())
