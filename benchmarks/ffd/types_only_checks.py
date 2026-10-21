"""Compare JS/TS types-only output with a frozen D build and independent AST names/lines."""
import argparse
import json
import pathlib
import subprocess
import sys
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--before', required=True)
    ap.add_argument('--after', required=True)
    ap.add_argument('--audit', required=True, help='unittest binary with FFDTypeAudit/FFDScanStats')
    ap.add_argument('--runtime', required=True, help='tree-sitter 0.25.2, JS 0.25.0, TS 0.23.2')
    args = ap.parse_args()
    before, after, audit = [str(pathlib.Path(p).resolve()) for p in (args.before, args.after, args.audit)]
    sys.path.insert(0, str(pathlib.Path(args.runtime).resolve()))
    from tree_sitter import Language, Parser
    import tree_sitter_javascript
    import tree_sitter_typescript

    checks = 0
    build = pathlib.Path(__file__).resolve().parents[2] / 'build'
    with tempfile.TemporaryDirectory(prefix='.types-only-', dir=build) as folder:
        root = pathlib.Path(folder)
        assert root.resolve().parent == build.resolve()
        for lang in ('js', 'ts'):
            directory = root / lang
            directory.mkdir()
            source = ('function outer() {\n class Local {}\n}\n'
                      'class C {\n method() { return `class Ghost {}`; }\n}\n'
                      '// class Comment {}\n')
            if lang == 'ts':
                source += 'interface I { value: string; }\ntype Alias = string | number;\nenum E { A, B }\n'
            path = directory / ('source с пробелами.' + lang)
            path.write_bytes(source.replace('\n', '\r\n').encode())
            (directory / ('empty.' + lang)).write_text('// no declarations', encoding='utf-8')
            language = tree_sitter_javascript.language() if lang == 'js' else tree_sitter_typescript.language_typescript()
            tree = Parser(Language(language)).parse(source.encode())
            assert not tree.root_node.has_error

            def walk(node):
                yield node
                for child in node.children:
                    yield from walk(child)

            expected = []
            for node in walk(tree.root_node):
                if node.type in ('class_declaration', 'interface_declaration', 'type_alias_declaration', 'enum_declaration'):
                    name = node.child_by_field_name('name')
                    expected.append((source.encode()[name.start_byte:name.end_byte].decode(), node.start_point.row + 1))

            for workers in (1, 8):
                for flags in ([], ['--all'], ['--struct']):
                    for output in (['--json'], ['--names'], ['--tree'], ['--tree-full']):
                        base = ['--dir', str(directory), '--workers', str(workers), *flags, *output]
                        old, new = [subprocess.run([exe, *base], capture_output=True) for exe in (before, after)]
                        assert old.returncode == new.returncode == 0 and old.stdout == new.stdout, base
                        checks += 1
                        if flags == ['--struct'] and output == ['--json']:
                            doc = json.loads(new.stdout)
                            found = [(t['name'], t['line']) for f in doc['files'] for t in f['classes']]
                            assert found == expected, (lang, found, expected)
                            checks += 1
                base = ['--dir', str(directory), '--struct', '--json', '--timing', '--workers', str(workers)]
                result = subprocess.run([audit, '--DRT-testmode=run-main', *base], capture_output=True)
                assert result.returncode == 0 and b'native_calls=0' in result.stderr and b'function_bounds=0' in result.stderr, result.stderr
                checks += 1
            # File field ownership and extraction retain the original full scan.
            for flags in (['--struct', '--map', '--json'], ['--struct', '--extract'], ['--all', '--json'], ['--doc', 'C,Local']):
                base = ['--inp', str(path), '--source', lang, *flags]
                old, new = [subprocess.run([exe, *base], capture_output=True) for exe in (before, after)]
                assert old.returncode == new.returncode == 0 and old.stdout == new.stdout, base
                checks += 1
    print(f'{checks} types-only baseline/AST/audit checks passed')


if __name__ == '__main__':
    main()
