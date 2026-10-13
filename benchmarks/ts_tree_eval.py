"""Score TypeScript functions and declarations against tree-sitter TypeScript.

Truth: function declarations and signatures (overloads, `declare function`), class methods, constructors,
abstract methods, class and interface method signatures, and functions bound to a variable (`const f = () => ...`,
`const g = function () {...}`). Anonymous callbacks and class property arrows are not truth.
Computed property names are not truth either and are counted apart.
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
import tree_sitter_typescript

BODYLESS = {'function_signature', 'abstract_method_signature', 'method_signature'}
FUNCTION_NODES = {'function_declaration', 'generator_function_declaration', 'function_signature',
                  'method_definition', 'abstract_method_signature', 'method_signature'}


def name_of(code, node):
    name = node.child_by_field_name('name')
    if name is None:
        return None, 'anonymous'
    if name.type in ('identifier', 'property_identifier'):
        return code[name.start_byte:name.end_byte].decode('utf-8'), None
    if name.type == 'string':
        # funcfinder keeps the quotes of a string-named method, as the syntax tree has it
        return code[name.start_byte:name.end_byte].decode('utf-8'), None
    return None, 'computed'


def is_member_signature(node):
    # an overload or interface signature is a member; a method signature in a type literal is not a function
    parent = node.parent
    return parent is not None and parent.type in ('class_body', 'interface_body')


def symbols(tree, code):
    starts = [0] + [m.end() for m in re.finditer(b'\n', code)]
    truth, excluded = [], collections.Counter()
    stack = [tree.root_node]
    while stack:
        node = stack.pop()
        kind = node.type
        if kind in FUNCTION_NODES and (kind != 'method_signature' or is_member_signature(node)):
            name, reason = name_of(code, node)
            if reason:
                excluded[reason] += 1
            elif name is not None:
                truth.append(entry(node, name, kind in BODYLESS))
        elif kind == 'variable_declarator':
            value = node.child_by_field_name('value')
            name_node = node.child_by_field_name('name')
            if value is not None and value.type in ('arrow_function', 'function_expression', 'function') \
                    and name_node is not None and name_node.type == 'identifier':
                truth.append(entry(node, code[name_node.start_byte:name_node.end_byte].decode('utf-8'), False,
                                   start_node=node))
        stack.extend(reversed(node.children))
    return sorted(truth, key=lambda n: n['start']), excluded


def entry(node, name, bodyless, start_node=None):
    # rows are 0-based; the report is 1-based
    anchor = start_node or node
    return {'name': name, 'start': anchor.start_point[0] + 1, 'name_line': anchor.start_point[0] + 1,
            'end': node.end_point[0] + 1, 'bodyless': bodyless}


def evaluate(args):
    root, binary = (pathlib.Path(p).resolve() for p in (args.root, args.binary))
    repo = pathlib.Path(__file__).resolve().parent.parent
    git = ['git', '-c', f'safe.directory={root.as_posix()}', '-C', str(root)]
    spec = ['--', *args.pathspec] if args.pathspec else []
    paths = [root / p for p in subprocess.check_output(git + ['ls-files', '-z', *spec], text=True).split('\0')
             if p.endswith(('.ts', '.tsx'))]
    parser = Parser(Language(tree_sitter_typescript.language_typescript()))
    tsx_parser = Parser(Language(tree_sitter_typescript.language_tsx()))
    counts = collections.Counter({k: 0 for k in ['files', 'unparseable', 'truth', 'bodyless_truth',
        'candidate', 'found', 'missing', 'false_positive', 'correct_end', 'wrong_end', 'computed_excluded',
        'anonymous_excluded']})
    findings, artifacts = [], []
    for path in sorted(paths):
        rel = path.relative_to(root).as_posix()
        code = path.read_bytes()
        tree = (tsx_parser if path.suffix == '.tsx' else parser).parse(code)
        if tree.root_node.has_error:
            counts['unparseable'] += 1
            findings.append({'file': rel, 'kind': 'unparseable'})
            continue
        truth, excluded = symbols(tree, code)
        counts['computed_excluded'] += excluded['computed']
        counts['anonymous_excluded'] += excluded['anonymous']
        result = subprocess.run([str(binary), '--inp', str(path), '--source', 'ts', '--map'],
                                capture_output=True, text=True, encoding='utf-8')
        if result.returncode and 'No functions found in file' not in result.stderr:
            raise RuntimeError((rel, result.stderr))
        candidate = [(name.strip(), int(start), int(end))
                     for name, start, end in re.findall(r'([^;\n]+): (\d+)-(\d+);', result.stdout)]
        counts.update(files=1, truth=len(truth), bodyless_truth=sum(n['bodyless'] for n in truth),
                      candidate=len(candidate))
        used = set()
        for node in truth:
            matches = [(i, f) for i, f in enumerate(candidate) if i not in used and f[0] == node['name']
                       and node['start'] <= f[1] <= node['name_line']]
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
    git_head = subprocess.check_output(git + ['rev-parse', 'HEAD'], text=True).strip()
    row = {'project': args.label, 'root': str(root), 'tracked_files': len(paths), 'sha': git_head,
        'dirty': bool(subprocess.check_output(git + ['status', '--porcelain'], text=True).strip()),
        'pathspec': args.pathspec,
        'funcfinder_sha': subprocess.check_output(['git', '-C', str(repo), 'rev-parse', 'HEAD'], text=True).strip(),
        'funcfinder_dirty': bool(subprocess.check_output(['git', '-C', str(repo), 'status', '--porcelain'], text=True).strip()),
        'python_version': platform.python_version(), 'tree_sitter_version': importlib.metadata.version('tree-sitter'),
        'grammar_version': importlib.metadata.version('tree-sitter-typescript'),
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
    for flag in ['root', 'label', 'binary', 'output']:
        p.add_argument('--' + flag, required=True)
    p.add_argument('--pathspec', action='append', default=[], help='tracked paths to measure')
    evaluate(p.parse_args())
