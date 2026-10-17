"""Compare bounded within-file scope parallelism with the serial pilot and production.

Reuses verified production/serial binaries. Builds a third overlay, checks exact
serial-pilot output parity, and sweeps file/scope worker counts at full Go CPU capacity.
"""
import argparse
import ctypes
import json
import os
import pathlib
import random
import subprocess
import time

from csharp_traversal_compare import digest, run, summarize


def process_cpu(process):
    if os.name != 'nt':
        return None
    from ctypes import wintypes
    values = [wintypes.FILETIME() for _ in range(4)]
    query = ctypes.WinDLL('kernel32', use_last_error=True).GetProcessTimes
    query.argtypes = [wintypes.HANDLE, *[ctypes.POINTER(wintypes.FILETIME)] * 4]
    query.restype = wintypes.BOOL
    if not query(int(process._handle), *(ctypes.byref(v) for v in values)):
        return None
    return sum((v.dwHighDateTime << 32) | v.dwLowDateTime for v in values[2:]) / 1e7


def timed(command, *, repo, env):
    started = time.perf_counter_ns()
    process = subprocess.Popen(command, cwd=repo, env=env, stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, text=True, encoding='utf-8')
    output, error = process.communicate()
    elapsed = (time.perf_counter_ns() - started) / 1e9
    cpu = process_cpu(process)
    if process.returncode:
        raise RuntimeError(f'{command}\n{output}\n{error}')
    return output, elapsed * 1000, cpu


