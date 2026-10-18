"""Freeze Go, compare JS/TS cleanup and full finder API with the D port, then time.

--project LANGUAGE=ROOT scans tracked language files; artifacts stay under build/.
"""
import argparse, hashlib, importlib.metadata, json, os, pathlib, platform, random, statistics, subprocess, sys, tarfile, time

GO_PROBE = r'''
package main
import("encoding/json";"os";"strconv";"strings";"time";ff "github.com/ruslano69/funcfinder/internal")
func main(){
 cfg,e:=ff.LoadConfig();if e!=nil{panic(e)}
 if os.Args[1]=="--config" {
  c,e:=cfg.GetLanguageConfig(os.Args[2]);if e!=nil{panic(e)}
  b,_:=json.Marshal(c);var out map[string]any;json.Unmarshal(b,&out);out["lang"]=os.Args[2]
  out["func_pattern"]=c.FuncRegex().String();if p:=c.ClassRegex();p!=nil{out["class_pattern"]=p.String()}
  for k,p:=range map[string]interface{String() string}{"class_method_open":c.ClassMethodOpenRegex(),"header_open_pattern":c.HeaderOpenRegex(),"open_type_header_pattern":c.OpenTypeHeaderRegex(),"declaration_pattern":c.DeclarationRegex(),"declarator_pattern":c.DeclaratorRegex(),"quoted_method_pattern":c.QuotedMethodRegex()}{
   // A typed nil regexp in an interface needs an explicit nil check.
   switch k{case "class_method_open":if c.ClassMethodOpenRegex()==nil{continue};case "header_open_pattern":if c.HeaderOpenRegex()==nil{continue};case "open_type_header_pattern":if c.OpenTypeHeaderRegex()==nil{continue};case "declaration_pattern":if c.DeclarationRegex()==nil{continue};case "declarator_pattern":if c.DeclaratorRegex()==nil{continue};case "quoted_method_pattern":if c.QuotedMethodRegex()==nil{continue}}
   out[k]=p.String()
  };json.NewEncoder(os.Stdout).Encode(out);return
 }
 b,e:=os.ReadFile(os.Args[1]);if e!=nil{panic(e)};var dc map[string]any;json.Unmarshal(b,&dc)
 c,e:=cfg.GetLanguageConfig(dc["lang"].(string));if e!=nil{panic(e)}
 b,e=os.ReadFile(os.Args[2]);if e!=nil{panic(e)};var paths []string;json.Unmarshal(b,&paths)
 for _,a:=range os.Args[4:]{if a=="--headers"{enc:=json.NewEncoder(os.Stdout);for _,s:=range paths{enc.Encode(c.FuncSubmatch(s))};return}}
 corpus:=make([][]string,len(paths));for i,p:=range paths{corpus[i],e=ff.ReadLines(p);if e!=nil{panic(e)}}
 timed,clean,raw:=false,false,false;for _,a:=range os.Args[4:]{if a=="--time"{timed=true};if a=="--clean"{clean=true};if a=="--raw"{raw=true}}
 repeats,_:=strconv.Atoi(os.Args[3]);results:=make([]*ff.FindResult,len(paths));facts:=make([][]ff.LineFacts,len(paths));checksum:=0
 pass:=func(collect bool){for i,lines:=range corpus{if clean{fs:=ff.NewSanitizer(c,raw).CleanLinesFacts(lines);if collect{facts[i]=fs};for _,f:=range fs{checksum+=len(f.Cleaned)+1}}else{r,err:=ff.NewFinder(c,nil,true,false,raw).FindFunctionsInLines(lines,1,"");if err!=nil{panic(err)};if collect{results[i]=r};checksum+=len(r.Functions)+len(r.Classes)}}}
 if timed{pass(false)};checksum=0;start:=time.Now();for n:=0;n<repeats;n++{pass(true)};ns:=time.Since(start).Nanoseconds();enc:=json.NewEncoder(os.Stdout)
 if timed{enc.Encode(map[string]any{"nanoseconds":ns,"checksum":checksum});return}
 for i,p:=range paths{row:=map[string]any{"path":strings.ReplaceAll(p,"\\","/")};if clean{fs:=make([][]any,0,len(facts[i]));for _,f:=range facts[i]{fs=append(fs,[]any{f.Cleaned,int(f.StartState),f.Indent,f.CodeText})};row["facts"]=fs}else{fs,cs:=make([][]any,0),make([][]any,0);for _,f:=range results[i].Functions{fs=append(fs,[]any{f.Name,f.Start,f.End,f.ClassName,f.SignatureLine,f.SignatureName})};for _,c:=range results[i].Classes{cs=append(cs,[]any{c.Name,c.Start,c.End})};row["functions"],row["classes"]=fs,cs};enc.Encode(row)}
}'''

