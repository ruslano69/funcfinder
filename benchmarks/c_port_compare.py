"""Build the serial token-based C finder and score Lua against GCC/tree-sitter."""
import argparse, collections, importlib.metadata, json, os, pathlib, platform, random, statistics, subprocess, sys, time
from brace_port_compare import json_rows, score
from js_ts_port_compare import run, digest
from c_tree_eval import preprocess, normalize_builtins, symbols

SOURCES = ['brace_port_unicode.d', 'brace_port_cleaner.d', 'brace_port_model.d',
           'cpp_tokenizer.d', 'c_port.d', 'c_port_probe.d']
HELPERS = ['c_port_compare.py', 'c_tree_eval.py', 'brace_port_compare.py', 'js_ts_port_compare.py']

def build(repo, work, mode):
    sources = work/'sources'; sources.mkdir(exist_ok=True)
    for name in SOURCES+HELPERS:
        (sources/name).write_bytes((repo/'benchmarks'/name).read_bytes())
    flags = ['-O3', '-release', '-mcpu=x86-64'] if mode == 'release' else [
        '-O2', '-boundscheck=on', '-d-version=AuditBraceCleaner', '-d-version=AuditCppTokens']
    if mode == 'unit': flags += ['-unittest']
    binary = work/('c-'+mode+'.exe')
    cmd = ['F:/D/ldc-1.43.0/bin/ldc2.exe', *flags, '-I='+str(sources), '-of='+str(binary),
           *[sources/name for name in SOURCES]]
    bat = work/('compile-'+mode+'.bat')
    bat.write_text('@echo off\ncall "F:\\D\\ldcvars64.bat" >nul\nif errorlevel 1 exit /b 1\n'+
                   subprocess.list2cmdline(list(map(str, cmd)))+'\nexit /b %errorlevel%\n')
    (work/('compile-'+mode+'.txt')).write_text(run(['cmd', '/d', '/c', bat], cwd=repo))
    return binary, list(map(str, cmd))

def reference(paths, root, repo):
    sys.path.insert(0, str(repo/'build/c-eval-runtime'))
    from tree_sitter import Language, Parser
    import tree_sitter_c
    parser = Parser(Language(tree_sitter_c.language()))
    gcc = pathlib.Path('C:/msys64/ucrt64/bin/gcc.exe')
    files = []; normalized = 0
    for name in paths:
        path = pathlib.Path(name)
        try:
            code, origins, active = preprocess(path, gcc, ['LUA_USE_JUMPTABLE=0'], [root], [root/'lua.h'])
            code, count = normalize_builtins(code); normalized += count
        except RuntimeError as exc:
            files.append(dict(path=name, preprocess_error=str(exc))); continue
        tree = parser.parse(code)
        files.append(dict(path=name, active=sorted(active), errors=['grammar rejection'] if tree.root_node.has_error else [],
                          functions=[] if tree.root_node.has_error else symbols(tree, code, origins)))
    return files, dict(tree_sitter=importlib.metadata.version('tree-sitter'),
        grammar=importlib.metadata.version('tree-sitter-c'), gcc=str(gcc), gcc_sha256=digest(gcc),
        gcc_version=run([gcc, '--version']).splitlines()[0], standard='c99',
        defines=['LUA_USE_JUMPTABLE=0'], include_dirs=[str(root)], include_headers=[str(root/'lua.h')],
        normalized_builtin_expressions=normalized)

