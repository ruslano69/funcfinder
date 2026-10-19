"""Compare the serial D PHP token finder with frozen Go and tree-sitter PHP."""
import argparse,bisect,importlib.metadata,json,pathlib,platform,random,re,statistics,subprocess,sys,time
from js_ts_port_compare import digest,run
from brace_port_compare import json_rows,score
SOURCES=['brace_port_unicode.d','brace_port_cleaner.d','brace_port_model.d','php_port.d','php_port_probe.d']
HELPERS=['php_port_compare.py','brace_port_compare.py','js_ts_port_compare.py']

def build(repo,work,mode):
    snapshot=work/'sources';snapshot.mkdir(exist_ok=True)
    for name in SOURCES+HELPERS:(snapshot/name).write_bytes((repo/'benchmarks'/name).read_bytes())
    flags=['-O3','-release','-mcpu=x86-64'] if mode=='release' else ['-O2','-boundscheck=on','-d-version=AuditPHPTokens']
    if mode=='unit':flags+=['-unittest']
    binary=work/('php-'+mode+'.exe');command=['F:/D/ldc-1.43.0/bin/ldc2.exe',*flags,'-I='+str(snapshot),'-of='+str(binary),*[snapshot/n for n in SOURCES]]
    bat=work/('compile-'+mode+'.bat');bat.write_text('@echo off\ncall "F:\\D\\ldcvars64.bat" >nul\nif errorlevel 1 exit /b 1\n'+subprocess.list2cmdline(list(map(str,command)))+'\nexit /b %errorlevel%\n')
    (work/('compile-'+mode+'.txt')).write_text(run(['cmd','/d','/c',bat],cwd=repo));return binary,list(map(str,command))

def reference(paths):
    from tree_sitter import Language,Parser
    import tree_sitter_php
    parser=Parser(Language(tree_sitter_php.language_php()));files=[]
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
                if node.type in ['string','encapsed_string','heredoc','nowdoc','shell_command_expression']:continue
                if node.type in ['function_definition','method_declaration','class_declaration','interface_declaration','trait_declaration','enum_declaration']:
                    name=node.child_by_field_name('name')
                    if name is not None:
                        item=dict(name=code[name.start_byte:name.end_byte].decode('utf-8'),start=line(node.start_byte),name_line=line(name.start_byte),end=line(last(node)))
                        row['functions' if node.type in ['function_definition','method_declaration'] else 'types'].append(item)
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
        'members.php':('<?php interface I {public function &f(\nint $x\n): A&B;} abstract class C implements I {abstract function f(int $x):A&B; function outer(){class Inner {function local(){}}}} enum E {case X;function e(){}};',['f','f','outer','local','e']),
        'nowdoc.php':("<?php function text(){$x=<<<'END'\nfunction fake(){}\n  END;\n}\n",['text']),
        'heredoc.php':('<?php function text(){$x=<<<"END"\n} function fake(){} ${thing}\nEND;\n}\n',['text']),
        'html.php':('function fake(){} <?php function real(){} ?> function fake2(){} <?= (function(){function inner(){}})();',['real','inner']),
        'attributes.php':('<?php #[A(C::class)] function good(){ $v = new class {function method(){}}; }',['good','method']),
        'anonymous.php':('<?php class C {function outer(){ $x=new class extends C implements I {function inner(){}};}}',['outer','inner']),
        'dnf.php':('<?php interface I {#[A] public function &f(\n A&B $x\n ): (A&B)|C;\n public function g():void;\n}',['f','g']),
        'interpolation.php':('<?php function quoted(){ $s="{$obj->call("}")} function fake(){}"; }',['quoted']),
        'continuing-heredoc.php':('<?php function multi(){ $x=[<<<END\nfunction fake(){}\nEND_more\nEND, 2];\n}\n',['multi']),
        'promotion.php':('<?php readonly class C {function __construct(public int $x=1){} function __invoke(): ?C {return null;}}',['__construct','__invoke']),
        'anonymous-functions.php':('<?php $x=function &() use (&$a) {function nested(){}}; $y=fn()=>1;',['nested']),
        'non-nested-comment.php':('<?php /* /* */ function real(){} // */\n',['real']),
        'case.php':('<?php CLASS C {PUBLIC STATIC FUNCTION F(){}} FUNCTION G(){}',['F','G']),
        'literals.php':('<?php function literals(){ $x="} function fake(){}"; $s=\'function fake2(){}\'; $c=`function fake3(){}`; }',['literals']),
        'unicode.php':('<?php // 😀\r\nfunction café(){ $ж=1; }\r\n',['café']),
        'comments.php':('<?php /* function fake(){} */ function real(){} # function fake2(){}\n// ?> function html(){}\n<?php function after(){}',['real','after']),
        'long.php':('<?php /*'+'x'*131072+'*/function longName(){}\n',['longName']),
        'deep.php':('<?php function outer(){\n'+'if (true) {\n'*128+'foo();\n'+'}\n'*128+'function inner(){}\n}\n',['outer','inner']),
        'nested.php':('<?php '+''.join(f'function n{i}(){{\n' for i in range(64))+'}\n'*64,[f'n{i}' for i in range(64)]),
        'incomplete.php':('<?php function unfinished(\n',[]),
    }
    paths=[]
    for name,(source,_) in cases.items():path=fixtures/name;path.write_bytes(source.encode());paths.append(path.as_posix())
    path=fixtures/'invalid.php';path.write_bytes(b'<?php /*\xff\xfe*/function valid(){}\n');paths.append(path.as_posix())
    manifest=work/'fixtures-manifest.json';manifest.write_text(json.dumps(paths))
    maps={n:json_rows(run([b,config,manifest,'1'])) for n,b in [('d',release),('audit',audit)]};assert maps['d']==maps['audit']
    for name,row in zip([*cases,'invalid.php'],maps['d']):
        expected=cases[name][1] if name in cases else ['valid'];assert [f[0] for f in row['functions']]==expected,(name,row)
    truth=reference([p for p in paths if pathlib.Path(p).name not in ['incomplete.php','invalid.php']]);data=quality(truth,maps['d'])
    assert not any(data[k]['counts'][i] for k in ['functions','types'] for i in ['missing','false_positive','wrong_end','unparseable']),data
    return dict(files=len(paths),input_sha256={p:digest(p) for p in paths},independent=data,grammar_exclusions=['incomplete.php','invalid.php'],release_audit_equal=True)