def run(cmd, **kw):
    r=subprocess.run(list(map(str,cmd)),capture_output=True,text=True,encoding="utf-8",errors="replace",**kw)
    if r.returncode: raise RuntimeError(f"{cmd}: {r.returncode}\n{r.stdout[-2000:]}\n{r.stderr[-4000:]}")
    return r.stdout
def digest(p): return hashlib.sha256(pathlib.Path(p).read_bytes()).hexdigest()

def oracle(rows,lang,repo):
    for path in [repo/"build/js-eval-runtime",repo/"build/eval-runtime"]:sys.path.insert(0,str(path))
    from tree_sitter import Language,Parser
    if lang=="js":
        import tree_sitter_javascript as grammar
        from js_tree_eval import symbols
        parser=Parser(Language(grammar.language()));tsx=parser;grammar_name="tree-sitter-javascript"
    else:
        import tree_sitter_typescript as grammar
        from ts_tree_eval import symbols
        parser=Parser(Language(grammar.language_typescript()));tsx=Parser(Language(grammar.language_tsx()));grammar_name="tree-sitter-typescript"
    counts=dict(files=0,unparseable=0,truth=0,candidate=0,found=0,missing=0,false_positive=0,wrong_end=0,anonymous_excluded=0,computed_excluded=0)
    findings=[]
    for row in rows:
        path=pathlib.Path(row["path"]);code=path.read_bytes();tree=(tsx if path.suffix==".tsx" else parser).parse(code)
        if tree.root_node.has_error:counts["unparseable"]+=1;findings.append({"path":str(path),"kind":"unparseable"});continue
        truth,excluded=symbols(tree,code);counts["files"]+=1;counts["truth"]+=len(truth);counts["candidate"]+=len(row["functions"])
        counts["anonymous_excluded"]+=excluded["anonymous"];counts["computed_excluded"]+=excluded["computed"];used=set()
        for node in truth:
            matches=[(i,f) for i,f in enumerate(row["functions"]) if i not in used and f[0]==node["name"] and node["start"]<=f[1]<=node["name_line"]]
            if not matches:counts["missing"]+=1;findings.append({"path":str(path),"kind":"missing",**node});continue
            i,f=min(matches,key=lambda x:abs(x[1][1]-node["start"]));used.add(i);counts["found"]+=1
            if f[2]!=node["end"]:counts["wrong_end"]+=1;findings.append({"path":str(path),"kind":"wrong_end",**node,"candidate":f})
        for i,f in enumerate(row["functions"]):
            if i not in used:counts["false_positive"]+=1;findings.append({"path":str(path),"kind":"false_positive","candidate":f})
    return {"counts":counts,"findings":findings,"tree_sitter":importlib.metadata.version("tree-sitter"),"grammar":importlib.metadata.version(grammar_name)}

