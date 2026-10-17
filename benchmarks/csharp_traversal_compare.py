"""Compare linear and recursive C# scope traversal on pinned local repositories.

The recursive prototype stays test-only. A Go overlay builds/tests it as the CLI
finder without modifying production source or adding a runtime switch.
"""
import argparse
import hashlib
import json
import os
import pathlib
import platform
import random
import re
import statistics
import subprocess
import sys
import time


def run(command, *, cwd, env):
    result = subprocess.run(command, cwd=cwd, env=env, capture_output=True, text=True,
                            encoding='utf-8', errors='replace')
    if result.returncode:
        raise RuntimeError(f'{command}\n{result.stdout}\n{result.stderr}')
    return result.stdout


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def size(text):
    lines = text.splitlines()
    return {'bytes_lf_utf8': len(text.encode('utf-8')), 'lines': len(lines),
            'nonblank_noncomment_lines': sum(bool(s.strip()) and not s.lstrip().startswith('//') for s in lines)}


def summarize(values):
    ordered = sorted(values)
    return {'median': statistics.median(values), 'min': min(values), 'max': max(values),
            'p10': ordered[int((len(ordered) - 1) * .1)],
            'p90': ordered[int((len(ordered) - 1) * .9)], 'samples': values}


def compare(args):
    repo = pathlib.Path(__file__).resolve().parent.parent
    work = pathlib.Path(args.work_dir).resolve()
    work.mkdir(parents=True, exist_ok=True)
    roots = [pathlib.Path(p).resolve() for p in args.root]
    env = dict(os.environ, GOPROXY='off', CSHARP_COMPARE_ROOTS=os.pathsep.join(map(str, roots)))
    env.pop('CSHARP_COMPARE_REVERSE', None)
    production = repo / 'internal/csharp_finder.go'
    prototype = repo / 'internal/csharp_recursive_test.go'
    current = production.read_text(encoding='utf-8')
    experimental = prototype.read_text(encoding='utf-8')
    entry = 'func (f *Finder) findCSharpFunctions('
    recursive_entry = 'func (f *Finder) findCSharpFunctionsRecursive('
    shared, linear = current.split(entry, 1)
    linear = entry + linear
    recursive = entry + experimental.split(recursive_entry, 1)[1]
    # A non-Go extension prevents `go test ./...` from treating this generated
    # overlay input as a standalone package under build/.
    replacement = work / 'csharp_finder_recursive.go.txt'
    replacement.write_text(shared + recursive, encoding='utf-8', newline='\n')
    overlay = work / 'overlay.json'
    overlay.write_text(json.dumps({'Replace': {str(production): str(replacement)}}), encoding='utf-8')
    suffix = '.exe' if os.name == 'nt' else ''
    binaries = {variant: work / ('funcfinder-' + variant + suffix) for variant in ['linear', 'recursive']}
    report = {'scope': 'brace-scope traversal; identical sanitizer, tokenizer, declaration and end rules',
              'revision': run(['git', 'rev-parse', 'HEAD'], cwd=repo, env=env).strip(),
              'dirty': run(['git', 'status', '--porcelain'], cwd=repo, env=env).strip(),
              'platform': platform.platform(), 'cpu': platform.processor(), 'logical_cpus': os.cpu_count(),
              'go': run(['go', 'version'], cwd=repo, env=env).strip(), 'python': platform.python_version(),
              'source': {'shared': size(shared), 'linear_traversal': size(linear),
                         'recursive_traversal': size(recursive), 'linear_total': size(current),
                         'recursive_total': size(shared + recursive)}, 'binaries': {}, 'projects': []}
    report['source']['linear_sha256'] = digest(production)
    report['source']['recursive_sha256'] = digest(replacement)
    report['source']['prototype_sha256'] = digest(prototype)
    for variant, binary in binaries.items():
        print(f'Build and test: {variant}', flush=True)
        extra = ['-overlay', str(overlay)] if variant == 'recursive' else []
        build = ['go', 'build', *extra, '-trimpath', '-buildvcs=false', '-ldflags=-buildid=',
                 '-o', str(binary), './cmd/funcfinder']
        run(build, cwd=repo, env=env)
        test = run(['go', 'test', *extra, './...', '-count=1'], cwd=repo, env=env)
        (work / f'{variant}.tests.txt').write_text(test, encoding='utf-8')
        report['binaries'][variant] = {'path': str(binary), 'bytes': binary.stat().st_size,
                                       'sha256': digest(binary), 'build_command': build}
    print('Benchmark: in-process, serial, preloaded files; forward and reverse order', flush=True)
    bench_text = ''
    for reverse in [False, True]:
        bench_env = dict(env, CSHARP_COMPARE_REVERSE='1' if reverse else '0')
        output = run(['go', 'test', './internal', '-run=^$', '-bench=^BenchmarkCSharpTraversal$',
                      '-benchmem', '-cpu=1', f'-benchtime={args.benchtime}', f'-count={args.bench_count}'],
                     cwd=repo, env=bench_env)
        bench_text += output
        (work / 'in-process.txt').write_text(bench_text, encoding='utf-8')
    parsed = {}
    for line in bench_text.splitlines():
        match = re.match(r'BenchmarkCSharpTraversal/([^/]+)/(linear|recursive)\s+\d+\s+(.*)', line)
        if not match:
            continue
        project, variant, metrics = match.groups()
        row = parsed.setdefault(project.lower(), {}).setdefault(variant, {})
        for number, unit in re.findall(r'([\d.eE+\-]+)\s+(ns/op|MB/s|B/op|allocs/op|lines/s)', metrics):
            row.setdefault(unit, []).append(float(number))
    rng = random.Random(20261016)
    for root in roots:
        print(f'Benchmark CLI and verify Roslyn: {root.name}', flush=True)
        git = ['git', '-C', str(root)]
        paths = run(git + ['ls-files', '-z', '--', '*.cs'], cwd=repo, env=env).split('\0')
        paths = [root / p for p in paths if p]
        project = {'name': root.name, 'root': str(root),
                   'revision': run(git + ['rev-parse', 'HEAD'], cwd=repo, env=env).strip(),
                   'dirty': bool(run(git + ['status', '--porcelain'], cwd=repo, env=env).strip()),
                   'tracked_cs_files': len(paths), 'tracked_cs_bytes': sum(p.stat().st_size for p in paths),
                   'tracked_cs_lines': sum(len(p.read_bytes().splitlines()) for p in paths),
                   'in_process': {k: {unit: summarize(v) for unit, v in metrics.items()}
                                  for k, metrics in parsed[root.name.lower()].items()},
                   'cli': {}, 'roslyn': {}}
        commands = {v: [str(b), '--dir', str(root), '--all', '--json', '--no-gitignore'] for v, b in binaries.items()}
        expected = None
        for variant in binaries:
            for _ in range(args.warmups):
                output = run(commands[variant], cwd=repo, env=env)
                if expected is None:
                    expected = output
                if output != expected:
                    raise AssertionError(f'{root}: CLI outputs differ ({variant})')
        measurements = {v: [] for v in binaries}
        schedule = [['linear', 'recursive'], ['recursive', 'linear']] * ((args.samples + 1) // 2)
        schedule = schedule[:args.samples]
        rng.shuffle(schedule)
        for order in schedule:
            for variant in order:
                started = time.perf_counter_ns()
                output = run(commands[variant], cwd=repo, env=env)
                elapsed_ms = (time.perf_counter_ns() - started) / 1e6
                if output != expected:
                    raise AssertionError(f'{root}: CLI output changed ({variant})')
                measurements[variant].append(elapsed_ms)
        cli_map = json.loads(expected)
        project['cli_output'] = {'sha256': hashlib.sha256(expected.encode()).hexdigest(),
                                 'total_files': cli_map['total_files'],
                                 'total_functions': cli_map['total_functions']}
        project['cli_order'] = schedule
        for variant, values in measurements.items():
            project['cli'][variant] = {'command': commands[variant], 'milliseconds': summarize(values)}
        for variant, binary in binaries.items():
            target = work / f'{root.name}.{variant}.roslyn.json'
            run([sys.executable, str(repo / 'benchmarks/csharp_roslyn_eval.py'),
                 '--root', str(root), '--binary', str(binary), '--label', root.name + '-' + variant,
                 '--output', str(target), '--work-dir', str(work / 'oracle')], cwd=repo, env=env)
            summary = json.loads(target.read_text(encoding='utf-8'))['summary']
            if any(summary[k] for k in ['unparseable', 'missing', 'false_positive', 'wrong_end']):
                raise AssertionError(f'{root}: Roslyn mismatch ({variant}): {summary}')
            project['roslyn'][variant] = summary
        report['projects'].append(project)
        (work / 'comparison.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
    report['method'] = {'in_process_samples_per_variant': args.bench_count * 2,
                        'in_process_benchtime_per_sample': args.benchtime, 'in_process_gomaxprocs': 1,
                        'in_process_includes': 'finder construction, sanitizing, tokenizing, traversal; no disk I/O',
                        'cli_samples_per_variant': args.samples, 'cli_warmups_per_variant': args.warmups,
                        'cli_includes': 'process startup, directory walk, reading, parsing, types, JSON capture',
                        'cli_workers': 'tool default (logical CPU count)', 'cache': 'warm OS file cache',
                        'source_count': 'LF-normalized UTF-8; full effective finder and traversal separately; no tests',
                        'binary_count': 'same Go flags, no stripping, no build VCS metadata or build ID'}
    (work / 'comparison.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
    print(f'Results: {work / "comparison.json"}', flush=True)
    for project in report['projects']:
        print(json.dumps({'project': project['name'],
            'parser_ms': {v: project['in_process'][v]['ns/op']['median'] / 1e6 for v in binaries},
            'cli_ms': {v: project['cli'][v]['milliseconds']['median'] for v in binaries},
            'truth': {v: project['roslyn'][v]['truth'] for v in binaries}}), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', action='append', required=True, help='pinned local repository; repeat for each project')
    parser.add_argument('--work-dir', required=True, help='ignored output/cache directory')
    parser.add_argument('--samples', type=int, default=20)
    parser.add_argument('--warmups', type=int, default=2)
    parser.add_argument('--bench-count', type=int, default=5, help='samples per traversal order')
    parser.add_argument('--benchtime', default='1s')
    arguments = parser.parse_args()
    if arguments.samples < 2 or arguments.warmups < 1 or arguments.bench_count < 1:
        parser.error('samples >= 2, warmups >= 1 and bench-count >= 1 are required')
    compare(arguments)
