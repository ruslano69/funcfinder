"""Compare tracked PHP named functions/methods with tree-sitter, including prototypes."""
import argparse
import bisect
import collections
import hashlib
import importlib.metadata
import json
import pathlib
import platform
import re
import subprocess
import sys


def evaluate(args):
    if args.module_dir:
        sys.path.insert(0, str(pathlib.Path(args.module_dir).resolve()))
    from tree_sitter import Language, Parser
    import tree_sitter_php

    parser = Parser(Language(tree_sitter_php.language_php()))
    root = pathlib.Path(args.root).resolve()
    binary = pathlib.Path(args.binary).resolve()
    repo = pathlib.Path(__file__).resolve().parent.parent
    git = ['git', '-c', f'safe.directory={root.as_posix()}', '-C', str(root)]
    paths = [root / p for p in subprocess.check_output(git + ['ls-files', '-z'], text=True).split('\0') if p.endswith('.php')]
    counts = collections.Counter({k: 0 for k in ['files', 'unparseable', 'truth', 'bodyless_truth', 'candidate', 'found', 'missing', 'false_positive', 'correct_end', 'wrong_end']})
    findings = []
    artifacts = []
    for path in sorted(paths):
        source = path.read_bytes()
        rel = path.relative_to(root).as_posix()
        tree = parser.parse(source)
        if tree.root_node.has_error:
            counts['unparseable'] += 1
            findings.append({'file': rel, 'kind': 'unparseable'})
            continue
        truth = []
        # Byte offsets avoid the Point wrapper, which crashes on this Windows
        # CPython 3.14/tree-sitter 0.26 combination when traversing large trees.
        line_starts = [0] + [match.end() for match in re.finditer(b'\n', source)]
        stack = [tree.root_node]
        while stack:
            node = stack.pop()
            if node.type in {'function_definition', 'method_declaration'}:
                name = node.child_by_field_name('name')
                if name is not None:
                    truth.append({'name': source[name.start_byte:name.end_byte].decode('utf-8'),
                                  'start': bisect.bisect_right(line_starts, node.start_byte),
                                  'name_line': bisect.bisect_right(line_starts, name.start_byte),
                                  'end': bisect.bisect_right(line_starts, node.end_byte),
                                  'bodyless': node.child_by_field_name('body') is None})
            stack.extend(reversed(node.named_children))
        truth.sort(key=lambda n: n['start'])
        result = subprocess.run([str(binary), '--inp', str(path), '--source', 'php', '--map'], capture_output=True, text=True, encoding='utf-8')
        if result.returncode and 'No functions found in file' not in result.stderr:
            raise RuntimeError((rel, result.stderr))
        candidate = [(n.strip(), int(s), int(e)) for n, s, e in re.findall(r'([^;\n]+): (\d+)-(\d+);', result.stdout)]
        counts['files'] += 1
        counts['truth'] += len(truth)
        counts['bodyless_truth'] += sum(n['bodyless'] for n in truth)
        counts['candidate'] += len(candidate)
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
    row = {'project': args.label, 'root': str(root),
           'sha': subprocess.check_output(git + ['rev-parse', 'HEAD'], text=True).strip(),
           'dirty': bool(subprocess.check_output(git + ['status', '--porcelain'], text=True).strip()),
           'funcfinder_sha': subprocess.check_output(['git', '-C', str(repo), 'rev-parse', 'HEAD'], text=True).strip(),
           'funcfinder_dirty': bool(subprocess.check_output(['git', '-C', str(repo), 'status', '--porcelain'], text=True).strip()),
           'python_version': platform.python_version(), 'tree_sitter_version': importlib.metadata.version('tree-sitter'),
           'php_grammar_version': importlib.metadata.version('tree-sitter-php'), 'binary': str(binary),
           'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(), **counts}
    row['recall'] = counts['found'] / counts['truth'] if counts['truth'] else None
    row['precision'] = counts['found'] / counts['candidate'] if counts['candidate'] else None
    row['correct_span_rate'] = counts['correct_end'] / counts['truth'] if counts['truth'] else None
    output = pathlib.Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps({'summary': row, 'findings': findings, 'files': artifacts}, indent=2), encoding='utf-8')
    print(json.dumps(row), flush=True)


if __name__ == '__main__':
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--root', required=True)
    ap.add_argument('--label', required=True)
    ap.add_argument('--binary', required=True)
    ap.add_argument('--output', required=True)
    ap.add_argument('--module-dir', help='Optional directory containing installed tree-sitter packages')
    evaluate(ap.parse_args())
