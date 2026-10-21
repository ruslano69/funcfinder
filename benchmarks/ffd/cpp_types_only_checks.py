"""C++ type requests: frozen CLI, complete bounds audit and independent libclang fixture truth."""
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
    from clang import cindex
    executables = [str(pathlib.Path(p).resolve()) for p in (args.before, args.after)]
    audit = str(pathlib.Path(args.audit).resolve())
    build = pathlib.Path(__file__).resolve().parents[2] / 'build'
    checks = 0
    with tempfile.TemporaryDirectory(prefix='.cpp-types-', dir=build) as folder:
        root = pathlib.Path(folder)
        source = ('namespace N {\nstruct Outer {\n struct Nested {};\n'
                  ' Outer() {}\n virtual ~Outer() = default;\n'
                  ' void method() { struct Hidden {}; }\n};\n'
                  'union U { int value; };\nenum class E { First, Second };\n'
                  'template<class T>\nstruct Box { T value; };\n}\n'
                  'void outside() { struct Local {}; }\n'
                  'const char* text = R"(struct Fake {})";\n'
                  '// class Comment {};\n')
        path = root / 'source с пробелами.cpp'
        path.write_bytes(source.replace('\n', '\r\n').encode())
        unit = cindex.Index.create().parse(str(path), args=['-x', 'c++', '-std=c++17'])
        assert not [d for d in unit.diagnostics if d.severity >= cindex.Diagnostic.Error]
        kinds = {cindex.CursorKind.STRUCT_DECL, cindex.CursorKind.CLASS_DECL, cindex.CursorKind.UNION_DECL,
                 cindex.CursorKind.ENUM_DECL, cindex.CursorKind.CLASS_TEMPLATE}
        expected = []
        local = []
        stack = [unit.cursor]
        while stack:
            node = stack.pop()
            if node.location.file and pathlib.Path(node.location.file.name) == path and node.kind in kinds and node.is_definition():
                row = (node.spelling, node.extent.start.line)
                if node.semantic_parent.kind in (cindex.CursorKind.FUNCTION_DECL, cindex.CursorKind.CXX_METHOD):
                    local.append(node.spelling)
                else:
                    expected.append(row)
            stack.extend(reversed(list(node.get_children())))
        assert sorted(local) == ['Hidden', 'Local']
        (root / 'empty.cpp').write_text('// empty', encoding='utf-8')
        for workers in (1, 8):
            for mode in ([], ['--all'], ['--struct']):
                for output in (['--json'], ['--names'], ['--tree'], ['--tree-full']):
                    base = ['--dir', str(root), '--workers', str(workers), *mode, *output]
                    old, new = [subprocess.run([exe, *base], capture_output=True) for exe in executables]
                    assert old.returncode == new.returncode == 0 and old.stdout == new.stdout, base
                    checks += 1
                    if mode == ['--struct'] and output != ['--json']:
                        audited = subprocess.run([audit, '--DRT-testmode=run-main', *base], capture_output=True)
                        assert audited.returncode == 0 and audited.stdout == new.stdout, audited.stderr
                        checks += 1
                    if mode == ['--struct'] and output == ['--json']:
                        found = [(t['name'], t['line']) for f in json.loads(new.stdout)['files'] for t in f['classes']]
                        assert found == expected, (found, expected)
                        checks += 1
            base = ['--dir', str(root), '--workers', str(workers), '--struct', '--json']
            result = subprocess.run([audit, '--DRT-testmode=run-main', *base], capture_output=True)
            assert result.returncode == 0 and result.stdout == subprocess.check_output([executables[1], *base], stderr=subprocess.PIPE), result.stderr
            checks += 1
        for flags in (['--struct', '--map', '--json'], ['--struct', '--extract'], ['--all', '--json'], ['--doc', 'Outer,Nested']):
            base = ['--inp', str(path), '--source', 'cpp', *flags]
            old, new = [subprocess.run([exe, *base], capture_output=True) for exe in executables]
            assert old.returncode == new.returncode == 0 and old.stdout == new.stdout, base
            checks += 1
        stress = root / 'stress'
        stress.mkdir()
        cases = {'long': 'const char* text = "' + 'x' * 131072 + '";\nstruct Long {};\n',
                 'nested': 'void f() { ' + '{ ' * 64 + 'struct Hidden {};' + '} ' * 64 + '}\nstruct Visible {};\n',
                 'unicode': 'struct Видимый { int поле; };\n// class Hidden {};\n',
                 'malformed': 'struct Open { int broken(\n',
                 'operators': 'struct C { C() : x{1} {}\noperator unsigned() const {return 1;}\nint operator()(int n) {return n;}\nint x; };\n',
                 'directives': '#define MAKE() \\\n struct Macro {};\n#if 0\nstruct Inactive {};\n#endif\nstruct Active {};\n'}
        for name, content in cases.items():
            (stress / (name + '.cpp')).write_bytes(content.replace('\n', '\r\n').encode())
        (stress / 'invalid.cpp').write_bytes(b'// \xff\xc0\xaf\xed\xa0\x80\nstruct Valid {};\n')
        for workers in (1, 8):
            base = ['--dir', str(stress), '--workers', str(workers), '--struct', '--json']
            old, new = [subprocess.run([exe, *base], capture_output=True) for exe in executables]
            assert old.returncode == new.returncode == 0 and old.stdout == new.stdout
            checks += 1
            result = subprocess.run([audit, '--DRT-testmode=run-main', *base], capture_output=True)
            assert result.returncode == 0 and result.stdout == new.stdout, result.stderr
            checks += 1
    print(f'{checks} C++ types-only baseline/libclang/audit checks passed')


if __name__ == '__main__':
    main()