def measure(a):
    repo=pathlib.Path(__file__).resolve().parent.parent; w=pathlib.Path(a.work_dir).resolve(); w.mkdir(parents=True,exist_ok=True)
    if not w.is_relative_to(repo/"build"): raise ValueError("work directory must stay under build/")
    env=dict(os.environ,GOPROXY="off",GOAMD64="v1",GOCACHE=str(pathlib.Path(os.environ["TEMP"])/"funcfinder-go-cache"))
    revision=run(["git","rev-parse",a.revision+"^{commit}"],cwd=repo).strip(); ref=w/"reference-src"; pin=w/"reference-revision.txt"
    if pin.exists() and pin.read_text()!=revision: raise ValueError("choose a new work directory for another revision")
    if not ref.exists():
        archive=w/"reference.tar"; run(["git","archive","--format=tar","-o",archive,revision],cwd=repo); ref.mkdir()
        with tarfile.open(archive) as t: t.extractall(ref,filter="data")
        pin.write_text(revision)
    src=ref/"build/js_ts_probe.go"; src.parent.mkdir(exist_ok=True);src.write_text(GO_PROBE)
    go=w/"go-probe.exe"; d=w/"d-probe.exe"
    run(["go","build","-ldflags=-s -w","-o",go,src],cwd=ref,env=env)
    sources=["js_ts_cleaner.d","js_ts_model.d","javascript_port.d","typescript_header.d","typescript_port.d","typescript_candidates.d","js_ts_probe.d"]
    command=[a.ldc,"-O3","-release","-mcpu=x86-64","-I="+str(repo/"benchmarks"),"-of="+str(d),*[repo/"benchmarks"/s for s in sources]]
    bat=w/"compile.bat";bat.write_text('@echo off\ncall "'+a.env_bat+'" >nul\nif errorlevel 1 exit /b 1\n'+subprocess.list2cmdline(list(map(str,command)))+'\nexit /b %errorlevel%\n')
    (w/"d-build.txt").write_text(run(["cmd","/d","/c",bat],cwd=repo,env=env))
    report={"revision":revision,"samples":a.samples,"repeats":a.repeats,"platform":platform.platform(),"python":platform.python_version(),"logical_cpus":os.cpu_count(),"go":run(["go","version"]).strip(),"go_amd64":"v1","ldc":run([a.ldc,"--version"]).split("Registered Targets:")[0].strip(),"seed":20261018,"scope":"Serial JS/TS finder API and lexical facts; no full CLI/parallel/extraction claim. Go retains generic language machinery.","build_command":list(map(str,command)),"go_probe_sha256":digest(src),"sources":{s:digest(repo/"benchmarks"/s) for s in sources+["js_ts_port_compare.py","js_ts_port_checks.py"]},"binaries":{n:{"path":str(p),"sha256":digest(p),"bytes":p.stat().st_size} for n,p in [("go",go),("d",d)]},"projects":[]}
    baseline=None
    if a.baseline_report:
        baseline_path=pathlib.Path(a.baseline_report).resolve();baseline=json.loads(baseline_path.read_text(encoding="utf-8"))
        if baseline['revision']!=revision or baseline['go_probe_sha256']!=report['go_probe_sha256']:
            raise ValueError("baseline must use the same frozen Go revision and probe")
        binary=baseline['binaries']['d'];old_d=pathlib.Path(binary['path'])
        if digest(old_d)!=binary['sha256']:raise ValueError("baseline D binary changed")
        if digest(pathlib.Path(baseline['binaries']['go']['path']))!=baseline['binaries']['go']['sha256']:
            raise ValueError("baseline Go binary changed")
        report['baseline_report']={"path":str(baseline_path),"sha256":digest(baseline_path)}
        report['binaries']['d_before']=binary
    def save(): (w/"report.json").write_text(json.dumps(report,indent=2),encoding="utf-8")
    rng=random.Random(20261018)
    for project in a.project:
        lang,root=project.split("=",1); root=pathlib.Path(root).resolve();label=root.name+"-"+lang
        suffixes=["*.js","*.jsx","*.mjs","*.cjs"] if lang=="js" else ["*.ts","*.tsx"]
        paths=run(["git","-c","core.quotepath=false","ls-files",*suffixes],cwd=root).splitlines(); paths=[(root/p).as_posix() for p in paths]
        manifest=w/(label+"-manifest.json");manifest.write_text(json.dumps(paths),encoding="utf-8")
        config=w/(lang+"-config.json");config.write_text(run([go,"--config",lang]),encoding="utf-8")
        if baseline:
            if json.loads(config.read_text(encoding="utf-8"))!=json.loads((baseline_path.parent/(lang+"-config.json")).read_text(encoding="utf-8")):
                raise ValueError("baseline configuration changed")
        p={"language":lang,"root":str(root),"pin":run(["git","rev-parse","HEAD"],cwd=root).strip(),"dirty":bool(run(["git","status","--porcelain"],cwd=root).strip()),"files":len(paths),"bytes":sum(pathlib.Path(f).stat().st_size for f in paths),"input_sha256":{f:digest(f) for f in paths}};report["projects"].append(p)
        commands={n:[str(b),str(config),str(manifest),"1"] for n,b in [("go",go),("d",d)]}
        if baseline:commands['d_before']=[str(old_d),str(config),str(manifest),'1']
        for mode in ["clean","find"]:
            extra=["--clean"] if mode=="clean" else [];maps={}
            for name,cmd in commands.items():
                output=run(cmd+extra,cwd=repo,env=env);(w/f"{label}-{mode}-{name}.jsonl").write_text(output,encoding="utf-8");maps[name]=[json.loads(s) for s in output.splitlines()]
            mismatch=[{"index":i,"path":paths[i],"go":x,"d":y} for i,(x,y) in enumerate(zip(maps["go"],maps["d"])) if x!=y]
            p[mode+"_parity"]=not mismatch and len(maps["go"])==len(maps["d"])==len(paths)
            if mismatch: (w/f"{label}-{mode}-mismatches.json").write_text(json.dumps(mismatch[:20],ensure_ascii=False,indent=2),encoding="utf-8")
            save(); print(label,mode,"parity",p[mode+"_parity"],flush=True)
            if not p[mode+"_parity"]: raise RuntimeError(f"{label} {mode}: {len(mismatch)} mismatching files")
            if baseline and maps['d_before']!=maps['go']:raise RuntimeError(f"{label} {mode}: baseline output differs")
            if mode=="find":
                p.update(functions=sum(len(r["functions"]) for r in maps["go"]),classes=sum(len(r["classes"]) for r in maps["go"]))
                p["oracle"]=oracle(maps["d"],lang,repo);save();print(label,"independent",p["oracle"]["counts"],flush=True)
            if not a.samples: continue
            samples={n:{"parser_ms":[],"process_ms":[]} for n in commands}
            for cmd in commands.values():run(cmd+extra,cwd=repo,env=env)
            for _ in range(a.samples):
                order=list(commands);rng.shuffle(order)
                for name in order:
                    cmd=commands[name];timed=json.loads(run(cmd[:-1]+[str(a.repeats),"--time"]+extra,cwd=repo,env=env));samples[name]["parser_ms"].append(timed["nanoseconds"]/a.repeats/1e6)
                    expected=sum(len(f[0].encode("utf-8"))+1 for r in maps[name] for f in r["facts"]) if mode=="clean" else p["functions"]+p["classes"]
                    if timed["checksum"]!=expected*a.repeats:raise RuntimeError("timed checksum changed")
                    start=time.perf_counter();out=run(cmd+extra,cwd=repo,env=env);samples[name]["process_ms"].append((time.perf_counter()-start)*1000)
                    if [json.loads(s) for s in out.splitlines()]!=maps[name]:raise RuntimeError("timed output changed")
            p[mode+"_timings"]={n:{k:{"median":statistics.median(v),"min":min(v),"max":max(v),"samples":v} for k,v in kinds.items()} for n,kinds in samples.items()};save();print(label,mode,"timings",p[mode+"_timings"],flush=True)
        if p['input_sha256']!={f:digest(f) for f in paths}:raise RuntimeError("corpus bytes changed during comparison")
        save()

if __name__=="__main__":
    p=argparse.ArgumentParser(description=__doc__);p.add_argument("--project",action="append",required=True);p.add_argument("--revision",default="HEAD");p.add_argument("--work-dir",default="build/js-ts-d-port-20261018");p.add_argument("--baseline-report",help="Include the verified previous D binary in the same shuffled rounds");p.add_argument("--samples",type=int,default=10);p.add_argument("--repeats",type=int,default=2);p.add_argument("--ldc",default="F:/D/ldc-1.43.0/bin/ldc2.exe");p.add_argument("--env-bat",default="F:/D/ldcvars64.bat");measure(p.parse_args())
