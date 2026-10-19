"""Build the serial D Kotlin finder; score pinned source with Kotlin compiler PSI."""
import argparse,json,pathlib,platform,random,statistics,subprocess,sys,time
from js_ts_port_compare import digest,run
from brace_port_compare import json_rows,score

SOURCES=['brace_port_unicode.d','brace_port_cleaner.d','brace_port_model.d','kotlin_port.d','kotlin_port_probe.d']
HELPERS=['kotlin_port_compare.py','kotlin_psi_oracle.java.txt','brace_port_compare.py','js_ts_port_compare.py']
ORACLE=None

def build_reference(repo,work):
    global ORACLE
    runtime=repo/'build/kotlin-oracle-runtime';jdk=next(runtime.glob('jdk-*'));java=jdk/'bin/java.exe';javac=jdk/'bin/javac.exe'
    classes=work/'oracle-classes';classes.mkdir(exist_ok=True);source=work/'KotlinPsiOracle.java'
    source.write_bytes((repo/'benchmarks/kotlin_psi_oracle.java.txt').read_bytes())
    libraries=str(runtime/'kotlinc/lib/*')
    command=[javac,'-proc:none','-cp',libraries,'-d',classes,source]
    (work/'oracle-build.txt').write_text(run(command))
    ORACLE=(work,[java,'-cp',str(classes)+';'+libraries,'KotlinPsiOracle'])
    version=subprocess.run([java,'-cp',libraries,'org.jetbrains.kotlin.cli.jvm.K2JVMCompiler','-version'],capture_output=True,text=True,encoding='utf-8')
    assert version.returncode==0,(version.stdout,version.stderr)
    return dict(kotlin_version=(version.stdout+version.stderr).strip(),java=run([java,'--version']).strip(),build_command=list(map(str,command)),source_sha256=digest(source),class_sha256=digest(classes/'KotlinPsiOracle.class'),runtime_sha256={str(p):digest(p) for p in sorted((runtime/'kotlinc/lib').glob('*.jar'))},java_sha256=digest(java),javac_sha256=digest(javac))

def build(repo,work,mode):
    snapshot=work/'sources';snapshot.mkdir(exist_ok=True)
    for name in SOURCES+HELPERS:(snapshot/name).write_bytes((repo/'benchmarks'/name).read_bytes())
    flags=['-O3','-release','-mcpu=x86-64'] if mode=='release' else ['-O2','-boundscheck=on','-d-version=AuditKotlinTokens']
    if mode=='unit':flags+=['-unittest']
    binary=work/('kotlin-'+mode+'.exe')
    command=['F:/D/ldc-1.43.0/bin/ldc2.exe',*flags,'-I='+str(snapshot),'-of='+str(binary),*[snapshot/n for n in SOURCES]]
    bat=work/('compile-'+mode+'.bat');bat.write_text('@echo off\ncall "F:\\D\\ldcvars64.bat" >nul\nif errorlevel 1 exit /b 1\n'+subprocess.list2cmdline(list(map(str,command)))+'\nexit /b %errorlevel%\n')
    (work/('compile-'+mode+'.txt')).write_text(run(['cmd','/d','/c',bat],cwd=repo))
    return binary,list(map(str,command))

def reference(paths):
    work,command=ORACLE
    manifest=work/('oracle-inputs-'+str(len(paths))+'.txt');manifest.write_text('\n'.join(paths)+'\n',encoding='utf-8')
    text=run(command+[manifest]);(work/(manifest.stem+'-truth.jsonl')).write_text(text,encoding='utf-8')
    return json_rows(text)

def quality(files,rows):
    functions=score(files,rows,lambda f,line:False,True)
    tf=[dict(path=f['path'],errors=f['errors'],functions=f['types']) for f in files]
    tr=[dict(path=r['path'],functions=[[n,s,e,'',s,n] for n,s,e in r['classes']]) for r in rows]
    return dict(functions=functions,types=score(tf,tr,lambda f,line:False,True))

