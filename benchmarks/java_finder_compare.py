"""Measure complete Java CLI runs before/after a parser fix and against an optional pilot."""
import argparse
import json
import os
import pathlib
import random

from brace_parallel_compare import timed
from csharp_traversal_compare import digest, run, summarize


def compare(args):
    repo = pathlib.Path(__file__).resolve().parent.parent
    root, work = pathlib.Path(args.root).resolve(), pathlib.Path(args.work_dir).resolve()
    work.mkdir(parents=True, exist_ok=True)
    binaries = {'before': pathlib.Path(args.before).resolve(), 'after': pathlib.Path(args.after).resolve()}
    if args.pilot:
        binaries['pilot'] = pathlib.Path(args.pilot).resolve()
    cpus = os.cpu_count() or 1
    env = dict(os.environ, GOMAXPROCS=str(cpus), PYTHONDONTWRITEBYTECODE='1')
    report = {'revision': run(['git', 'rev-parse', 'HEAD'], cwd=repo, env=env).strip(),
              'go': run(['go', 'version'], cwd=repo, env=env).strip(),
              'pin': run(['git', '-C', str(root), 'rev-parse', 'HEAD'], cwd=repo, env=env).strip(),
              'root': str(root), 'logical_cpus': cpus,
              'source_sha256': {p: digest(repo / p) for p in [
                  'internal/java_finder.go', 'internal/finder.go', 'internal/csharp_finder.go',
                  'internal/callgraph.go', 'cmd/funcstats/main.go']},
              'binaries': {v: {'path': str(p), 'sha256': digest(p)} for v, p in binaries.items()},
              'method': {'samples': args.samples, 'warmups': args.warmups, 'GOMAXPROCS': cpus,
                         'order': 'shuffled rounds, seed 20261016',
                         'timing': 'whole directory CLI, warm caches, captured JSON',
                         'cpu': 'Windows child kernel+user CPU / wall / logical CPUs; run average, not peak'},
              'cases': [], 'orders': []}
    outputs = {}
    for workers in sorted(set([1, cpus])):
        for variant, binary in binaries.items():
            name = f'{variant}-files{workers}'
            command = [str(binary), '--dir', str(root), '--all', '--json', '--no-gitignore',
                       '--workers', str(workers)]
            for _ in range(args.warmups):
                output, _, _ = timed(command, repo=repo, env=env)
                if name in outputs and output != outputs[name]:
                    raise AssertionError(f'{name}: unstable warmup output')
                outputs[name] = output
            data = json.loads(output)
            output_path = work / f'{name}.map.json'
            output_path.write_text(output, encoding='utf-8')
            report['cases'].append({'name': name, 'command': command,
                'counts': {k: data.get(k) for k in ['total_files', 'total_functions', 'total_classes']},
                'map_sha256': digest(output_path), 'samples_ms': [], 'cpu_samples': []})
    rng = random.Random(20261016)
    for sample in range(args.samples):
        order = list(range(len(report['cases'])))
        rng.shuffle(order)
        report['orders'].append([report['cases'][i]['name'] for i in order])
        for index in order:
            case = report['cases'][index]
            output, ms, cpu = timed(case['command'], repo=repo, env=env)
            if output != outputs[case['name']]:
                raise AssertionError(f'{case["name"]}: unstable measured output')
            case['samples_ms'].append(ms)
            if cpu is not None:
                case['cpu_samples'].append(cpu / (ms / 1000) / cpus * 100)
        print(f'Java CLI round {sample+1}/{args.samples}', flush=True)
    for case in report['cases']:
        case['milliseconds'] = summarize(case['samples_ms'])
        if case['cpu_samples']:
            case['average_total_cpu_percent'] = summarize(case['cpu_samples'])
        print(f'{case["name"]}: {case["milliseconds"]["median"]:.3f} ms', flush=True)
    (work / 'comparison.json').write_text(json.dumps(report, indent=2), encoding='utf-8')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', required=True)
    parser.add_argument('--before', required=True)
    parser.add_argument('--after', required=True)
    parser.add_argument('--pilot')
    parser.add_argument('--work-dir', required=True)
    parser.add_argument('--samples', type=int, default=20)
    parser.add_argument('--warmups', type=int, default=2)
    arguments = parser.parse_args()
    if min(arguments.samples, arguments.warmups) < 1:
        parser.error('sample and warmup counts must be positive')
    compare(arguments)
