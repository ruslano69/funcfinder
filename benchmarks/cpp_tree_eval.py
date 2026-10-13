"""Score C++ functions and declarations against libclang, keeping only active source lines.

The reference is the clang front end (libclang), so macros such as FMT_CONSTEXPR are expanded
the way the compiler sees them. Active lines come from `g++ -E` with the same standard and
defines: a candidate inside a disabled #if branch is reported as inactive, not scored.
"""
import argparse
import collections
import hashlib
import importlib.metadata
import json
import os
import pathlib
import platform
import re
import subprocess

import clang.cindex as cindex


FUNCTION_KINDS = {cindex.CursorKind.FUNCTION_DECL, cindex.CursorKind.CXX_METHOD,
                  cindex.CursorKind.CONSTRUCTOR, cindex.CursorKind.DESTRUCTOR,
                  cindex.CursorKind.CONVERSION_FUNCTION, cindex.CursorKind.FUNCTION_TEMPLATE}


def active_lines(path, gxx, std, defines, includes):
    env = dict(os.environ)
    env['PATH'] = str(gxx.parent) + os.pathsep + env.get('PATH', '')
    command = [str(gxx), '-E', '-x', 'c++', '-std=' + std, *['-D' + d for d in defines],
               *['-I' + str(p) for p in includes], str(path)]
    result = subprocess.run(command, capture_output=True, env=env)
    if result.returncode:
        raise RuntimeError(result.stderr.decode('utf-8', errors='replace'))
    current_file, current_line, active = None, 0, set()
    for line in result.stdout.splitlines(keepends=True):
        marker = re.match(rb'#\s+(\d+)\s+("(?:[^"\\]|\\.)*")', line)
        if marker:
            current_line = int(marker[1])
            name = json.loads(marker[2].decode('utf-8'))
            current_file = pathlib.Path(name).resolve() if not name.startswith('<') else None
            continue
        if current_file == path and line.strip():
            active.add(current_line)
        current_line += 1
    return active


def reference_name(spelling):
    # clang spells a member of a class template with its arguments (basic_string_view<Char>);
    # funcfinder names the function. Operator symbols (operator==, operator()) are kept whole,
    # conversion functions (operator basic_format_args<T>) lose their arguments.
    if spelling.startswith('operator') and re.match(r'operator\s+[A-Za-z_]', spelling):
        return re.sub(r'<.*', '', spelling)
    if spelling.startswith('operator'):
        return spelling
    return spelling.split('<', 1)[0]


def terminator_lines(tu, path):
    # libclang ends a function template's extent before its trailing return type (`-> decltype(...)`),
    # so the declaration's end is also read from its tokens: the first ';' or the '}' closing its body.
    size = path.stat().st_size
    tokens = list(tu.get_tokens(extent=tu.get_extent(str(path), (0, size))))
    first = {}
    for i, tok in enumerate(tokens):
        first.setdefault((tok.location.line, tok.spelling), i)
    return tokens, first


def token_end(tokens, index):
    depth, braces = 0, 0
    for tok in tokens[index:]:
        s = tok.spelling
        if s in ('(', '['):
            depth += 1
        elif s in (')', ']'):
            depth -= 1
        elif depth == 0 and s == ';' and not braces:
            return tok.location.line
        elif depth == 0 and s == '{':
            braces += 1
        elif depth == 0 and s == '}' and braces:
            braces -= 1
            if braces == 0:
                return tok.location.line
    return None


def symbols(tu, path):
    tokens, first = terminator_lines(tu, path)
    truth, stack = [], list(tu.cursor.get_children())
    while stack:
        cursor = stack.pop()
        # Statement bodies hold local declarations such as `std::string s(name);`, which are not
        # function declarations; lambdas inside initializers are not reference functions either.
        if cursor.kind == cindex.CursorKind.COMPOUND_STMT:
            continue
        file = cursor.location.file
        if file is None or pathlib.Path(file.name).resolve() != path:
            continue
        if cursor.kind in FUNCTION_KINDS:
            end = cursor.extent.end.line
            index = first.get((cursor.location.line, cursor.spelling.split('<', 1)[0]))
            if index is not None:
                terminator = token_end(tokens, index)
                if terminator is not None:
                    end = max(end, terminator)
            truth.append({'name': reference_name(cursor.spelling), 'start': cursor.extent.start.line,
                          'name_line': cursor.location.line, 'end': end,
                          'bodyless': not cursor.is_definition()})
            continue
        stack.extend(cursor.get_children())
    return sorted(truth, key=lambda n: n['start'])