def checks(work, go, release, audit, config):
    fixtures = work/'fixtures'; fixtures.mkdir(exist_ok=True)
    cases = {
        'declarators.c': ('int f(void), *g(int), (*variable)(int);\nint (*factory(void))(int) {return 0;}\nstruct S *make(void) {return 0;}\nint (*callback)(int);\nint array[4];\nint value=call(1);\n', ['f','g','factory','make']),
        'prefix.c': ('#define FAKE() \\\n int hidden(void) {}\nextern int\nreal(int (*cb)(int)) __attribute__((unused));\n', ['real']),
        'statements.c': ('typedef int Fn(int);\nstruct S {int (*callback)(int);};\nint f(void) {if(1){return call(2);}return 0;} int g(void);\n', ['f','g']),
        'unicode.c': ('/* ж */ int café(void) {char *s="ж"; return 0;}\n', ['café']),
        'long.c': ('/*'+'x'*131072+'*/ int long_name(void) {return 0;}\n', ['long_name']),
        'nested.c': ('int nested(void) {'+'{'*128+';'+'}'*128+'}\nint after(void);\n', ['nested','after']),
        'incomplete.c': ('int incomplete(\n', []),
        'literals.c': ('int literal(void){char c=\'\\\'\';char *s="/* } */";return c;}\n', ['literal']),
        'knr.c': ('int old(a,b)\nint a; char *b;\n{extern int local(void); return a;}\nint next(void);\n', ['old','local','next']),
        'macro.c': ('DECLARE(int variable;)\nint next(void);\n#if FLAG\nUNKNOWN\n#endif\nint after(void);\n', ['next','after']),
    }
    paths = []
    for name, (text, expected) in cases.items():
        path=fixtures/name;path.write_bytes(text.encode());paths.append(path.as_posix())
    bad=fixtures/'invalid.c';bad.write_bytes(b'/*\xff\xfe*/ int valid(void){return 0;}\n');paths.append(bad.as_posix())
    manifest=work/'fixtures-manifest.json';manifest.write_text(json.dumps(paths))
    commands={n:[b,config,manifest,'1'] for n,b in [('go',go),('d',release),('audit',audit)]}
    maps={n:json_rows(run(cmd)) for n,cmd in commands.items()}
    assert maps['d']==maps['audit']
    for name,row in zip([*cases,'invalid.c'],maps['d']):
        expected=cases[name][1] if name in cases else ['valid']
        assert [f[0] for f in row['functions']]==expected, (name,row)
    for extra in [['--clean'],['--clean','--raw']]:
        data={n:json_rows(run(cmd+extra)) for n,cmd in commands.items()}
        assert data['go']==data['d']==data['audit'], ('fixture cleanup',extra)
    from tree_sitter import Language, Parser
    import tree_sitter_c
    parser=Parser(Language(tree_sitter_c.language()));truth=[]
    # Macro wrappers and deliberately incomplete/invalid UTF-8 files have their
    # own explicit expectations above, and are not accepted grammar coverage.
    for row in maps['d']:
        path=pathlib.Path(row['path'])
        if path.name in ['macro.c','incomplete.c','invalid.c']:continue
        code=path.read_bytes();tree=parser.parse(code)
        assert not tree.root_node.has_error, ('fixture grammar rejection',path)
        truth.append(dict(path=row['path'],functions=symbols(tree,code,list(range(1,code.count(b'\n')+2)))))
    quality=score(truth,maps['d'],lambda f,line:False,True)
    assert not any(quality['counts'][k] for k in ['missing','false_positive','wrong_end']),quality
    return dict(files=len(paths), release_audit_maps_equal=True, normal_raw_cleaner_go_parity=True,
                input_sha256={p:digest(p) for p in paths}, cases={k:v[1] for k,v in cases.items()}, independent=quality,
                grammar_exclusions=['macro.c','incomplete.c','invalid.c'])

