"""C# type requests: frozen CLI, complete bounds audit and independent Roslyn fixture truth."""
import argparse
import json
import pathlib
import subprocess
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    for name in ('before', 'after', 'audit', 'dotnet', 'oracle'):
        ap.add_argument('--' + name, required=True)
    args = ap.parse_args()
    before, after, audit, dotnet, oracle = [str(pathlib.Path(getattr(args, k)).resolve()) for k in ('before', 'after', 'audit', 'dotnet', 'oracle')]
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

    with tempfile.TemporaryDirectory(prefix='.cs-types-', dir=build) as folder:
        root = pathlib.Path(folder)
        source = ('namespace N;\n[System.Obsolete("old")]\npublic class C {\n'
                  ' public struct Nested { public int Value; }\n'
                  ' public C() {}\n public void Method() { int Local() => 1; }\n'
                  ' public int P { get { return 1; } }\n'
                  ' public static C operator +(C x, C y) => x;\n'
                  ' public static implicit operator int(C x) => 1;\n'
                  ' string text = @"class Fake {}";\n}\n'
                  'public record struct R(int X) { public int Method() => X; }\n'
                  'interface I { void Method(); }\nenum E { First, Second }\n')
        path = root / 'source с пробелами.cs'
        path.write_bytes(source.replace('\n', '\r\n').encode())
        manifest = root / 'manifest.json'
        manifest.write_text(json.dumps([str(path)]))
        truth = json.loads(subprocess.check_output([dotnet, oracle, str(manifest)]))['files'][0]
        assert not truth['errors'], truth['errors']
        expected = [(t['name'], t['start']) for t in truth['classes']]
        (root / 'empty.cs').write_text('// empty', encoding='utf-8')
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
        for flags in (['--struct', '--map', '--json'], ['--struct', '--extract'], ['--all', '--json'], ['--doc', 'C,Nested']):
            result = same(['--inp', str(path), '--source', 'cs', *flags])
            if flags == ['--struct', '--map', '--json']:
                found = [(t['name'], t['start'], t['end']) for t in json.loads(result)['types']]
                assert found == [(t['name'], t['start'], t['end']) for t in truth['classes']], (found, truth['classes'])
                checks += 1
        stress = root / 'stress'
        stress.mkdir()
        cases = {'long': 'class C { string text = "' + 'x' * 131072 + '"; }\n',
                 'nested': 'class C { void Method() { ' + '{ ' * 64 + 'int Local() => 1;' + '} ' * 64 + '} }\n',
                 'unicode': 'class Видимый { int поле; void @method() {} }\n',
                 'directives': 'class C { int Method() =>\n#if FEATURE\n 1;\n#else\n 2;\n#endif\n}\n#if FEATURE\nclass Disabled {}\n#else\nclass Active {}\n#endif\n',
                 'malformed': 'class Open { void Broken(\n'}
        for name, text in cases.items():
            (stress / (name + '.cs')).write_bytes(text.replace('\n', '\r\n').encode())
        (stress / 'invalid.cs').write_bytes(b'// \xff\xc0\xaf\xed\xa0\x80\nclass Valid {}\n')
        # Raw discovery covers both branches; independently verify that scope
        # against the union of Roslyn's two active preprocessor configurations.
        directive_path = stress / 'directives.cs'
        manifest.write_text(json.dumps([str(directive_path)]))
        union = set()
        for defines in ([], ['FEATURE']):
            reference = json.loads(subprocess.check_output([dotnet, oracle, str(manifest), *defines]))['files'][0]
            assert not reference['errors'], reference['errors']
            union.update((t['name'], t['start'], t['end']) for t in reference['classes'])
            checks += 1
        result = same(['--inp', str(directive_path), '--source', 'cs', '--struct', '--map', '--json'])
        assert {(t['name'], t['start'], t['end']) for t in json.loads(result)['types']} == union
        checks += 1
        for workers in (1, 8):
            base = ['--dir', str(stress), '--workers', str(workers), '--struct', '--json']
            audited(base, same(base))
    print(f'{checks} C# types-only baseline/Roslyn/audit checks passed')


if __name__ == '__main__':
    main()
