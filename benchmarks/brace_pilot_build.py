"""Build/test the shared recursive-scope pilot for C#, Java and D using a Go overlay.

Python and Ruby are excluded. Default production dispatch remains unchanged.
Use the existing Roslyn and Scala/D/Java evaluators on the resulting executable.
"""
import argparse
import hashlib
import json
import os
import pathlib
import subprocess


def build(args):
    repo = pathlib.Path(__file__).resolve().parent.parent
    work = pathlib.Path(args.work_dir).resolve()
    work.mkdir(parents=True, exist_ok=True)
    empty = work / 'empty.go.txt'
    empty.write_text('package internal\n', encoding='utf-8')
    replacements, hashes = {}, {}
    for name in ['brace_recursive', 'brace_adapters']:
        source = repo / 'internal' / (name + '_test.go')
        # Add the implementation as ordinary Go source for CLI compilation;
        # hide its test-file copy to avoid duplicate definitions under go test.
        replacements[str(repo / 'internal' / (name + '_pilot.go'))] = str(source)
        replacements[str(source)] = str(empty)
        hashes[source.relative_to(repo).as_posix()] = hashlib.sha256(source.read_bytes()).hexdigest()
    source = repo / 'internal/finder.go'
    text = source.read_text(encoding='utf-8')
    entry = 'func (f *Finder) FindFunctionsInLines(lines []string, startLine int, filename string) (*FindResult, error) {'
    if text.count(entry) != 1:
        raise RuntimeError('production entry point changed; review the overlay insertion')
    text = text.replace(entry, entry + '''
    if f.config.LangKey == "cs" || f.config.LangKey == "java" || f.config.LangKey == "d" {
        return f.findBraceFunctionsRecursive(lines, startLine, filename)
    }''', 1)
    injected = work / 'finder.go.txt'
    injected.write_text(text, encoding='utf-8', newline='\n')
    replacements[str(source)] = str(injected)
    overlay = work / 'overlay.json'
    overlay.write_text(json.dumps({'Replace': replacements}), encoding='utf-8')
    env = dict(os.environ)
    if args.cs_root:
        env['CSHARP_COMPARE_ROOTS'] = os.pathsep.join(str(pathlib.Path(p).resolve()) for p in args.cs_root)
    suffix = '.exe' if os.name == 'nt' else ''
    binaries, tests = {}, {}
    for variant in ['baseline', 'pilot']:
        extra = ['-overlay', str(overlay)] if variant == 'pilot' else []
        binary = work / ('funcfinder-' + variant + suffix)
        command = ['go', 'build', *extra, '-trimpath', '-buildvcs=false', '-o', str(binary), './cmd/funcfinder']
        subprocess.run(command, cwd=repo, env=env, check=True)
        binaries[variant] = {'path': str(binary), 'sha256': hashlib.sha256(binary.read_bytes()).hexdigest(),
                             'bytes': binary.stat().st_size, 'command': command}
        if not args.skip_tests:
            for check, command in [('tests', ['go', 'test', *extra, './...', '-count=1']),
                                   ('vet', ['go', 'vet', *extra, './...'])]:
                result = subprocess.run(command, cwd=repo, env=env, capture_output=True, text=True, encoding='utf-8')
                (work / f'{variant}.{check}.txt').write_text(result.stdout + result.stderr, encoding='utf-8')
                result.check_returncode()
                tests[f'{variant}.{check}'] = 'pass'
        print(f'{variant}: {binary}', flush=True)
    result = {'revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=repo, text=True).strip(),
              'go': subprocess.check_output(['go', 'version'], text=True).strip(),
              'languages': ['cs', 'java', 'd'], 'source_sha256': hashes,
              'overlay_sha256': hashlib.sha256(injected.read_bytes()).hexdigest(),
              'binaries': binaries, 'checks': tests}
    (work / 'build.json').write_text(json.dumps(result, indent=2), encoding='utf-8')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--work-dir', required=True, help='ignored build/cache directory')
    parser.add_argument('--cs-root', action='append', default=[], help='pinned C# root for complete-result parity; repeat')
    parser.add_argument('--skip-tests', action='store_true', help='build only during development')
    build(parser.parse_args())