def measure(a):
    repo=pathlib.Path(__file__).resolve().parent.parent;work=pathlib.Path(a.work_dir).resolve()
    if not work.is_relative_to(repo/'build'):raise ValueError('work directory must stay under build/')
    work.mkdir(parents=True,exist_ok=True)
    previous=repo/'build/go-java-d-port-20261018/report.json'; old=json.loads(previous.read_text())
    go=pathlib.Path(old['binaries']['go']['path'])
    assert digest(go)==old['binaries']['go']['sha256']
    unchanged={n:sha for n,sha in old['sources'].items() if n.endswith('.d')}
    for name,sha in unchanged.items():
        assert digest(repo/'benchmarks'/name)==digest(previous.parent/'sources'/name)==sha, ('earlier backend changed',name)
    release,command=build(repo,work,'release');audit,audit_command=build(repo,work,'audit');unit,unit_command=build(repo,work,'unit')
    unit_result=subprocess.run([str(unit)],capture_output=True,text=True,encoding='utf-8')
    (work/'unit.txt').write_text(unit_result.stdout+unit_result.stderr)
    assert unit_result.returncode==0, (unit_result.stdout,unit_result.stderr)
    root=pathlib.Path(a.root).resolve();git=['git','-c','safe.directory='+root.as_posix(),'-C',root]
    paths=[(root/p).as_posix() for p in run([*git,'ls-files','*.c','*.h']).splitlines()]
    manifest=work/'lua-manifest.json';manifest.write_text(json.dumps(paths))
    config=work/'c-config.json';config.write_text(run([go,'--config','c']),encoding='utf-8')
    inputs={p:digest(p) for p in paths}
    report=dict(scope='Serial C finder API; specialized D versus generic Go; no complete CLI or parallelism.',
        revision=old['revision'], frozen_go_report=dict(path=str(previous),sha256=digest(previous)),
        unchanged_prior_d_sources=unchanged,
        sources={n:digest(repo/'benchmarks'/n) for n in SOURCES+HELPERS},
        builds=dict(release=command,audit=audit_command,unit=unit_command),
        binaries={n:dict(path=str(b),sha256=digest(b),bytes=b.stat().st_size) for n,b in [('go',go),('d',release),('audit',audit),('unit',unit)]},
        root=str(root),pin=run([*git,'rev-parse','HEAD']).strip(),dirty=bool(run([*git,'status','--porcelain']).strip()),
        files=len(paths),input_sha256=inputs,config_sha256=digest(config),manifest_sha256=digest(manifest),
        python=platform.python_version(),platform=platform.platform(),ldc=run([command[0],'--version']).split('Registered Targets:')[0],
        go=old['go'],go_amd64='v1',samples=a.samples,repeats=a.repeats,seed=20261019)
    output=work/'report.json'
    def save():output.write_text(json.dumps(report,indent=2))
    commands={n:[b,config,manifest,'1'] for n,b in [('go',go),('d',release),('audit',audit)]}
    for mode,extra in [('find',[]),('clean',['--clean']),('raw-clean',['--clean','--raw'])]:
        data={n:json_rows(run(cmd+extra)) for n,cmd in commands.items()}
        for n,rows in data.items():(work/(mode+'-'+n+'.jsonl')).write_text('\n'.join(json.dumps(r,ensure_ascii=False) for r in rows)+'\n',encoding='utf-8')
        assert all(len(rows)==len(paths) for rows in data.values())
        assert data['d']==data['audit'], (mode,'audit mismatch')
        if mode!='find':assert data['go']==data['d'], (mode,'Go mismatch')
        else:maps=data
        print(mode,'audit passed',flush=True)
    truth,metadata=reference(paths,root,repo);(work/'truth.json').write_text(json.dumps(truth,indent=2))
    active={f['path']:set(f.get('active',[])) for f in truth}
    report['reference']=metadata;report['oracle']={n:score(truth,rows,lambda f,line:line not in active[f['path']],True) for n,rows in maps.items() if n!='audit'}
    report['raw_counts']={n:dict(functions=sum(len(r['functions']) for r in rows),types=sum(len(r['classes']) for r in rows)) for n,rows in maps.items() if n!='audit'}
    report['checks']=checks(work,go,release,audit,config);save()
    print('quality',json.dumps({n:r['counts'] for n,r in report['oracle'].items()}),flush=True)
    if any(report['oracle']['d']['counts'][k] for k in ['missing','false_positive','wrong_end','unparseable','preprocess_failed']):
        raise RuntimeError('C quality gate failed; inspect retained report')
    samples={n:dict(parser_ms=[],process_ms=[]) for n in ['go','d']};rng=random.Random(20261019)
    for n in samples:run(commands[n])
    for _ in range(a.samples):
        order=list(samples);rng.shuffle(order)
        for n in order:
            cmd=commands[n];timed=json.loads(run(cmd[:-1]+[str(a.repeats),'--time']))
            expected=sum(len(r['functions'])+len(r['classes']) for r in maps[n])*a.repeats
            assert timed['checksum']==expected
            samples[n]['parser_ms'].append(timed['nanoseconds']/a.repeats/1e6)
            start=time.perf_counter();out=run(cmd);samples[n]['process_ms'].append((time.perf_counter()-start)*1e3)
            assert json_rows(out)==maps[n]
    report['timings']={n:{k:dict(median=statistics.median(v),min=min(v),max=max(v),samples=v) for k,v in kinds.items()} for n,kinds in samples.items()} if a.samples else {}
    assert inputs=={p:digest(p) for p in paths}
    for name,sha in report['sources'].items():assert digest(repo/'benchmarks'/name)==digest(work/'sources'/name)==sha
    for binary in report['binaries'].values():assert digest(binary['path'])==binary['sha256']
    for name,sha in unchanged.items():assert digest(repo/'benchmarks'/name)==sha
    for path,sha in report['checks']['input_sha256'].items():assert digest(path)==sha
    report['postflight']='passed';save();print('timings',json.dumps(report['timings']),flush=True)

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--root',default='build/language-eval-20261012/repos/lua')
    p.add_argument('--work-dir',default='build/c-d-port-20261019')
    p.add_argument('--samples',type=int,default=7);p.add_argument('--repeats',type=int,default=3)
    measure(p.parse_args())
