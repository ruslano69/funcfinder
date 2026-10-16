"""Compare C# named methods/constructors/operators/local functions with SDK Roslyn."""
import argparse
import collections
import hashlib
import json
import os
import pathlib
import platform
import re
import subprocess
import xml.sax.saxutils


def build_oracle(work):
    sdks = subprocess.check_output(['dotnet', '--list-sdks'], text=True).splitlines()
    version, base = re.fullmatch(r'(\S+) \[(.*)\]', sdks[-1]).groups()
    sdk = pathlib.Path(base) / version
    target = 'net' + version.split('.')[0] + '.0'
    source = pathlib.Path(__file__).with_name('csharp_oracle.cs').resolve()
    escape = xml.sax.saxutils.escape
    project = work / 'Oracle.csproj'
    work.mkdir(parents=True, exist_ok=True)
    project.write_text(f'''<Project Sdk="Microsoft.NET.Sdk">
<PropertyGroup><OutputType>Exe</OutputType><TargetFramework>{target}</TargetFramework>
<EnableDefaultCompileItems>false</EnableDefaultCompileItems><Nullable>enable</Nullable>
<RestoreSources></RestoreSources></PropertyGroup><ItemGroup>
<Compile Include="{escape(str(source))}" />
<Reference Include="Microsoft.CodeAnalysis"><HintPath>{escape(str(sdk / 'Roslyn/bincore/Microsoft.CodeAnalysis.dll'))}</HintPath></Reference>
<Reference Include="Microsoft.CodeAnalysis.CSharp"><HintPath>{escape(str(sdk / 'Roslyn/bincore/Microsoft.CodeAnalysis.CSharp.dll'))}</HintPath></Reference>
</ItemGroup></Project>''', encoding='utf-8')
    env = dict(os.environ, DOTNET_CLI_HOME=str(work / 'cli-home'),
               DOTNET_SKIP_FIRST_TIME_EXPERIENCE='1', DOTNET_CLI_TELEMETRY_OPTOUT='1')
    result = subprocess.run(['dotnet', 'build', str(project), '-c', 'Release',
        '--ignore-failed-sources', '--nologo'], env=env, capture_output=True, text=True)
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    return work / 'bin/Release' / target / 'Oracle.dll', version


def evaluate(args):
    root, binary, work = (pathlib.Path(p).resolve() for p in (args.root, args.binary, args.work_dir))
    repo = pathlib.Path(__file__).resolve().parent.parent
    git = ['git', '-c', f'safe.directory={root.as_posix()}', '-C', str(root)]
    paths = [str(root / p) for p in subprocess.check_output(git + ['ls-files', '-z'], text=True).split('\0') if p.endswith('.cs')]
    oracle, sdk = build_oracle(work)
    inputs = work / (args.label + '.inputs.json')
    inputs.write_text(json.dumps(paths), encoding='utf-8')
    truth_data = json.loads(subprocess.check_output(['dotnet', str(oracle), str(inputs), *args.define], text=True, encoding='utf-8'))
    counts = collections.Counter({k: 0 for k in ['files', 'unparseable', 'truth', 'bodyless_truth',
        'candidate', 'inactive_candidate', 'found', 'missing', 'false_positive', 'correct_end', 'wrong_end']})
    findings, artifacts = [], []
    for file in truth_data['files']:
        path = pathlib.Path(file['path'])
        rel = path.relative_to(root).as_posix()
        if file['errors']:
            counts['unparseable'] += 1
            findings.append({'file': rel, 'kind': 'unparseable', 'errors': file['errors']})
            continue
        truth = file['functions']
        result = subprocess.run([str(binary), '--inp', str(path), '--source', 'cs', '--map'], capture_output=True, text=True, encoding='utf-8')
        if result.returncode and 'No functions found in file' not in result.stderr:
            raise RuntimeError((rel, result.stderr))
        candidate = []
        for name, start, end in re.findall(r'([^;\n]+): (\d+)-(\d+);', result.stdout):
            fn = (name.strip(), int(start), int(end))
            if any(s <= fn[1] <= e for s, e in file['inactive']):
                counts['inactive_candidate'] += 1
                findings.append({'file': rel, 'kind': 'inactive_candidate', 'name': fn[0], 'start': fn[1], 'end': fn[2]})
            else:
                candidate.append(fn)
        counts.update(files=1, truth=len(truth), bodyless_truth=sum(n['bodyless'] for n in truth), candidate=len(candidate))
        used = set()
        for node in truth:
            matches = [(i, f) for i, f in enumerate(candidate) if i not in used and f[0] == node['name'] and node['start'] <= f[1] <= node['name_line']]
            if not matches:
                counts['missing'] += 1
                findings.append({'file': rel, 'kind': 'missing', **node})
                continue
            i, (_, start, end) = min(matches, key=lambda x: abs(x[1][1] - node['start']))
            used.add(i)
            counts['found'] += 1
            if end == node['end']:
                counts['correct_end'] += 1
            else:
                counts['wrong_end'] += 1
                findings.append({'file': rel, 'kind': 'wrong_end', **node, 'ff_start': start, 'ff_end': end})
        for i, (name, start, end) in enumerate(candidate):
            if i not in used:
                counts['false_positive'] += 1
                findings.append({'file': rel, 'kind': 'false_positive', 'name': name, 'start': start, 'end': end})
        artifacts.append({'file': rel, 'truth': truth, 'candidate': candidate})
    row = {'project': args.label, 'root': str(root), 'tracked_files': len(paths),
        'sha': subprocess.check_output(git + ['rev-parse', 'HEAD'], text=True).strip(),
        'dirty': bool(subprocess.check_output(git + ['status', '--porcelain'], text=True).strip()),
        'funcfinder_sha': subprocess.check_output(['git', '-C', str(repo), 'rev-parse', 'HEAD'], text=True).strip(),
        'funcfinder_dirty': bool(subprocess.check_output(['git', '-C', str(repo), 'status', '--porcelain'], text=True).strip()),
        'python_version': platform.python_version(), 'sdk_version': sdk, 'roslyn_version': truth_data['roslyn_version'],
        'language_version': truth_data['language_version'], 'features': truth_data['features'], 'defines': args.define,
        'oracle_sha256': hashlib.sha256(oracle.read_bytes()).hexdigest(),
        'binary': str(binary), 'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(), **counts}
    row['recall'] = counts['found'] / counts['truth'] if counts['truth'] else None
    row['precision'] = counts['found'] / counts['candidate'] if counts['candidate'] else None
    row['correct_span_rate'] = counts['correct_end'] / counts['truth'] if counts['truth'] else None
    output = pathlib.Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps({'summary': row, 'findings': findings, 'files': artifacts}, indent=2), encoding='utf-8')
    print(json.dumps(row), flush=True)


if __name__ == '__main__':
    p = argparse.ArgumentParser(description=__doc__)
    for flag in ['root', 'label', 'binary', 'output', 'work-dir']:
        p.add_argument('--' + flag, required=True)
    p.add_argument('--define', action='append', default=[])
    evaluate(p.parse_args())
