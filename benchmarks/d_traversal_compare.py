"""Measure production D parsing versus the shared recursive pilot on pinned clones.

Uses previously verified binaries from brace_pilot_build.py. Parsing benchmarks
include every tracked D file; CLI timing uses the complete directory command.
No parser implementation is modified by this measurement.
"""
import argparse
import collections
import hashlib
import json
import os
import pathlib
import platform
import random
import re
import subprocess
import time

from csharp_traversal_compare import digest, run, summarize


def compare(args):
    repo = pathlib.Path(__file__).resolve().parent.parent
    work = pathlib.Path(args.work_dir).resolve()
    work.mkdir(parents=True, exist_ok=True)
    build_dir = pathlib.Path(args.build_dir).resolve()
    build = json.loads((build_dir / 'build.json').read_text(encoding='utf-8'))
    binaries = {name: pathlib.Path(build['binaries'][key]['path'])
                for name, key in [('baseline', 'baseline'), ('recursive', 'pilot')]}
    for name, key in [('baseline', 'baseline'), ('recursive', 'pilot')]:
        if digest(binaries[name]) != build['binaries'][key]['sha256']:
            raise RuntimeError(f'{name} binary differs from verified build')
    for relative, expected in build['source_sha256'].items():
        if digest(repo / relative) != expected:
            raise RuntimeError(f'{relative} differs from verified pilot source')
    roots = [pathlib.Path(p).resolve() for p in args.root]
    env = dict(os.environ, GOPROXY='off', D_COMPARE_ROOTS=os.pathsep.join(map(str, roots)))
    report = {'revision': run(['git', 'rev-parse', 'HEAD'], cwd=repo, env=env).strip(),
              'go': run(['go', 'version'], cwd=repo, env=env).strip(), 'platform': platform.platform(),
              'cpu': platform.processor(), 'logical_cpus': os.cpu_count(), 'build': build,
              'bench_source_sha256': digest(repo / 'internal/d_traversal_bench_test.go'),
              'projects': [], 'method': {
                  'parser': 'complete D finder, preparation and class/function discovery; preloaded tracked D lines; no disk I/O',
                  'parser_samples_per_variant': args.bench_count * 2, 'benchtime': args.benchtime, 'GOMAXPROCS': 1,
                  'cli': 'startup, directory walk, reads, all function/type discovery and JSON capture; warm OS cache',
                  'cli_samples_per_variant': args.samples, 'warmups': args.warmups, 'workers': 'default (logical CPUs)',
                  'order': 'both parser traversal orders; balanced shuffled CLI pairs, seed 20261016',
                  'quality': 'recorded D oracle covers accepted files; timing includes all tracked D files, including six oracle exclusions',
                  'scope': 'two complete implementations, including adapters; does not isolate recursion cost'}}
    raw = ''
    for reverse in [False, True]:
        print('In-process benchmark: ' + ('recursive first' if reverse else 'baseline first'), flush=True)
        command = ['go', 'test', './internal', '-run=^$', '-bench=^BenchmarkDTraversal$',
                   '-benchmem', '-cpu=1', f'-benchtime={args.benchtime}', f'-count={args.bench_count}']
        output = run(command, cwd=repo, env=dict(env, D_COMPARE_REVERSE='1' if reverse else '0'))
        raw += output
        (work / 'in-process.txt').write_text(raw, encoding='utf-8')
    parsed = {}
    for line in raw.splitlines():
        match = re.match(r'BenchmarkDTraversal/([^/]+)/(baseline|recursive)\s+\d+\s+(.*)', line)
        if not match:
            continue
        project, variant, metrics = match.groups()
        row = parsed.setdefault(project, {}).setdefault(variant, {})
        for number, unit in re.findall(r'([\d.eE+\-]+)\s+(ns/op|MB/s|B/op|allocs/op|lines/s)', metrics):
            row.setdefault(unit, []).append(float(number))
    rng = random.Random(20261016)
    for root in roots:
        print(f'CLI benchmark: {root.name}', flush=True)
        git = ['git', '-C', str(root)]
        paths = [root / p for p in run(git + ['ls-files', '-z', '--', '*.d'], cwd=repo, env=env).split('\0') if p]
        project = {'name': root.name, 'root': str(root),
                   'pin': run(git + ['rev-parse', 'HEAD'], cwd=repo, env=env).strip(),
                   'dirty': bool(run(git + ['status', '--porcelain'], cwd=repo, env=env).strip()),
                   'tracked_d_files': len(paths), 'tracked_d_bytes': sum(p.stat().st_size for p in paths),
                   'tracked_d_lines': sum(len(p.read_bytes().splitlines()) for p in paths),
                   'parser': {v: {u: summarize(x) for u, x in metrics.items()}
                              for v, metrics in parsed[root.name].items()}, 'cli': {}}
        commands = {v: [str(b), '--dir', str(root), '--all', '--json', '--no-gitignore'] for v, b in binaries.items()}
        expected = {}
        for variant in binaries:
            for _ in range(args.warmups):
                result = run(commands[variant], cwd=repo, env=env)
                if variant in expected and result != expected[variant]:
                    raise AssertionError(f'{root}/{variant}: unstable output during warmup')
                expected[variant] = result
            (work / f'{root.name}.{variant}.map.json').write_text(expected[variant], encoding='utf-8')
        schedule = [['baseline', 'recursive'], ['recursive', 'baseline']] * ((args.samples + 1) // 2)
        schedule = schedule[:args.samples]
        rng.shuffle(schedule)
        measurements = {v: [] for v in binaries}
        for order in schedule:
            for variant in order:
                started = time.perf_counter_ns()
                output = run(commands[variant], cwd=repo, env=env)
                measurements[variant].append((time.perf_counter_ns() - started) / 1e6)
                if output != expected[variant]:
                    raise AssertionError(f'{root}/{variant}: unstable output')
        project['cli_order'] = schedule
        project['cli_outputs_equal'] = expected['baseline'] == expected['recursive']
        maps = {v: json.loads(output) for v, output in expected.items()}
        def functions(mapped):
            return sorted((file['path'], json.dumps(fn, sort_keys=True))
                          for file in mapped['files'] for fn in file.get('functions', []))
        project['cli_functions_equal_ignoring_order'] = functions(maps['baseline']) == functions(maps['recursive'])
        for variant, values in measurements.items():
            mapped = maps[variant]
            project['cli'][variant] = {'command': commands[variant], 'milliseconds': summarize(values),
                'output_sha256': hashlib.sha256(expected[variant].encode()).hexdigest(),
                'mapped_files': mapped['total_files'], 'mapped_functions': mapped['total_functions'],
                'mapped_classes': mapped['total_classes'],
                'mapped_extensions': dict(collections.Counter(pathlib.Path(f['path']).suffix for f in mapped['files']))}
        report['projects'].append(project)
        (work / 'comparison.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
        print(json.dumps({'project': root.name,
            'parser_ms': {v: project['parser'][v]['ns/op']['median'] / 1e6 for v in binaries},
            'cli_ms': {v: project['cli'][v]['milliseconds']['median'] for v in binaries},
            'allocated_bytes': {v: project['parser'][v]['B/op']['median'] for v in binaries}}), flush=True)
    print(f'Results: {work / "comparison.json"}', flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', action='append', required=True)
    parser.add_argument('--build-dir', required=True)
    parser.add_argument('--work-dir', required=True)
    parser.add_argument('--bench-count', type=int, default=5, help='samples per parser order')
    parser.add_argument('--benchtime', default='1s')
    parser.add_argument('--samples', type=int, default=20)
    parser.add_argument('--warmups', type=int, default=2)
    arguments = parser.parse_args()
    if arguments.bench_count < 1 or arguments.samples < 2 or arguments.warmups < 1:
        parser.error('bench-count >= 1, samples >= 2, warmups >= 1 required')
    compare(arguments)