def compare(args):
    repo = pathlib.Path(__file__).resolve().parent.parent
    work = pathlib.Path(args.work_dir).resolve()
    work.mkdir(parents=True, exist_ok=True)
    previous = pathlib.Path(args.serial_build_dir).resolve()
    verified = json.loads((previous / 'build.json').read_text(encoding='utf-8'))
    env = dict(os.environ, GOPROXY='off', PYTHONDONTWRITEBYTECODE='1')
    cpus = os.cpu_count() or 1
    env['GOMAXPROCS'] = str(cpus)
    for relative, expected in verified['source_sha256'].items():
        if digest(repo / relative) != expected:
            raise RuntimeError(f'{relative}: serial pilot source changed')
    binaries = {v: pathlib.Path(verified['binaries'][k]['path'])
                for v, k in [('baseline', 'baseline'), ('serial', 'pilot')]}
    for v, k in [('baseline', 'baseline'), ('serial', 'pilot')]:
        if digest(binaries[v]) != verified['binaries'][k]['sha256']:
            raise RuntimeError(f'{v}: verified binary changed')
    overlay_data = json.loads((previous / 'overlay.json').read_text(encoding='utf-8'))
    source = repo / 'internal/finder.go'
    text = pathlib.Path(overlay_data['Replace'][str(source)]).read_text(encoding='utf-8')
    old_call = 'return f.findBraceFunctionsRecursive(lines, startLine, filename)'
    if text.count(old_call) != 1:
        raise RuntimeError('serial pilot dispatch changed')
    injected = work / 'finder.parallel.go.txt'
    injected.write_text(text.replace(old_call, old_call.replace('Recursive', 'Parallel')),
                        encoding='utf-8', newline='\n')
    overlay_data['Replace'][str(source)] = str(injected)
    empty = work / 'empty.go.txt'
    empty.write_text('package internal\n', encoding='utf-8')
    helper = repo / 'internal/brace_parallel_test.go'
    overlay_data['Replace'][str(repo / 'internal/brace_parallel_pilot.go')] = str(helper)
    overlay_data['Replace'][str(helper)] = str(empty)
    overlay = work / 'overlay.json'
    overlay.write_text(json.dumps(overlay_data), encoding='utf-8')
    binaries['parallel'] = work / ('funcfinder-parallel.exe' if os.name == 'nt' else 'funcfinder-parallel')
    build = ['go', 'build', '-overlay', str(overlay), '-trimpath', '-buildvcs=false',
             '-o', str(binaries['parallel']), './cmd/funcfinder']
    run(build, cwd=repo, env=env)
    roots = [pathlib.Path(p).resolve() for p in args.root]
    projects = [{'Language': args.language, 'Root': str(p)} for p in roots]
    for value in args.parity_project:
        language, root = value.split('=', 1)
        projects.append({'Language': language, 'Root': str(pathlib.Path(root).resolve())})
    test_env = dict(env, BRACE_PARALLEL_PROJECTS=json.dumps(projects))
    print('Verify parallel overlay: complete tests, corpus parity and vet', flush=True)
    for name, command in [('tests', ['go', 'test', '-overlay', str(overlay), './...', '-count=1']),
                          ('vet', ['go', 'vet', '-overlay', str(overlay), './...'])]:
        result = run(command, cwd=repo, env=test_env)
        (work / f'parallel.{name}.txt').write_text(result, encoding='utf-8')
    report = {'revision': run(['git', 'rev-parse', 'HEAD'], cwd=repo, env=env).strip(),
              'go': run(['go', 'version'], cwd=repo, env=env).strip(), 'logical_cpus': cpus,
              'build_command': build, 'parallel_source_sha256': digest(helper),
              'binaries': {v: {'path': str(p), 'sha256': digest(p)} for v, p in binaries.items()},
              'parity_projects': projects, 'projects': [], 'method': {
                  'samples': args.samples, 'warmups': args.warmups, 'GOMAXPROCS': cpus,
                  'scope_workers': args.scope_workers, 'min_scope_tokens': args.min_tokens,
                  'file_workers': [1, cpus], 'order': 'shuffled rounds; seed 20261016',
                  'scope_budget': 'process-wide workers-1 extra tasks; small/saturated scopes run inline',
                  'cpu': 'Windows child kernel+user CPU seconds / wall seconds; utilization normalized by logical CPUs',
                  'timing': 'whole directory CLI, warm caches, captured JSON; includes serial preparation/tokenization'}}
    rng = random.Random(20261016)
    for root in roots:
        print(f'Measure file/scope worker matrix: {root.name}', flush=True)
        cases = []
        for files in sorted(set([1, cpus])):
            for variant in ['baseline', 'serial']:
                cases.append({'name': f'{variant}-files{files}', 'variant': variant,
                              'file_workers': files, 'scope_workers': 1})
            for scopes in sorted(set(min(cpus, n) for n in args.scope_workers)):
                cases.append({'name': f'parallel{scopes}-files{files}', 'variant': 'parallel',
                              'file_workers': files, 'scope_workers': scopes})
        outputs, measurements, cpu_samples = {}, {}, {}
        for case in cases:
            name, variant = case['name'], case['variant']
            case['command'] = [str(binaries[variant]), '--dir', str(root), '--all', '--json',
                               '--no-gitignore', '--workers', str(case['file_workers'])]
            case_env = dict(env, FUNCFINDER_SCOPE_WORKERS=str(case['scope_workers']),
                            FUNCFINDER_SCOPE_MIN_TOKENS=str(args.min_tokens))
            for _ in range(args.warmups):
                output, _, _ = timed(case['command'], repo=repo, env=case_env)
                if name in outputs and outputs[name] != output:
                    raise AssertionError(f'{name}: unstable warmup output')
                outputs[name] = output
            measurements[name], cpu_samples[name] = [], []
        serial_output = outputs[f'serial-files{cpus}']
        for case in cases:
            if case['variant'] != 'baseline' and outputs[case['name']] != serial_output:
                raise AssertionError(f'{root}: parallel output differs ({case["name"]})')
        orders = []
        for sample in range(args.samples):
            order = list(range(len(cases)))
            rng.shuffle(order)
            orders.append([cases[i]['name'] for i in order])
            for index in order:
                case = cases[index]
                case_env = dict(env, FUNCFINDER_SCOPE_WORKERS=str(case['scope_workers']),
                                FUNCFINDER_SCOPE_MIN_TOKENS=str(args.min_tokens))
                output, wall_ms, cpu = timed(case['command'], repo=repo, env=case_env)
                if output != outputs[case['name']]:
                    raise AssertionError(f'{root}/{case["name"]}: unstable measured output')
                measurements[case['name']].append(wall_ms)
                if cpu is not None:
                    cpu_samples[case['name']].append({'cpu_s': cpu, 'effective_cores': cpu / (wall_ms / 1000),
                        'total_cpu_percent': cpu / (wall_ms / 1000) / cpus * 100})
            print(f'{root.name}: measured round {sample+1}/{args.samples}', flush=True)
        for case in cases:
            name = case['name']
            case['milliseconds'] = summarize(measurements[name])
            case['cpu_samples'] = cpu_samples[name]
            if cpu_samples[name]:
                for key in ['cpu_s', 'effective_cores', 'total_cpu_percent']:
                    case[key] = summarize([v[key] for v in cpu_samples[name]])
        row = {'name': root.name, 'root': str(root), 'language': args.language,
               'pin': run(['git', '-C', str(root), 'rev-parse', 'HEAD'], cwd=repo, env=env).strip(),
               'cases': cases, 'orders': orders, 'parallel_output_exactly_matches_serial': True}
        report['projects'].append(row)
        (work / 'comparison.json').write_text(json.dumps(report, indent=2), encoding='utf-8')
        for case in cases:
            print(f'{root.name} {case["name"]}: {case["milliseconds"]["median"]:.3f} ms, '
                  f'{case.get("total_cpu_percent", {}).get("median", 0):.1f}% total CPU', flush=True)
    print(f'Results: {work / "comparison.json"}', flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', action='append', required=True)
    parser.add_argument('--language', choices=['cs', 'java', 'd'], default='d',
                        help='corpus parity language for --root projects (default: d)')
    parser.add_argument('--serial-build-dir', required=True)
    parser.add_argument('--work-dir', required=True)
    parser.add_argument('--parity-project', action='append', default=[], help='extra language=path for corpus parity')
    parser.add_argument('--scope-workers', type=int, nargs='+', default=[4, 16, 32])
    parser.add_argument('--min-tokens', type=int, default=2048)
    parser.add_argument('--samples', type=int, default=3)
    parser.add_argument('--warmups', type=int, default=1)
    arguments = parser.parse_args()
    if min(arguments.samples, arguments.warmups, arguments.min_tokens, *arguments.scope_workers) < 1:
        parser.error('all counts must be positive')
    compare(arguments)
