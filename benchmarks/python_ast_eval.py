"""Compare tracked Python functions with native AST, retaining duplicate names."""
import argparse
import ast
import collections
import hashlib
import json
import pathlib
import platform
import re
import subprocess
import tokenize

REPO = pathlib.Path(__file__).resolve().parent.parent


def run(root, name, binary, output):
    root = pathlib.Path(root).resolve()
    binary = pathlib.Path(binary).resolve()
    git = ['git', '-c', f'safe.directory={root.as_posix()}', '-C', str(root)]
    sha = subprocess.check_output(git + ['rev-parse', 'HEAD'], text=True).strip()
    dirty = bool(subprocess.check_output(git + ['status', '--porcelain'], text=True).strip())
    paths = [root / p for p in subprocess.check_output(git + ['ls-files', '-z'], text=True).split('\0') if p.endswith('.py')]
    counts = collections.Counter({key: 0 for key in ['files', 'unparseable', 'truth', 'candidate', 'found', 'missing', 'false_positive', 'correct_end', 'wrong_end']})
    findings = []
    artifacts = []
    for path in sorted(paths):
        with tokenize.open(path) as stream:
            source = stream.read()
        lines = source.split('\n')
        rel = path.relative_to(root).as_posix()
        try:
            tree = ast.parse(source, filename=rel)
        except SyntaxError as exc:
            counts['unparseable'] += 1
            findings.append({'file': rel, 'kind': 'unparseable', 'detail': str(exc)})
            continue
        truth = sorted((n for n in ast.walk(tree) if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef))), key=lambda n: n.lineno)
        result = subprocess.run([str(binary), '--inp', str(path), '--source', 'py', '--map'], capture_output=True, text=True, encoding='utf-8')
        if result.returncode and 'No functions found in file' not in result.stderr:
            raise RuntimeError((rel, result.stderr))
        candidate = [(n.strip(), int(s), int(e)) for n, s, e in re.findall(r'([^;\n]+): (\d+)-(\d+);', result.stdout)]
        counts['files'] += 1
        counts['truth'] += len(truth)
        counts['candidate'] += len(candidate)
        used = set()
        for node in truth:
            start = min([node.lineno] + [d.lineno for d in node.decorator_list])
            matches = [(i, f) for i, f in enumerate(candidate) if i not in used and f[0] == node.name and start <= f[1] <= node.lineno]
            if not matches:
                counts['missing'] += 1
                findings.append({'file': rel, 'kind': 'missing', 'name': node.name, 'start': start, 'def_line': node.lineno, 'end': node.end_lineno})
                continue
            i, (_, ff_start, ff_end) = min(matches, key=lambda x: abs(x[1][1] - start))
            used.add(i)
            counts['found'] += 1
            end = ff_end
            while ff_start < end <= len(lines) and (not lines[end-1].strip() or lines[end-1].lstrip().startswith('#')):
                end -= 1
            if end == node.end_lineno:
                counts['correct_end'] += 1
            else:
                counts['wrong_end'] += 1
                findings.append({'file': rel, 'kind': 'wrong_end', 'name': node.name, 'start': start, 'def_line': node.lineno, 'truth_end': node.end_lineno, 'ff_end': ff_end, 'normalized_end': end})
        for i, (n, s, e) in enumerate(candidate):
            if i not in used:
                counts['false_positive'] += 1
                findings.append({'file': rel, 'kind': 'false_positive', 'name': n, 'start': s, 'end': e})
        artifacts.append({'file': rel, 'truth': [{'name': n.name, 'line': n.lineno, 'end': n.end_lineno} for n in truth], 'candidate': candidate})
    revision = subprocess.check_output(['git', '-C', str(REPO), 'rev-parse', 'HEAD'], text=True).strip()
    source_dirty = bool(subprocess.check_output(['git', '-C', str(REPO), 'status', '--porcelain'], text=True).strip())
    row = {'project': name, 'root': str(root), 'sha': sha, 'dirty': dirty, 'funcfinder_sha': revision,
           'funcfinder_dirty': source_dirty,
           'python_version': platform.python_version(), 'binary': str(binary),
           'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(), **counts}
    row['recall'] = counts['found'] / counts['truth'] if counts['truth'] else None
    row['precision'] = counts['found'] / counts['candidate'] if counts['candidate'] else None
    row['correct_span_rate'] = counts['correct_end'] / counts['truth'] if counts['truth'] else None
    output = pathlib.Path(output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps({'summary': row, 'findings': findings, 'files': artifacts}, indent=2), encoding='utf-8')
    print(json.dumps(row), flush=True)
    return row


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', required=True, help='Git checkout to evaluate (tracked .py files)')
    parser.add_argument('--label', required=True, help='Project label for the report')
    parser.add_argument('--binary', required=True, help='Built funcfinder executable')
    parser.add_argument('--output', required=True, help='JSON output path (includes every mismatch)')
    args = parser.parse_args()
    run(args.root, args.label, args.binary, args.output)
