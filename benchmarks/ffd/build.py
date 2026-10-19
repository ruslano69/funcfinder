"""Build ffd, the D port of funcfinder's directory run, with LDC.

    python benchmarks/ffd/build.py                 # release build: build/ffd/ffd[.exe]
    python benchmarks/ffd/build.py --test          # unit tests (compiled in, run before main) and exit status
    python benchmarks/ffd/build.py --out DIR --flag=-d-version=Foo

LDC is taken from $LDC2, then PATH, then F:\\D\\ldc-1.43.0 (the project's Windows installation). On Windows the
linker needs the Visual Studio x64 environment: $FFD_LDCVARS, or F:\\D\\ldcvars64.bat, is called first. ffd_config.json
and VERSION are string imports; the config must be current (`go test ./benchmarks/ffd/genconfig` checks it).
"""
import argparse
import os
import pathlib
import platform
import shutil
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
BENCH = HERE.parent
ROOT = BENCH.parent
WINDOWS = platform.system() == "Windows"
RELEASE = ["-O3", "-release", "-mcpu=x86-64"]
# dlang_port.d carries a main for its own probe; PredicateCheck leaves it out.
COMMON = ["-i", "-d-version=PredicateCheck", f"-I={BENCH}", f"-I={HERE}", f"-J={HERE}", f"-J={ROOT}"]


def find_ldc():
    for candidate in (os.environ.get("LDC2"), shutil.which("ldc2"), r"F:\D\ldc-1.43.0\bin\ldc2.exe"):
        if candidate and pathlib.Path(candidate).exists():
            return candidate
    sys.exit("ldc2 not found: set LDC2 or put it on PATH")


def vars_script():
    if not WINDOWS:
        return None
    for candidate in (os.environ.get("FFD_LDCVARS"), r"F:\D\ldcvars64.bat"):
        if candidate and pathlib.Path(candidate).exists():
            return candidate
    return None


def run(args, out_dir, name):
    ldc = find_ldc()
    command = [ldc, *args]
    if WINDOWS:
        init = vars_script()
        script = out_dir / f"{name}.bat"
        lines = ["@echo off"]
        if init:
            lines += [f'call "{init}" >nul', "if errorlevel 1 exit /b 1"]
        lines += [" ".join(f'"{c}"' if " " in c else c for c in command), "exit /b %errorlevel%"]
        script.write_text("\r\n".join(lines) + "\r\n")
        return subprocess.run(["cmd", "/c", str(script)]).returncode
    return subprocess.run(command).returncode


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", default=str(ROOT / "build" / "ffd"), help="output directory (default: build/ffd)")
    ap.add_argument("--test", action="store_true", help="build with unit tests and run them")
    ap.add_argument("--flag", action="append", default=[], help="extra ldc2 flag (repeatable)")
    a = ap.parse_args()
    out = pathlib.Path(a.out)
    out.mkdir(parents=True, exist_ok=True)
    exe = out / ("ffd-test" if a.test else "ffd")
    if WINDOWS:
        exe = exe.with_suffix(".exe")
    flags = ["-O1", "-unittest", "-g"] if a.test else RELEASE
    code = run([*flags, *COMMON, *a.flag, f"-of={exe}", str(HERE / "ffd_main.d")], out, "build-test" if a.test else "build")
    if code:
        sys.exit(code)
    print(f"built {exe} ({exe.stat().st_size} bytes)")
    if a.test:
        # The unit tests run before main; --version then exits at once.
        sys.exit(subprocess.run([str(exe), "--version"]).returncode)


if __name__ == "__main__":
    main()
