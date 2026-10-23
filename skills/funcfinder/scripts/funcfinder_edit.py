#!/usr/bin/env python3
"""Stdlib-only client for funcfinder's JSON code-object editing protocol.

Use it from any agent that drives tools through Python. It never builds shell
strings: JSON goes to funcfinder over stdin and every subprocess pipe is UTF-8,
so it behaves the same on Windows and Unix.

Workflow (the same request must be previewed and then applied):

    read     -> object JSON; change only its "source" field
    preview  -> validates the edited request, writes nothing
    apply    -> writes the SAME request after you reviewed the preview

Examples:

    python funcfinder_edit.py read file.go go Count > object.json
    python funcfinder_edit.py preview object.json
    python funcfinder_edit.py apply object.json

The binary is found via $FUNCFINDER, then PATH ("funcfinder" or "funcfinder.exe").
"""

import argparse
import json
import os
import shutil
import subprocess
import sys


class FuncfinderError(RuntimeError):
    """funcfinder refused the request; ``code`` is the stable error.code."""

    def __init__(self, code, message, response):
        super().__init__(f"{code}: {message}")
        self.code = code
        self.message = message
        self.response = response


def find_binary():
    explicit = os.environ.get("FUNCFINDER")
    if explicit:
        return explicit
    for name in ("funcfinder", "funcfinder.exe"):
        found = shutil.which(name)
        if found:
            return found
    raise FileNotFoundError(
        "funcfinder not found: set FUNCFINDER to the binary path or add it to PATH"
    )


def run(args, payload=None, exe=None):
    """Run funcfinder, return the parsed JSON response, or raise FuncfinderError."""
    exe = exe or find_binary()
    stdin = None if payload is None else json.dumps(payload, ensure_ascii=False)
    result = subprocess.run(
        [exe, *args],
        input=stdin,
        capture_output=True,
        text=True,
        encoding="utf-8",
    )
    if not result.stdout.strip():
        raise RuntimeError(
            f"funcfinder produced no JSON (exit {result.returncode}): {result.stderr.strip()}"
        )
    response = json.loads(result.stdout)
    if result.returncode != 0 or response.get("status") == "error":
        error = response.get("error", {})
        raise FuncfinderError(error.get("code", "unknown"), error.get("message", ""), response)
    return response


def read_object(path, source, name, kind="function", line=None, column=None):
    """Read one declaration as an editable code object (only ``source`` should change)."""
    args = ["--inp", path, "--source", source, "--read-object", name, "--object-kind", kind]
    if line is not None:
        args += ["--line", str(line)]
    if column is not None:
        args += ["--column", str(column)]
    return run(args)


def preview(request, exe=None):
    """Validate an object, operation or batch without writing. Returns the preview response."""
    return run(["--edit-object", "-"], request, exe=exe)


def apply(request, exe=None):
    """Apply the SAME request previewed earlier. Returns the applied response."""
    return run(["--edit-object", "-", "--apply"], request, exe=exe)

def pipeline(path, source, name, new_source, check=None, kind="function", line=None, column=None):
    """Replace one declaration, run an optional check, and roll back if the check fails.

    ``check`` is an argv list, never a shell string. The original declaration is kept
    in memory; on a failed check it is written back through the same preview/apply
    protocol, using the object returned by the applied edit (its revision is current).
    """
    original = read_object(path, source, name, kind, line, column)
    edited = dict(original, source=new_source)
    preview(edited)
    applied = apply(edited)
    result = {"status": "applied", "file": path, "name": name, "rolled_back": False}
    if check:
        proc = subprocess.run(check, capture_output=True, text=True, encoding="utf-8")
        result["check"] = {
            "argv": check,
            "returncode": proc.returncode,
            "stdout": proc.stdout[-4000:],
            "stderr": proc.stderr[-4000:],
        }
        if proc.returncode != 0:
            restore = dict(applied["object"], source=original["source"])
            try:
                preview(restore)
                apply(restore)
            except FuncfinderError as err:
                result["status"] = "restore_failed"
                result["restore_error"] = {"code": err.code, "message": err.message}
                return result
            result["status"] = "rolled_back"
            result["rolled_back"] = True
    return result


def load_request(path):
    # Read JSON as UTF-8 explicitly; relying on the locale breaks on Windows.
    with open(path, encoding="utf-8") as handle:
        return json.load(handle)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)

    p_read = sub.add_parser("read", help="print the editable object for a declaration")
    p_read.add_argument("file")
    p_read.add_argument("source", help="language key, e.g. go, rust, cs, js, ts, kotlin, goose")
    p_read.add_argument("name")
    p_read.add_argument("--kind", default="function", help="function (default), type, property, accessor")
    p_read.add_argument("--line", type=int)
    p_read.add_argument("--column", type=int)

    p_preview = sub.add_parser("preview", help="validate a request without writing")
    p_preview.add_argument("request", help="JSON file: object, operation wrapper or funcfinder.edits/v1 batch")

    p_apply = sub.add_parser("apply", help="apply the same request after reviewing the preview")
    p_apply.add_argument("request")

    p_pipe = sub.add_parser(
        "pipeline",
        help="replace a declaration, run a check command, roll back if the check fails",
        description="Usage: pipeline FILE SOURCE NAME --new-source NEW.txt [--kind K] [--line N --column N] --check CMD ARGS...",
    )
    p_pipe.add_argument("file")
    p_pipe.add_argument("source")
    p_pipe.add_argument("name")
    p_pipe.add_argument("--new-source", required=True, help="file holding the complete new declaration source (UTF-8)")
    p_pipe.add_argument("--kind", default="function")
    p_pipe.add_argument("--line", type=int)
    p_pipe.add_argument("--column", type=int)
    p_pipe.add_argument("--check", nargs=argparse.REMAINDER, default=None,
                        help="check command as argv; must be the LAST option (it takes the rest)")

    args = parser.parse_args(argv)
    try:
        if args.command == "read":
            response = read_object(args.file, args.source, args.name, args.kind, args.line, args.column)
        elif args.command == "preview":
            response = preview(load_request(args.request))
        elif args.command == "apply":
            response = apply(load_request(args.request))
        else:
            with open(args.new_source, encoding="utf-8") as handle:
                new_source = handle.read()
            response = pipeline(args.file, args.source, args.name, new_source,
                                check=args.check or None, kind=args.kind, line=args.line, column=args.column)
    except FuncfinderError as err:
        # Print the full structured envelope so callers can branch on error.code.
        json.dump(err.response, sys.stdout, ensure_ascii=False, indent=2)
        sys.stdout.write("\n")
        return 1
    json.dump(response, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    if response.get("status") in ("rolled_back", "restore_failed"):
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
