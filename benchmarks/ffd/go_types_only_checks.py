"""Go type requests: frozen CLI, complete type/alias audit and native Go AST fixture truth."""
import argparse
import json
import pathlib
import subprocess
import tempfile


def main():
    ap = argparse.ArgumentParser(description=__doc__)
    for name in ('before', 'after', 'audit', 'oracle'):
        ap.add_argument('--' + name, required=True)
    args = ap.parse_args()
    executables = [str(pathlib.Path(p).resolve()) for p in (args.before, args.after)]
    audit, oracle = [str(pathlib.Path(p).resolve()) for p in (args.audit, args.oracle)]
    build = pathlib.Path(__file__).resolve().parents[2] / 'build'
    checks = 0
    with tempfile.TemporaryDirectory(prefix='.go-types-', dir=build) as folder:
        root = pathlib.Path(folder)
        source = ('package p\ntype (\n S[T any] struct { Value T }\n I interface { M() }\n'
                  ' Alias = []int\n Scalar int\n Fn func(int) int\n)\n'
                  'func F[T interface { ~[]byte | string }](value T) T {\n type Local struct{}\n'
                  ' type LocalAlias = Scalar\n return value\n}\n'
                  'func (s S[T]) M(value func() struct{ X int }) interface{ M() } { return nil }\n'
                  'func Native(value int) (result int)\n'
                  'var literal = func() { type ClosureLocal int }\n'
                  'var text = `type Fake struct{}; func Ghost() {}`\n')
        path = root / 'source с пробелами.go'
        path.write_bytes(source.replace('\n', '\r\n').encode())
        manifest = root / 'manifest.json'
        manifest.write_text(json.dumps([str(path)]))
        truth = json.loads(subprocess.check_output([oracle, str(manifest)]))['files'][0]
        assert not truth['errors']
        expected = sorted((t['name'], t['start']) for t in truth['classes'])
        (root / 'empty.go').write_text('package empty', encoding='utf-8')
        for workers in (1, 8):
            for mode in ([], ['--all'], ['--struct']):
                for output in (['--json'], ['--names'], ['--tree'], ['--tree-full']):
                    base = ['--dir', str(root), '--workers', str(workers), *mode, *output]
                    old, new = [subprocess.run([exe, *base], capture_output=True) for exe in executables]
                    assert old.returncode == new.returncode == 0 and old.stdout == new.stdout, base
                    checks += 1
                    if mode == ['--struct'] and output != ['--json']:
                        checked = subprocess.run([audit, '--DRT-testmode=run-main', *base], capture_output=True)
                        assert checked.returncode == 0 and checked.stdout == new.stdout, checked.stderr
                        checks += 1
                    if mode == ['--struct'] and output == ['--json']:
                        doc = json.loads(new.stdout)
                        found = sorted((t['name'], t['line']) for f in doc['files'] for key in ('classes', 'aliases') for t in f.get(key, []))
                        assert found == expected, (found, expected)
                        checks += 1
            base = ['--dir', str(root), '--workers', str(workers), '--struct', '--json']
            result = subprocess.run([audit, '--DRT-testmode=run-main', *base], capture_output=True)
            assert result.returncode == 0 and result.stdout == subprocess.check_output([executables[1], *base], stderr=subprocess.PIPE), result.stderr
            checks += 1
        for flags in (['--struct', '--map', '--json'], ['--struct', '--extract'], ['--all', '--json'], ['--doc', 'S,Local']):
            base = ['--inp', str(path), '--source', 'go', *flags]
            old, new = [subprocess.run([exe, *base], capture_output=True) for exe in executables]
            assert old.returncode == new.returncode == 0 and old.stdout == new.stdout, base
            checks += 1
        stress = root / 'stress'
        stress.mkdir()
        cases = {'long': 'package p\nvar text = `' + 'x' * 131072 + '`\ntype Long struct{}\n',
                 'nested': 'package p\nfunc F() { ' + '{ ' * 64 + 'type Deep int;' + '} ' * 64 + '}\n',
                 'unicode': 'package p\ntype Видимый struct { Поле int }\n// type Hidden struct{}\n',
                 'malformed': 'package p\nfunc Broken(\ntype Open struct {\n'}
        for name, content in cases.items():
            (stress / (name + '.go')).write_bytes(content.replace('\n', '\r\n').encode())
        (stress / 'invalid.go').write_bytes(b'package p\n// \xff\xc0\xaf\xed\xa0\x80\ntype Valid struct{}\n')
        for workers in (1, 8):
            base = ['--dir', str(stress), '--workers', str(workers), '--struct', '--json']
            old, new = [subprocess.run([exe, *base], capture_output=True) for exe in executables]
            assert old.returncode == new.returncode == 0 and old.stdout == new.stdout
            checks += 1
            result = subprocess.run([audit, '--DRT-testmode=run-main', *base], capture_output=True)
            assert result.returncode == 0 and result.stdout == new.stdout, result.stderr
            checks += 1
    print(f'{checks} Go types-only baseline/native-AST/audit checks passed')


if __name__ == '__main__':
    main()
