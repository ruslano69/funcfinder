"""Score C definitions/prototypes after GCC preprocessing, retaining source lines."""
import argparse
import bisect
import collections
import hashlib
import importlib.metadata
import json
import os
import pathlib
import platform
import re
import subprocess
import sys


def preprocess(path, gcc, defines, includes, headers):
    env = dict(os.environ)
    env['PATH'] = str(gcc.parent) + os.pathsep + env.get('PATH', '')
    command = [str(gcc), '-E', '-x', 'c', '-std=c99', *['-D' + d for d in defines],
               *['-I' + str(p) for p in includes],
               *[part for header in headers for part in ('-include', str(header))], str(path)]
    result = subprocess.run(command, capture_output=True, env=env)
    if result.returncode:
        raise RuntimeError(result.stderr.decode('utf-8', errors='replace'))
    current_file, current_line = None, 0
    code, origins, active = [], [], set()
    for line in result.stdout.splitlines(keepends=True):
        marker = re.match(rb'#\s+(\d+)\s+("(?:[^"\\]|\\.)*")', line)
        if marker:
            current_line = int(marker[1])
            name = json.loads(marker[2].decode('utf-8'))
            current_file = pathlib.Path(name).resolve() if not name.startswith('<') else None
            continue
        if current_file == path:
            code.append(line if line.endswith(b'\n') else line + b'\n')
            origins.append(current_line)
            if line.strip():
                active.add(current_line)
        current_line += 1
    return b''.join(code), origins, active


def normalize_builtins(code):
    # GCC's va_arg/offsetof accept a type operand that this C grammar reads as
    # an expression. Mask only these expressions, preserving bytes/newlines.
    masked, count = bytearray(code), 0
    for match in re.finditer(rb'\b__builtin_(?:va_arg|offsetof)\s*\(', code):
        depth, end = 1, match.end()
        while end < len(code) and depth:
            if code[end] == ord('('):
                depth += 1
            elif code[end] == ord(')'):
                depth -= 1
            end += 1
        if depth:
            raise RuntimeError('Unclosed GCC builtin expression')
        for i in range(match.start(), end):
            if code[i] not in (10, 13):
                masked[i] = 32
        masked[match.start()] = ord('0')
        count += 1
    return bytes(masked), count


def function_name(declarator):
    chain = []
    node = declarator
    while node is not None:
        chain.append(node.type)
        if node.type == 'identifier':
            # A pointer between the identifier and its first function declarator
            # denotes a function-pointer variable, not a function declaration.
            nearest = next((t for t in reversed(chain[:-1]) if t in
                            {'function_declarator', 'pointer_declarator', 'array_declarator'}), None)
            return node if nearest == 'function_declarator' else None
        child = node.child_by_field_name('declarator')
        if child is None and node.type == 'parenthesized_declarator':
            child = next((c for c in node.named_children if c.type != 'comment'), None)
        node = child
    return None


def symbols(tree, code, origins):
    starts = [0] + [m.end() for m in re.finditer(b'\n', code)]
    def line(offset):
        return origins[bisect.bisect_right(starts, offset) - 1]
    truth, stack = [], [tree.root_node]
    while stack:
        node = stack.pop()
        if node.type in {'function_definition', 'declaration'}:
            for declarator in node.children_by_field_name('declarator'):
                name = function_name(declarator)
                if name is not None:
                    truth.append({'name': code[name.start_byte:name.end_byte].decode('utf-8'),
                                  'start': line(node.start_byte), 'name_line': line(name.start_byte),
                                  'end': line(node.end_byte - 1),
                                  'bodyless': node.type == 'declaration'})
        stack.extend(reversed(node.named_children))
    return sorted(truth, key=lambda n: n['start'])


def evaluate(args):
    sys.path.insert(0, str(pathlib.Path(args.module_dir).resolve()))
    from tree_sitter import Language, Parser
    import tree_sitter_c
    parser = Parser(Language(tree_sitter_c.language()))
    root, binary, gcc = (pathlib.Path(p).resolve() for p in (args.root, args.binary, args.gcc))
    repo = pathlib.Path(__file__).resolve().parent.parent
    git = ['git', '-c', f'safe.directory={root.as_posix()}', '-C', str(root)]
    paths = [root / p for p in subprocess.check_output(git + ['ls-files', '-z'], text=True).split('\0')
             if p.endswith(('.c', '.h'))]
    counts = collections.Counter({k: 0 for k in ['files', 'unparseable', 'preprocess_failed', 'truth',
        'bodyless_truth', 'candidate', 'inactive_candidate', 'found', 'missing', 'false_positive',
        'correct_end', 'wrong_end', 'normalized_builtin_expressions']})
    findings, artifacts = [], []
    for path in sorted(paths):
        rel = path.relative_to(root).as_posix()
        try:
            code, origins, active = preprocess(path, gcc, args.define, args.include_dir, args.include_header)
            code, normalized = normalize_builtins(code)
            counts['normalized_builtin_expressions'] += normalized
        except RuntimeError as exc:
            counts['preprocess_failed'] += 1
            findings.append({'file': rel, 'kind': 'preprocess_failed', 'error': str(exc)})
            continue
        tree = parser.parse(code)
        if tree.root_node.has_error:
            counts['unparseable'] += 1
            findings.append({'file': rel, 'kind': 'unparseable'})
            continue
        truth = symbols(tree, code, origins)
        result = subprocess.run([str(binary), '--inp', str(path), '--source', 'c', '--map'],
                                capture_output=True, text=True, encoding='utf-8')
        if result.returncode and 'No functions found in file' not in result.stderr:
            raise RuntimeError((rel, result.stderr))
        candidate = []
        for name, start, end in re.findall(r'([^;\n]+): (\d+)-(\d+);', result.stdout):
            item = (name.strip(), int(start), int(end))
            if item[1] not in active:
                counts['inactive_candidate'] += 1
                findings.append({'file': rel, 'kind': 'inactive_candidate', 'name': item[0], 'start': item[1], 'end': item[2]})
            else:
                candidate.append(item)
        counts.update(files=1, truth=len(truth), bodyless_truth=sum(n['bodyless'] for n in truth), candidate=len(candidate))
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
    row = {'project': args.label, 'root': str(root), 'tracked_files': len(paths),
        'sha': subprocess.check_output(git + ['rev-parse', 'HEAD'], text=True).strip(),
        'dirty': bool(subprocess.check_output(git + ['status', '--porcelain'], text=True).strip()),
        'funcfinder_sha': subprocess.check_output(['git', '-C', str(repo), 'rev-parse', 'HEAD'], text=True).strip(),
        'funcfinder_dirty': bool(subprocess.check_output(['git', '-C', str(repo), 'status', '--porcelain'], text=True).strip()),
        'python_version': platform.python_version(), 'tree_sitter_version': importlib.metadata.version('tree-sitter'),
        'grammar_version': importlib.metadata.version('tree-sitter-c'),
        'gcc': str(gcc), 'gcc_version': subprocess.check_output([str(gcc), '--version'], text=True).splitlines()[0],
        'gcc_sha256': hashlib.sha256(gcc.read_bytes()).hexdigest(), 'defines': args.define,
        'include_dirs': args.include_dir, 'include_headers': args.include_header, 'standard': 'c99',
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
    for flag in ['root', 'label', 'binary', 'output', 'module-dir', 'gcc']:
        p.add_argument('--' + flag, required=True)
    p.add_argument('--define', action='append', default=[])
    p.add_argument('--include-dir', action='append', default=[])
    p.add_argument('--include-header', action='append', default=[])
    evaluate(p.parse_args())
