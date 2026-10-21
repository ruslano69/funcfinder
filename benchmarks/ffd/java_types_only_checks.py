"""Java type requests: frozen CLI, complete bounds audit and independent grammar fixture truth."""
import argparse
import json
import pathlib
import subprocess
import sys
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    for name in ('before', 'after', 'audit', 'runtime'):
        ap.add_argument('--' + name, required=True)
    args = ap.parse_args()
    sys.path.insert(0, str(pathlib.Path(args.runtime).resolve()))
    from tree_sitter import Language, Parser
    import tree_sitter_java
    parser = Parser(Language(tree_sitter_java.language()))
    before, after, audit = [str(pathlib.Path(p).resolve()) for p in (args.before, args.after, args.audit)]
    build = pathlib.Path(__file__).resolve().parents[2] / 'build'
    checks = 0

    def same(base):
        nonlocal checks
        old, new = [subprocess.run([exe, *base], capture_output=True) for exe in (before, after)]
        assert old.returncode == new.returncode == 0 and old.stdout == new.stdout, base
        checks += 1
        return new.stdout

    def audited(base, expected):
        nonlocal checks
        result = subprocess.run([audit, '--DRT-testmode=run-main', *base], capture_output=True)
        assert result.returncode == 0 and result.stdout == expected, result.stderr
        checks += 1

    with tempfile.TemporaryDirectory(prefix='.java-types-', dir=build) as folder:
        root = pathlib.Path(folder)
        source = ('@interface A { int[] values() default {1,2}; }\n'
                  '@A(values={1,2})\nrecord R(\n @A(values={1,2}) int value\n) { R {} }\n'
                  'class C {\n void outer() {\n class Local { void inner() {} }\n'
                  ' Object value = new Object() { class Inside {} };\n }\n'
                  ' String text = "class Fake {}";\n}\n'
                  'interface I { void method(); }\nenum E { FIRST { void method() {} }, SECOND; }\n')
        path = root / 'source с пробелами.java'
        path.write_bytes(source.replace('\n', '\r\n').encode())
        tree = parser.parse(source.encode())
        assert not tree.root_node.has_error
        expected = []
        stack = [tree.root_node]
        while stack:
            node = stack.pop()
            if node.type in ('class_declaration', 'interface_declaration', 'enum_declaration', 'record_declaration', 'annotation_type_declaration'):
                name = node.child_by_field_name('name')
                keyword = next((c for c in node.children if c.type in ('class', 'interface', 'enum', 'record', '@interface', '@')), node)
                expected.append((source.encode()[name.start_byte:name.end_byte].decode(), keyword.start_point.row + 1))
            stack.extend(reversed(node.named_children))
        (root / 'empty.java').write_text('// empty', encoding='utf-8')
        for workers in (1, 8):
            for mode in ([], ['--all'], ['--struct']):
                for output in (['--json'], ['--names'], ['--tree'], ['--tree-full']):
                    base = ['--dir', str(root), '--workers', str(workers), *mode, *output]
                    result = same(base)
                    if mode == ['--struct']:
                        audited(base, result)
                        if output == ['--json']:
                            found = [(t['name'], t['line']) for f in json.loads(result)['files'] for t in f['classes']]
                            assert found == expected, (found, expected)
                            checks += 1
        for flags in (['--struct', '--map', '--json'], ['--struct', '--extract'], ['--all', '--json'], ['--doc', 'C,Local']):
            same(['--inp', str(path), '--source', 'java', *flags])
        stress = root / 'stress'
        stress.mkdir()
        cases = {'long': 'class C { String text = "' + 'x' * 131072 + '"; }\n',
                 'nested': 'class C { void method() { ' + '{ ' * 64 + 'class Deep {}' + '} ' * 64 + '} }\n',
                 'unicode': 'class Видимый { int поле; void $method() {} }\n',
                 'malformed': 'class Open { void broken(\n'}
        for name, text in cases.items():
            (stress / (name + '.java')).write_bytes(text.replace('\n', '\r\n').encode())
        (stress / 'invalid.java').write_bytes(b'// \xff\xc0\xaf\xed\xa0\x80\nclass Valid {}\n')
        for workers in (1, 8):
            base = ['--dir', str(stress), '--workers', str(workers), '--struct', '--json']
            audited(base, same(base))
    print(f'{checks} Java types-only baseline/AST/audit checks passed')


if __name__ == '__main__':
    main()
