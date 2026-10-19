"""Compare the serial D Ruby token finder with frozen Go and tree-sitter Ruby."""
import argparse,bisect,importlib.metadata,json,pathlib,platform,random,re,statistics,subprocess,sys,time
from js_ts_port_compare import digest,run
from brace_port_compare import json_rows,score
SOURCES=['brace_port_unicode.d','brace_port_cleaner.d','brace_port_model.d','ruby_port.d','ruby_port_probe.d']
HELPERS=['ruby_port_compare.py','brace_port_compare.py','js_ts_port_compare.py']

def build(repo,work,mode):
    snapshot=work/'sources';snapshot.mkdir(exist_ok=True)
    for name in SOURCES+HELPERS:(snapshot/name).write_bytes((repo/'benchmarks'/name).read_bytes())
    flags=['-O3','-release','-mcpu=x86-64'] if mode=='release' else ['-O2','-boundscheck=on','-d-version=AuditRubyTokens']
    if mode=='unit':flags+=['-unittest']
    binary=work/('ruby-'+mode+'.exe');command=['F:/D/ldc-1.43.0/bin/ldc2.exe',*flags,'-I='+str(snapshot),'-of='+str(binary),*[snapshot/n for n in SOURCES]]
    bat=work/('compile-'+mode+'.bat');bat.write_text('@echo off\ncall "F:\\D\\ldcvars64.bat" >nul\nif errorlevel 1 exit /b 1\n'+subprocess.list2cmdline(list(map(str,command)))+'\nexit /b %errorlevel%\n')
    (work/('compile-'+mode+'.txt')).write_text(run(['cmd','/d','/c',bat],cwd=repo));return binary,list(map(str,command))