def evaluate(args):
    root, binary, gxx = (pathlib.Path(p).resolve() for p in (args.root, args.binary, args.gxx))
    repo = pathlib.Path(__file__).resolve().parent.parent
    git = ['git', '-c', f'safe.directory={root.as_posix()}', '-C', str(root)]
    spec = ['--', *args.pathspec] if args.pathspec else []
    paths = [root / p for p in subprocess.check_output(git + ['ls-files', '-z', *spec], text=True).split('\0')
             if p.endswith(('.cc', '.cpp', '.cxx', '.hpp', '.h'))]
    clang_args = ['-x', 'c++', '-std=' + args.std, '-target', args.target,
                  *['-I' + str(p) for p in args.include_dir], *[x for d in args.system_include for x in ('-isystem', d)],
                  *['-D' + d for d in args.define]]
    index = cindex.Index.create()
    counts = collections.Counter({k: 0 for k in ['files', 'unparseable', 'preprocess_failed', 'truth',
        'bodyless_truth', 'candidate', 'inactive_candidate', 'found', 'missing', 'false_positive',
        'correct_end', 'wrong_end']})
    findings, artifacts = [], []
    for path in sorted(paths):
        rel = path.relative_to(root).as_posix()
        try:
            active = active_lines(path, gxx, args.std, args.define, args.include_dir)
        except RuntimeError as exc:
            counts['preprocess_failed'] += 1
            findings.append({'file': rel, 'kind': 'preprocess_failed', 'error': str(exc)})
            continue
        tu = index.parse(str(path), args=clang_args)
        errors = [d for d in tu.diagnostics if d.severity >= cindex.Diagnostic.Error]
        if errors:
            counts['unparseable'] += 1
            findings.append({'file': rel, 'kind': 'unparseable', 'error': errors[0].spelling,
                             'line': errors[0].location.line})
            continue
        truth = symbols(tu, path)
        result = subprocess.run([str(binary), '--inp', str(path), '--source', 'cpp', '--map'],
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
    git_head = subprocess.check_output(git + ['rev-parse', 'HEAD'], text=True).strip()
    row = {'project': args.label, 'root': str(root), 'tracked_files': len(paths), 'sha': git_head,
        'dirty': bool(subprocess.check_output(git + ['status', '--porcelain'], text=True).strip()),
        'pathspec': args.pathspec,
        'funcfinder_sha': subprocess.check_output(['git', '-C', str(repo), 'rev-parse', 'HEAD'], text=True).strip(),
        'funcfinder_dirty': bool(subprocess.check_output(['git', '-C', str(repo), 'status', '--porcelain'], text=True).strip()),
        'python_version': platform.python_version(), 'libclang_version': importlib.metadata.version('libclang'),
        'gxx': str(gxx), 'gxx_version': subprocess.check_output([str(gxx), '--version'], text=True).splitlines()[0],
        'gxx_sha256': hashlib.sha256(gxx.read_bytes()).hexdigest(), 'standard': args.std, 'target': args.target,
        'defines': args.define, 'include_dirs': args.include_dir, 'system_include_dirs': args.system_include,
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
    for flag in ['root', 'label', 'binary', 'output', 'gxx']:
        p.add_argument('--' + flag, required=True)
    p.add_argument('--std', default='c++17')
    p.add_argument('--target', default='x86_64-w64-mingw32')
    p.add_argument('--define', action='append', default=[])
    p.add_argument('--include-dir', action='append', default=[])
    p.add_argument('--system-include', action='append', default=[])
    p.add_argument('--pathspec', action='append', default=[], help='tracked paths to measure, e.g. include src')
    evaluate(p.parse_args())
