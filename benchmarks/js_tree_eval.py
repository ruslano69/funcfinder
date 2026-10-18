"""Score named JavaScript functions against pinned tree-sitter JavaScript.

Truth includes named declarations/expressions, generators, literal methods and
anonymous function/arrow values directly bound to an identifier variable.
Named expressions use their internal name, not a second alias entry. Anonymous
callbacks, assignment/property function values and computed methods are outside
this profile. Retain duplicate names: file-mode JSON would overwrite them.
"""
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

from tree_sitter import Language, Parser
import tree_sitter_javascript


def symbols(tree, code):
    lines = [0] + [m.end() for m in re.finditer(b'\n', code)]
    truth, excluded = [], collections.Counter()
    stack = [tree.root_node]
    while stack:
        node = stack.pop()
        kind = node.type
        name, anchor = None, node
        if kind in ('function_declaration', 'generator_function_declaration',
                    'function_expression', 'generator_function'):
            name = node.child_by_field_name('name')
            if name is None:
                parent = node.parent
                if parent and parent.type == 'variable_declarator' and parent.child_by_field_name('value') == node:
                    name = parent.child_by_field_name('name')
                    anchor = parent
                else:
                    excluded['anonymous'] += 1
        elif kind == 'arrow_function':
            parent = node.parent
            if parent and parent.type == 'variable_declarator' and parent.child_by_field_name('value') == node:
                name = parent.child_by_field_name('name')
                anchor = parent
            else:
                excluded['anonymous'] += 1
        elif kind == 'method_definition':
            name = node.child_by_field_name('name')
        if name is not None:
            if name.type not in ('identifier', 'property_identifier', 'private_property_identifier', 'string', 'number'):
                excluded['computed'] += 1
            else:
                truth.append({'name': code[name.start_byte:name.end_byte].decode('utf-8'),
                    'start': bisect.bisect_right(lines, anchor.start_byte),
                    'name_line': bisect.bisect_right(lines, name.start_byte),
                    'end': bisect.bisect_right(lines, node.end_byte-1),
                    'start_byte': anchor.start_byte, 'end_byte': node.end_byte, 'node_type': kind})
        stack.extend(reversed(node.named_children))
    return sorted(truth, key=lambda n: n['start_byte']), excluded


def evaluate(args):
    root, binary = pathlib.Path(args.root).resolve(), pathlib.Path(args.binary).resolve()
    repo = pathlib.Path(__file__).resolve().parent.parent
    git = ['git', '-C', str(root)]
    spec = ['--', *args.pathspec] if args.pathspec else []
    paths = [root / p for p in subprocess.check_output(git + ['ls-files', '-z', *spec], text=True, encoding='utf-8').split('\0')
             if p.endswith(('.js', '.jsx', '.mjs', '.cjs'))]
    parser = Parser(Language(tree_sitter_javascript.language()))
    counts = collections.Counter({k: 0 for k in ('files', 'unparseable', 'truth', 'candidate', 'found',
        'missing', 'false_positive', 'correct_end', 'wrong_end', 'computed_excluded', 'anonymous_excluded')})
    findings, artifacts = [], []
    for path in sorted(paths):
        relative = path.relative_to(root).as_posix()
        code = path.read_bytes()
        tree = parser.parse(code)
        if tree.root_node.has_error:
            counts['unparseable'] += 1
            findings.append({'file': relative, 'kind': 'unparseable'})
            continue
        truth, excluded = symbols(tree, code)
        counts['computed_excluded'] += excluded['computed']
        counts['anonymous_excluded'] += excluded['anonymous']
        result = subprocess.run([str(binary), '--inp', str(path), '--source', 'js', '--map'],
            capture_output=True, text=True, encoding='utf-8', timeout=120)
        if result.returncode and 'No functions found in file' not in result.stderr:
            raise RuntimeError((relative, result.stderr))
        candidates = [(name.strip(), int(start), int(end))
                      for name, start, end in re.findall(r'([^;\n]+): (\d+)-(\d+);', result.stdout)]
        counts.update(files=1, truth=len(truth), candidate=len(candidates))
        used = set()
        for node in truth:
            matches = [(i, f) for i, f in enumerate(candidates) if i not in used and f[0] == node['name']
                       and node['start'] <= f[1] <= node['name_line']]
            if not matches:
                counts['missing'] += 1
                findings.append({'file': relative, 'kind': 'missing', **node})
                continue
            index, (_, start, end) = min(matches, key=lambda p: abs(p[1][1]-node['start']))
            used.add(index)
            counts['found'] += 1
            if end == node['end']:
                counts['correct_end'] += 1
            else:
                counts['wrong_end'] += 1
                findings.append({'file': relative, 'kind': 'wrong_end', **node, 'ff_start': start, 'ff_end': end})
        for i, (name, start, end) in enumerate(candidates):
            if i not in used:
                counts['false_positive'] += 1
                findings.append({'file': relative, 'kind': 'false_positive', 'name': name, 'start': start, 'end': end})
        artifacts.append({'file': relative, 'truth': truth, 'candidate': candidates})
    summary = {'project': args.label, 'root': str(root), 'tracked_files': len(paths),
        'sha': subprocess.check_output(git + ['rev-parse', 'HEAD'], text=True, encoding='utf-8').strip(),
        'dirty': bool(subprocess.check_output(git + ['status', '--porcelain'], text=True, encoding='utf-8').strip()),
        'pathspec': args.pathspec,
        'funcfinder_sha': subprocess.check_output(['git', '-C', str(repo), 'rev-parse', 'HEAD'], text=True, encoding='utf-8').strip(),
        'funcfinder_dirty': bool(subprocess.check_output(['git', '-C', str(repo), 'status', '--porcelain'], text=True, encoding='utf-8').strip()),
        'python_version': platform.python_version(), 'tree_sitter_version': importlib.metadata.version('tree-sitter'),
        'grammar_version': importlib.metadata.version('tree-sitter-javascript'),
        'binary': str(binary), 'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(), **counts}
    summary['recall'] = counts['found']/counts['truth'] if counts['truth'] else None
    summary['precision'] = counts['found']/counts['candidate'] if counts['candidate'] else None
    summary['correct_span_rate'] = counts['correct_end']/counts['truth'] if counts['truth'] else None
    output = pathlib.Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps({'summary': summary, 'findings': findings, 'files': artifacts}, indent=2), encoding='utf-8')
    print(json.dumps(summary), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for flag in ('root', 'label', 'binary', 'output'):
        parser.add_argument('--'+flag, required=True)
    parser.add_argument('--pathspec', action='append', default=[], help='tracked paths to score; repeat')
    evaluate(parser.parse_args())