def checks(work,config,release,audit):
    fixtures=work/'fixtures';fixtures.mkdir(exist_ok=True)
    cases={
        'local.kt':('fun outer() {\n fun counters() = value.toInt()\n if (ready) { use() }\n}\nfun String.ext() = length\n',['outer','counters','ext']),
        'bodies.kt':('fun f() = if (ok)\n  foo()\nelse\n  bar()\nfun g() = run { x() }\n .let { y() }\nfun h() = """multi\n} fun fake() {}\n"""\n',['f','g','h']),
        'types.kt':('interface I {\n fun f(x: Int): Int\n fun g(): String\n}\nclass C<T>(val f: () -> Unit)\nobject O {}\ntypealias Alias = List<String>\n',['f','g']),
        'strings.kt':('/* outer /* nested */ */\nfun `a test`() = "${run { "}" }}"\nfun chars() = \'}\'\n',['`a test`','chars']),
        'generic.kt':('inline fun <T : Comparable<T>> List<T>.best(\n p: (T) -> Boolean\n): T? = firstOrNull(p)\n',['best']),
        'control.kt':('fun f() = try { foo() }\ncatch (e: Exception) { bar() }\nfinally { cleanup() }\nfun g() = when (x) {\n 1 -> foo()\n else -> bar()\n}\n',['f','g']),
        'infix.kt':('fun f() = call() togetherWith\n other()\nfun g() = a to\n b\ndata class C(val x:Int)\nprivate const val X=1\n',['f','g']),
        'unicode.kt':('// 😀\r\nfun café() = "ж"\r\n',['café']),
        'long.kt':('/*'+'x'*131072+'*/ fun longName() = 1\n',['longName']),
        'deep.kt':('fun outer(){\n'+'if (true) {\n'*128+'foo()\n'+'}\n'*128+'fun inner(){}\n}\n',['outer','inner']),
        'incomplete.kt':('fun unfinished(\n',[]),
    }
    paths=[]
    for name,(source,_) in cases.items():path=fixtures/name;path.write_bytes(source.encode());paths.append(path.as_posix())
    path=fixtures/'invalid.kt';path.write_bytes(b'/*\xff\xfe*/fun valid(){}\n');paths.append(path.as_posix())
    manifest=work/'fixtures-manifest.json';manifest.write_text(json.dumps(paths))
    maps={n:json_rows(run([b,config,manifest,'1'])) for n,b in [('d',release),('audit',audit)]};assert maps['d']==maps['audit']
    for name,row in zip([*cases,'invalid.kt'],maps['d']):
        expected=cases[name][1] if name in cases else ['valid'];assert [f[0] for f in row['functions']]==expected,(name,row)
    truth=reference([p for p in paths if pathlib.Path(p).name not in ['incomplete.kt','invalid.kt']]);data=quality(truth,maps['d'])
    assert not any(data[k]['counts'][i] for k in ['functions','types'] for i in ['missing','false_positive','wrong_end','unparseable']),data
    return dict(files=len(paths),input_sha256={p:digest(p) for p in paths},independent=data,grammar_exclusions=['incomplete.kt','invalid.kt'],release_audit_equal=True)

