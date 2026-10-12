"""Score tracked Scala/D named functions and declarations against tree-sitter."""
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


def symbols(tree, source, language):
    starts = [0] + [m.end() for m in re.finditer(b'\n', source)]
    truth = []
    stack = [tree.root_node]
    while stack:
        node = stack.pop()
        name = None
        bodyless = False
        label = None
        if language == 'scala' and node.type in {'function_definition', 'function_declaration'}:
            name = node.child_by_field_name('name')
            bodyless = node.type == 'function_declaration'
        elif language == 'd' and node.type in {'function_declaration', 'constructor', 'destructor', 'postblit'}:
            if node.type == 'function_declaration':
                name = next((c for c in node.named_children if c.type == 'identifier'), None)
            else:
                name = next((c for c in node.children if c.type == 'this'), None)
                label = '~this' if node.type == 'destructor' else 'this'
            body = next((c for c in node.named_children if c.type == 'function_body'), None)
            bodyless = body is None or source[body.start_byte:body.end_byte].strip() == b';'
        if name is not None:
            label = label or source[name.start_byte:name.end_byte].decode('utf-8').strip('`')
            truth.append({'name': label, 'start': bisect.bisect_right(starts, node.start_byte),
                          'name_line': bisect.bisect_right(starts, name.start_byte),
                          'end': bisect.bisect_right(starts, significant_end(node)),
                          'bodyless': bodyless, 'node_type': node.type})
        stack.extend(reversed(node.named_children))
    return sorted(truth, key=lambda n: (n['start'], n['name_line']))


def significant_end(node):
    """Exclude syntax-tree extras and whitespace attached to indented bodies."""
    stack = [node]
    while stack:
        current = stack.pop()
        if current.is_extra or current.type in {'comment', 'line_comment', 'block_comment'}:
            continue
        children = current.children
        if not children and current.end_byte > current.start_byte:
            return current.end_byte
        stack.extend(children)
    return node.end_byte


def evaluate(args):
    sys.path.insert(0, str(pathlib.Path(args.module_dir).resolve()))
    if args.language == 'scala':
        from tree_sitter import Language, Parser
        import tree_sitter_scala
        parser = Parser(Language(tree_sitter_scala.language()))
        grammar = {'grammar_package': 'tree-sitter-scala', 'grammar_version': importlib.metadata.version('tree-sitter-scala')}
    else:
        import tree_sitter_language_pack as pack
        cache = pathlib.Path(args.grammar_cache).resolve()
        pack.configure(pack.PackConfig(cache_dir=str(cache), languages=['d']))
        parser = pack.get_parser('d')
        dll = next(cache.rglob('tree_sitter_d.dll'), None)
        grammar = {'grammar_package': 'tree-sitter-language-pack', 'grammar_version': importlib.metadata.version('tree-sitter-language-pack'),
                   'grammar_binary_sha256': hashlib.sha256(dll.read_bytes()).hexdigest() if dll else None}
    root = pathlib.Path(args.root).resolve()
    binary = pathlib.Path(args.binary).resolve()
    repo = pathlib.Path(__file__).resolve().parent.parent
    git = ['git', '-c', f'safe.directory={root.as_posix()}', '-C', str(root)]
    suffix = '.scala' if args.language == 'scala' else '.d'
    paths = [root / p for p in subprocess.check_output(git + ['ls-files', '-z'], text=True).split('\0') if p.endswith(suffix)]
    counts = collections.Counter({k: 0 for k in ['files', 'unparseable', 'truth', 'bodyless_truth', 'candidate', 'found', 'missing', 'false_positive', 'correct_end', 'wrong_end']})
    findings, artifacts = [], []
    for index, path in enumerate(sorted(paths)):
        source = path.read_bytes()
        rel = path.relative_to(root).as_posix()
        tree = parser.parse(source)
        if tree.root_node.has_error:
            counts['unparseable'] += 1
            findings.append({'file': rel, 'kind': 'unparseable'})
            continue
        truth = symbols(tree, source, args.language)
        result = subprocess.run([str(binary), '--inp', str(path), '--source', args.language, '--map'], capture_output=True, text=True, encoding='utf-8')
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
    row = {'project': args.label, 'language': args.language, 'root': str(root), 'tracked_files': len(paths),
           'sha': subprocess.check_output(git + ['rev-parse', 'HEAD'], text=True).strip(),
           'dirty': bool(subprocess.check_output(git + ['status', '--porcelain'], text=True).strip()),
           'funcfinder_sha': subprocess.check_output(['git', '-C', str(repo), 'rev-parse', 'HEAD'], text=True).strip(),
           'funcfinder_dirty': bool(subprocess.check_output(['git', '-C', str(repo), 'status', '--porcelain'], text=True).strip()),
           'python_version': platform.python_version(), 'tree_sitter_version': importlib.metadata.version('tree-sitter'),
           **grammar, 'binary': str(binary), 'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(), **counts}
    row['recall'] = counts['found'] / counts['truth'] if counts['truth'] else None
    row['precision'] = counts['found'] / counts['candidate'] if counts['candidate'] else None
    row['correct_span_rate'] = counts['correct_end'] / counts['truth'] if counts['truth'] else None
    output = pathlib.Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps({'summary': row, 'findings': findings, 'files': artifacts}, indent=2), encoding='utf-8')
    print(json.dumps(row), flush=True)


if __name__ == '__main__':
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--language', required=True, choices=['scala', 'd'])
    ap.add_argument('--root', required=True)
    ap.add_argument('--label', required=True)
    ap.add_argument('--binary', required=True)
    ap.add_argument('--output', required=True)
    ap.add_argument('--module-dir', required=True)
    ap.add_argument('--grammar-cache', default='build/d-grammar-cache')
    evaluate(ap.parse_args())