def reference(paths):
    from tree_sitter import Language,Parser
    import tree_sitter_ruby
    parser=Parser(Language(tree_sitter_ruby.language()));files=[]
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
            nodes=[];pending=[tree.root_node]
            while pending:
                n=pending.pop();nodes.append(n);pending.extend(n.named_children)
            beginnings=sorted((n for n in nodes if n.type=='heredoc_beginning'),key=lambda n:n.start_byte)
            bodies=sorted((n for n in nodes if n.type=='heredoc_body'),key=lambda n:n.start_byte)
            assert len(beginnings)==len(bodies),(path,len(beginnings),len(bodies))
            heredocs=[(a.start_byte,last(b)) for a,b in zip(beginnings,bodies)]
            while stack:
                node=stack.pop()
                if node.type in ['string','regex','subshell','heredoc_body','bare_string','simple_symbol','delimited_symbol']:continue
                if node.type in ['method','singleton_method','class','module']:
                    name=node.child_by_field_name('name')
                    if name is not None:
                        end=max([last(node)]+[finish for start,finish in heredocs if node.start_byte<=start<node.end_byte])
                        text=code[name.start_byte:name.end_byte].decode('utf-8')
                        if node.type in ['class','module']:text=re.sub(r'\s+','',text)
                        item=dict(name=text,start=line(node.start_byte),name_line=line(name.start_byte),end=line(end))
                        row['functions' if node.type in ['method','singleton_method'] else 'types'].append(item)
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
        'members.rb':('class C\n def outer\n  def inner(x)\n   x if x\n  end\n  3.times do |x|\n   x\n  end\n end\n def self.[](x) = x\n def []=(x,y)\n end\nend\n',['outer','inner','[]','[]=']),
        'heredoc.rb':('def text\n x=%q{end def fake; end}\n x=<<~TXT\n end def fake; end\n TXT\nend\ndef endless = 1\n',['text','endless']),
        'endless-heredoc.rb':('def f = <<~TXT\nhello def fake; end\nTXT\n',['f']),
        'literals.rb':('def text\n x="#{call("}")} def fake;end"\n x=%Q{end #{value}}\n r=/[{}]end/\n c=?}\nend\n',['text']),
        'comments.rb':('=begin\ndef fake;end\n=end\ndef real;end # def fake2;end\n__END__\ndef data;end\n',['real']),
        'controls.rb':('def f(x)\n while x do\n x-=1\n end\n x+=1 while x<3\n begin\n foo\n rescue E\n bar\n ensure\n done\n end\nend\n',['f']),
        'operators.rb':('class C\n def -@;self;end\n def /(x);x;end\n def field=(x);end\n def self.==(x) = true\nend\n',['-@','/','field=','==']),
        'conditional-values.rb':('class C\n def f\n  a << if ok\n   1\n  else\n   2\n  end\n  @env ||= if ok\n   3\n  end\n end\nend\n',['f']),
        'singleton.rb':('def @object.f(x);x;end\nclass ::A::B\n def g;end\nend\n',['f','g']),
        'variable-modifiers.rb':('def f\n return @value if ok\n @value ||= if ok\n  1\n end\n @value unless failed\nend\n',['f']),
        'division.rb':('def f(x,y)\n x /y\n x/y/2\n /[{}]end/.match(x)\nend\n',['f']),
        'percent.rb':('def f\n a=%q{end {def fake;end}}\n b=%Q{#{value} end}\n c=%w[def fake end]\n d=%i[def fake end]\n e=%r{[{}]end}\n f=%s(end)\n g=%(end #{value})\nend\n',['f']),
        'queued-heredocs.rb':('def f\n x=[<<A, <<~\'B\']\n end def fake;end\nA\n end def fake2;end\n B\nend\n',['f']),
        'singleton-class.rb':('class C\n class << self\n  def f;end\n end\n def g(x=1);x;end\nend\n',['f','g']),
        'endless-block.rb':('def f = values.map do |x|\n x*2\nend\nclass C; def g = 1; end\n',['f','g']),
        'for-loop.rb':('def f\n for x in values do\n  foo(x)\n end\n begin\n  bar\n end while more\nend\n',['f']),
        'unicode.rb':('# \U0001f600\r\ndef café; "ж"; end\r\n',['café']),
        'long.rb':('#'+'x'*131072+'\ndef longName;end\n',['longName']),
        'deep.rb':('def outer\n'+'if true\n'*128+'foo\n'+'end\n'*128+'def inner;end\nend\n',['outer','inner']),
        'nested.rb':(''.join(f'def n{i}\n' for i in range(64))+'end\n'*64,[f'n{i}' for i in range(64)]),
        'incomplete.rb':('def unfinished(\n',[]),
    }
    paths=[]
    for name,(source,_) in cases.items():path=fixtures/name;path.write_bytes(source.encode());paths.append(path.as_posix())
    path=fixtures/'invalid.rb';path.write_bytes(b'#\xff\xfe\ndef valid;end\n');paths.append(path.as_posix())
    manifest=work/'fixtures-manifest.json';manifest.write_text(json.dumps(paths))
    maps={n:json_rows(run([b,config,manifest,'1'])) for n,b in [('d',release),('audit',audit)]};assert maps['d']==maps['audit']
    for name,row in zip([*cases,'invalid.rb'],maps['d']):
        expected=cases[name][1] if name in cases else ['valid'];assert [f[0] for f in row['functions']]==expected,(name,row)
    truth=reference([p for p in paths if pathlib.Path(p).name not in ['incomplete.rb','invalid.rb']]);data=quality(truth,maps['d'])
    assert not any(data[k]['counts'][i] for k in ['functions','types'] for i in ['missing','false_positive','wrong_end','unparseable']),data
    return dict(files=len(paths),input_sha256={p:digest(p) for p in paths},independent=data,grammar_exclusions=['incomplete.rb','invalid.rb'],release_audit_equal=True)