def measure(a):
    repo=pathlib.Path(__file__).resolve().parent.parent;work=pathlib.Path(a.work_dir).resolve()
    if not work.is_relative_to(repo/'build'):raise ValueError('work directory must stay under build/')
    work.mkdir(parents=True,exist_ok=True)
    psi=build_reference(repo,work)
    old_path=repo/'build/rust-d-port-20261019/report.json';old=json.loads(old_path.read_text());go=pathlib.Path(old['binaries']['go']['path']);assert digest(go)==old['binaries']['go']['sha256']
    unchanged=dict(old['unchanged_prior_d_sources']);unchanged.update({n:sha for n,sha in old['sources'].items() if n.endswith('.d')})
    for name,sha in unchanged.items():assert digest(repo/'benchmarks'/name)==sha,name
    binaries={'go':go};commands={}
    for mode in ['release','audit','unit']:binaries[mode],commands[mode]=build(repo,work,mode)
    unit=subprocess.run([str(binaries['unit'])],capture_output=True,text=True,encoding='utf-8');(work/'unit.txt').write_text(unit.stdout+unit.stderr);assert unit.returncode==0,(unit.stdout,unit.stderr)
    config=work/'kotlin-config.json';config.write_text(run([go,'--config','kotlin']),encoding='utf-8')
    report=dict(scope='Serial Kotlin token finder; named fun and named types, no constructors/accessors/string-interpolation declarations/full CLI.',revision=old['revision'],frozen_go_report=dict(path=str(old_path),sha256=digest(old_path)),unchanged_prior_d_sources=unchanged,
        sources={n:digest(repo/'benchmarks'/n) for n in SOURCES+HELPERS},builds=commands,binaries={n:dict(path=str(b),sha256=digest(b),bytes=b.stat().st_size) for n,b in binaries.items()},python=platform.python_version(),platform=platform.platform(),go=old['go'],go_amd64='v1',ldc=run([commands['release'][0],'--version']).split('Registered Targets:')[0],ldc_sha256=digest(commands['release'][0]),reference=psi,config_sha256=digest(config),samples=a.samples,repeats=a.repeats,seed=20261019,projects=[])
    report['checks']=checks(work,config,binaries['release'],binaries['audit']);output=work/'report.json'
    def save():output.write_text(json.dumps(report,indent=2))
    rng=random.Random(20261019)
    for label,root_text in (s.split('=',1) for s in a.project):
        root=pathlib.Path(root_text).resolve();git=['git','-c','safe.directory='+root.as_posix(),'-C',root]
        paths=[(root/p).as_posix() for p in run([*git,'-c','core.quotepath=false','ls-files','*.kt','*.kts']).splitlines()];assert paths
        manifest=work/(label+'-manifest.json');manifest.write_text(json.dumps(paths))
        row=dict(label=label,root=str(root),pin=run([*git,'rev-parse','HEAD']).strip(),dirty=bool(run([*git,'status','--porcelain']).strip()),files=len(paths),input_sha256={p:digest(p) for p in paths},manifest_sha256=digest(manifest));report['projects'].append(row)
        cmds={n:[b,config,manifest,'1'] for n,b in [('go',go),('d',binaries['release']),('audit',binaries['audit'])]};maps={n:json_rows(run(cmd)) for n,cmd in cmds.items()}
        assert maps['d']==maps['audit'] and all(len(r)==len(paths) for r in maps.values())
        for name,rows in maps.items():(work/(label+'-'+name+'.jsonl')).write_text('\n'.join(json.dumps(r,ensure_ascii=False) for r in rows)+'\n',encoding='utf-8')
        truth=reference(paths);(work/(label+'-truth.json')).write_text(json.dumps(truth,indent=2))
        row['quality']={n:quality(truth,maps[n]) for n in ['go','d']};row['raw_counts']={n:dict(functions=sum(len(r['functions']) for r in rows),types=sum(len(r['classes']) for r in rows)) for n,rows in maps.items() if n!='audit'}
        save();print(label,'quality',json.dumps({n:{k:v['counts'] for k,v in q.items()} for n,q in row['quality'].items()}),flush=True)
        assert row['quality']['d']['functions']['counts']['files']>0 and row['quality']['d']['functions']['counts']['truth']>0,'no accepted function truth'
        if any(row['quality']['d'][kind]['counts'][issue] for kind in ['functions','types'] for issue in ['missing','false_positive','wrong_end']):raise RuntimeError('Kotlin quality gate failed')
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
    for path,sha in report['reference']['runtime_sha256'].items():assert digest(path)==sha,path
    assert digest(work/'oracle-classes/KotlinPsiOracle.class')==psi['class_sha256']
    assert digest(work/'KotlinPsiOracle.java')==psi['source_sha256']
    assert digest(config)==report['config_sha256'];report['postflight']='passed';save()

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--project',action='append',required=True,help='label=Git checkout')
    p.add_argument('--work-dir',default='build/kotlin-d-port-20261019');p.add_argument('--samples',type=int,default=7);p.add_argument('--repeats',type=int,default=3);measure(p.parse_args())