def measure(a):
    repo=pathlib.Path(__file__).resolve().parent.parent;work=pathlib.Path(a.work_dir).resolve()
    if not work.is_relative_to(repo/'build'):raise ValueError('work directory must stay under build/')
    work.mkdir(parents=True,exist_ok=True);sys.path.insert(0,str(repo/'build/eval-runtime'))
    import tree_sitter_php
    old_path=repo/'build/swift-d-port-20261019/report.json';old=json.loads(old_path.read_text());go=pathlib.Path(old['binaries']['go']['path']);assert digest(go)==old['binaries']['go']['sha256']
    unchanged=dict(old['unchanged_prior_d_sources']);unchanged.update({n:sha for n,sha in old['sources'].items() if n.endswith('.d')})
    for name,sha in unchanged.items():assert digest(repo/'benchmarks'/name)==sha,name
    binaries={'go':go};commands={}
    for mode in ['release','audit','unit']:binaries[mode],commands[mode]=build(repo,work,mode)
    unit=subprocess.run([str(binaries['unit'])],capture_output=True,text=True,encoding='utf-8');(work/'unit.txt').write_text(unit.stdout+unit.stderr);assert unit.returncode==0,(unit.stdout,unit.stderr)
    config=work/'php-config.json';config.write_text(run([go,'--config','php']),encoding='utf-8')
    grammar_binary=next(pathlib.Path(tree_sitter_php.__file__).parent.glob('*.pyd'))
    report=dict(scope='Serial PHP token finder; named functions/methods and class/interface/trait/enum types; no anonymous closure/type truth or full CLI.',revision=old['revision'],frozen_go_report=dict(path=str(old_path),sha256=digest(old_path)),unchanged_prior_d_sources=unchanged,
        sources={n:digest(repo/'benchmarks'/n) for n in SOURCES+HELPERS},builds=commands,binaries={n:dict(path=str(b),sha256=digest(b),bytes=b.stat().st_size) for n,b in binaries.items()},python=platform.python_version(),platform=platform.platform(),go=old['go'],go_amd64='v1',ldc=run([commands['release'][0],'--version']).split('Registered Targets:')[0],ldc_sha256=digest(commands['release'][0]),tree_sitter=importlib.metadata.version('tree-sitter'),grammar=importlib.metadata.version('tree-sitter-php'),grammar_binary=dict(path=str(grammar_binary),sha256=digest(grammar_binary)),reference_newlines='CRLF normalized to LF in memory; original line numbers preserved',config_sha256=digest(config),samples=a.samples,repeats=a.repeats,seed=20261019,projects=[])
    report['checks']=checks(work,config,binaries['release'],binaries['audit']);output=work/'report.json'
    def save():output.write_text(json.dumps(report,indent=2))
    rng=random.Random(20261019)
    for label,root_text in (s.split('=',1) for s in a.project):
        root=pathlib.Path(root_text).resolve();git=['git','-c','safe.directory='+root.as_posix(),'-C',root]
        paths=[(root/p).as_posix() for p in run([*git,'-c','core.quotepath=false','ls-files','*.php']).splitlines()];assert paths
        manifest=work/(label+'-manifest.json');manifest.write_text(json.dumps(paths));row=dict(label=label,root=str(root),pin=run([*git,'rev-parse','HEAD']).strip(),dirty=bool(run([*git,'status','--porcelain']).strip()),files=len(paths),input_sha256={p:digest(p) for p in paths},manifest_sha256=digest(manifest));report['projects'].append(row)
        cmds={n:[b,config,manifest,'1'] for n,b in [('go',go),('d',binaries['release']),('audit',binaries['audit'])]};maps={n:json_rows(run(cmd)) for n,cmd in cmds.items()};assert maps['d']==maps['audit'] and all(len(r)==len(paths) for r in maps.values())
        for name,rows in maps.items():(work/(label+'-'+name+'.jsonl')).write_text('\n'.join(json.dumps(r,ensure_ascii=False) for r in rows)+'\n',encoding='utf-8')
        truth=reference(paths);(work/(label+'-truth.json')).write_text(json.dumps(truth,indent=2));row['quality']={n:quality(truth,maps[n]) for n in ['go','d']};row['raw_counts']={n:dict(functions=sum(len(r['functions']) for r in rows),types=sum(len(r['classes']) for r in rows)) for n,rows in maps.items() if n!='audit'}
        save();print(label,'quality',json.dumps({n:{k:v['counts'] for k,v in q.items()} for n,q in row['quality'].items()}),flush=True)
        assert row['quality']['d']['functions']['counts']['truth']>0,'no accepted function truth'
        if any(row['quality']['d'][kind]['counts'][issue] for kind in ['functions','types'] for issue in ['missing','false_positive','wrong_end']):raise RuntimeError('PHP quality gate failed')
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
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--project',action='append',required=True,help='label=Git checkout');p.add_argument('--work-dir',default='build/php-d-port-20261019');p.add_argument('--samples',type=int,default=7);p.add_argument('--repeats',type=int,default=3);measure(p.parse_args())
