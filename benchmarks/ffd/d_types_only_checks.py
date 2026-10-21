"""D types-only regressions: frozen CLI, original bounds audit, independent fixture grammar."""
import argparse
import json
import pathlib
import subprocess
import sys
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    for name in ('before', 'after', 'audit', 'runtime', 'grammar-cache'):
        ap.add_argument('--' + name, required=True)
    args = ap.parse_args()
    sys.path.insert(0, str(pathlib.Path(args.runtime).resolve()))
    import tree_sitter_language_pack as pack
    pack.configure(pack.PackConfig(cache_dir=str(pathlib.Path(args.grammar_cache).resolve()), languages=['d']))
    parser = pack.get_parser('d')
    executables = [str(pathlib.Path(p).resolve()) for p in (args.before, args.after)]
    audit = str(pathlib.Path(args.audit).resolve())
    build = pathlib.Path(__file__).resolve().parents[2] / 'build'
    checks = 0
    with tempfile.TemporaryDirectory(prefix='.d-types-', dir=build) as folder:
        root = pathlib.Path(folder)
        # Keep declarations on their own lines: this is the existing line-header type profile.
        source = ('module fixture;\nvoid outer() {\n struct Local { int value; }\n}\n'
                  'class C {\n void method() {}\n}\ninterface I { void method(); }\n'
                  'union U { int value; }\nenum E { first, second }\n'
                  '// class Ghost {}\nstring text = "struct Fake {}";\n')
        path = root / 'source с пробелами.d'
        path.write_bytes(source.replace('\n', '\r\n').encode())
        tree = parser.parse(source.encode())
        assert not tree.root_node.has_error
        expected = []
        stack = [tree.root_node]
        while stack:
            node = stack.pop()
            if node.type in ('class_declaration', 'struct_declaration', 'interface_declaration', 'union_declaration', 'enum_declaration'):
                name = next(c for c in node.named_children if c.type == 'identifier')
                expected.append((source.encode()[name.start_byte:name.end_byte].decode(), node.start_point.row + 1))
            stack.extend(reversed(node.named_children))
        (root / 'empty.d').write_text('// empty', encoding='utf-8')
        for workers in (1, 8):
            for mode in ([], ['--all'], ['--struct']):
                for output in (['--json'], ['--names'], ['--tree'], ['--tree-full']):
                    base = ['--dir', str(root), '--workers', str(workers), *mode, *output]
                    old, new = [subprocess.run([exe, *base], capture_output=True) for exe in executables]
                    assert old.returncode == new.returncode == 0 and old.stdout == new.stdout, base
                    checks += 1
                    if mode == ['--struct'] and output == ['--json']:
                        found = [(t['name'], t['line']) for f in json.loads(new.stdout)['files'] for t in f['classes']]
                        # The inherited class_pattern omits unions. Keep that known
                        # accuracy gap visible; an optimization must not hide/change it.
                        assert ('U', 9) in expected
                        assert found == [t for t in expected if t[0] != 'U'], (found, expected)
                        checks += 1
            base = ['--dir', str(root), '--workers', str(workers), '--struct', '--json']
            result = subprocess.run([audit, '--DRT-testmode=run-main', *base], capture_output=True)
            assert result.returncode == 0, result.stderr
            assert result.stdout == subprocess.check_output([executables[1], *base], stderr=subprocess.PIPE)
            checks += 1
        for flags in (['--struct', '--map', '--json'], ['--struct', '--extract'], ['--all', '--json'], ['--doc', 'C,Local']):
            base = ['--inp', str(path), '--source', 'd', *flags]
            old, new = [subprocess.run([exe, *base], capture_output=True) for exe in executables]
            assert old.returncode == new.returncode == 0 and old.stdout == new.stdout, base
            checks += 1
        stress = root / 'stress'
        stress.mkdir()
        nested = ''.join(f'void level{i}() {{\n' for i in range(64))
        nested += 'struct Deep { int value; }\n' + '}\n' * 64
        long_literal = 'string text = "' + 'x' * 131072 + '";\n'
        contracts = ('int\n joined(int value)\n in { assert(value > 0); }\n'
                     'out(result) { assert(result > 0); }\n do {\n struct InBody {}\n return value;\n}\n')
        literals = 'string text = q"END\nclass Fake {}\nEND";\n/+ struct Hidden {} /+ nested +/ +/\nstruct Видимый {}\n'
        for name, content in (('nested', nested), ('long', long_literal), ('contracts', contracts), ('unicode', literals)):
            (stress / (name + '.d')).write_bytes(content.replace('\n', '\r\n').encode())
        for workers in (1, 8):
            base = ['--dir', str(stress), '--workers', str(workers), '--struct', '--json']
            old, new = [subprocess.run([exe, *base], capture_output=True) for exe in executables]
            assert old.returncode == new.returncode == 0 and old.stdout == new.stdout
            checks += 1
            result = subprocess.run([audit, '--DRT-testmode=run-main', *base], capture_output=True)
            assert result.returncode == 0 and result.stdout == new.stdout, result.stderr
            checks += 1
    print(f'{checks} D types-only baseline/AST/audit checks passed')


if __name__ == '__main__':
    main()