def measure(a):
    repo=pathlib.Path(__file__).resolve().parent.parent;work=pathlib.Path(a.work_dir).resolve()
    if not work.is_relative_to(repo/'build'):raise ValueError('work directory must stay under build/')
    work.mkdir(parents=True,exist_ok=True);sys.path.insert(0,str(repo/'build/ruby-eval-runtime'))
    import tree_sitter_ruby
    old_path=repo/'build/python-d-port-production-20261019/report.json';old=json.loads(old_path.read_text());go=pathlib.Path(old['binaries']['go_generic']['path']);assert digest(go)==old['binaries']['go_generic']['sha256']
    unchanged=dict(old['unchanged_prior_d_sources']);unchanged.update({n:sha for n,sha in old['sources'].items() if n.endswith('.d')})
    for name,sha in unchanged.items():assert digest(repo/'benchmarks'/name)==sha,name
    binaries={'go':go};commands={}
    for mode in ['release','audit','unit']:binaries[mode],commands[mode]=build(repo,work,mode)
    unit=subprocess.run([str(binaries['unit'])],capture_output=True,text=True,encoding='utf-8');(work/'unit.txt').write_text(unit.stdout+unit.stderr);assert unit.returncode==0,(unit.stdout,unit.stderr)
    config=work/'ruby-config.json';config.write_text(run([go,'--config','ruby']),encoding='utf-8')
    grammar_binary=next(pathlib.Path(tree_sitter_ruby.__file__).parent.glob('*.pyd'))
    report=dict(scope='Serial Ruby block/token finder; named methods and class/module types, excluding literal/interpolation contents; no full CLI.',revision=old['revision'],frozen_go_report=dict(path=str(old_path),sha256=digest(old_path)),unchanged_prior_d_sources=unchanged,
        sources={n:digest(repo/'benchmarks'/n) for n in SOURCES+HELPERS},builds=commands,binaries={n:dict(path=str(b),sha256=digest(b),bytes=b.stat().st_size) for n,b in binaries.items()},python=platform.python_version(),platform=platform.platform(),go=old['go'],go_amd64='v1',ldc=run([commands['release'][0],'--version']).split('Registered Targets:')[0],ldc_sha256=digest(commands['release'][0]),tree_sitter=importlib.metadata.version('tree-sitter'),grammar=importlib.metadata.version('tree-sitter-ruby'),grammar_binary=dict(path=str(grammar_binary),sha256=digest(grammar_binary)),reference_newlines='CRLF normalized to LF in memory; original line numbers preserved',config_sha256=digest(config),samples=a.samples,repeats=a.repeats,seed=20261019,projects=[])
    report['checks']=checks(work,config,binaries['release'],binaries['audit']);output=work/'report.json'
    def save():output.write_text(json.dumps(report,indent=2))
    rng=random.Random(20261019)
    for label,root_text in (s.split('=',1) for s in a.project):
        root=pathlib.Path(root_text).resolve();git=['git','-c','safe.directory='+root.as_posix(),'-C',root]
        paths=[(root/p).as_posix() for p in run([*git,'-c','core.quotepath=false','ls-files','*.rb']).splitlines()];assert paths
        manifest=work/(label+'-manifest.json');manifest.write_text(json.dumps(paths));row=dict(label=label,root=str(root),pin=run([*git,'rev-parse','HEAD']).strip(),dirty=bool(run([*git,'status','--porcelain']).strip()),files=len(paths),input_sha256={p:digest(p) for p in paths},manifest_sha256=digest(manifest));report['projects'].append(row)
        cmds={n:[b,config,manifest,'1'] for n,b in [('go',go),('d',binaries['release']),('audit',binaries['audit'])]};maps={n:json_rows(run(cmd)) for n,cmd in cmds.items()};assert maps['d']==maps['audit'] and all(len(r)==len(paths) for r in maps.values())
        for name,rows in maps.items():(work/(label+'-'+name+'.jsonl')).write_text('\n'.join(json.dumps(r,ensure_ascii=False) for r in rows)+'\n',encoding='utf-8')
        truth=reference(paths);(work/(label+'-truth.json')).write_text(json.dumps(truth,indent=2));row['quality']={n:quality(truth,maps[n]) for n in ['go','d']};row['raw_counts']={n:dict(functions=sum(len(r['functions']) for r in rows),types=sum(len(r['classes']) for r in rows)) for n,rows in maps.items() if n!='audit'}
        save();print(label,'quality',json.dumps({n:{k:v['counts'] for k,v in q.items()} for n,q in row['quality'].items()}),flush=True)
        assert row['quality']['d']['functions']['counts']['truth']>0,'no accepted function truth'
        if any(row['quality']['d'][kind]['counts'][issue] for kind in ['functions','types'] for issue in ['missing','false_positive','wrong_end']):raise RuntimeError('Ruby quality gate failed')
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
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--project',action='append',required=True,help='label=Git checkout');p.add_argument('--work-dir',default='build/ruby-d-port-20261019');p.add_argument('--samples',type=int,default=7);p.add_argument('--repeats',type=int,default=3);measure(p.parse_args())
