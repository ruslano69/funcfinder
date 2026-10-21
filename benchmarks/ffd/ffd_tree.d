module ffd_tree;

import ffd_model;
import std.algorithm : sort, startsWith, canFind;
import std.array : appender;
import std.format : format;
import std.string : strip, split, join, indexOf;

private class Node {
    Sym sym;
    bool type;
    Node[] children;
    this(Sym s, bool t = false) { sym = s; type = t; }
}

// Preserve Go's first containing ancestor convention (strict bounds).
private Node[] functions(Sym[] syms) {
    auto ordered = syms.dup;
    ordered.sort!((a,b) => a.start < b.start);
    Node[] nodes, roots;
    foreach (s; ordered) nodes ~= new Node(s);
    foreach (n; nodes) {
        Node parent;
        foreach (p; nodes) if (p !is n && p.sym.start < n.sym.start && p.sym.end > n.sym.end) { parent = p; break; }
        if (parent is null) roots ~= n;
        else parent.children ~= n;
    }
    return roots;
}

private string signature(string[] lines) {
    foreach (i,line; lines) {
        auto t = line.strip;
        if (!t.length || t.startsWith("@")) continue;
        if (t.startsWith("func ")) {
            auto k = t.indexOf("{");
            return (k > 0 ? t[0..k] : t)[5..$];
        }
        if (t.startsWith("def ") || t.startsWith("async def ")) {
            auto s = t;
            for (size_t j=i+1; j<lines.length && !s.canFind(":"); ++j) s ~= " " ~ lines[j].strip;
            auto k = s.indexOf(":"); return k > 0 ? s[0..k].strip : s;
        }
        if (t.canFind("function ")) {
            auto k = t.indexOf("{"); return k > 0 ? t[0..k].strip : t;
        }
        if (t.canFind("(") && (t.canFind("{") || i+1 < lines.length)) {
            auto s = t;
            if (!s.canFind("{")) for (size_t j=i+1; j<lines.length && j<i+5; ++j) {
                s ~= " " ~ lines[j].strip; if (lines[j].canFind("{")) break;
            }
            auto k = s.indexOf("{"); if (k > 0) s = s[0..k].strip;
            return s.split.join(" ");
        }
    }
    return "";
}

string functionTree(FileResult r, string[] lines = null, bool full = false) {
    Node[] roots;
    if (r.classes.length) {
        size_t[string] indices;
        foreach (s; r.classes) {
            if (auto p = s.name in indices) {
                auto n = roots[*p];
                if (s.start < n.sym.start) n.sym.start = s.start;
                if (s.end > n.sym.end) n.sym.end = s.end;
            } else { indices[s.name] = roots.length; roots ~= new Node(s,true); }
        }
        foreach (n; roots) {
            Sym[] methods;
            foreach (s; r.functions) if (s.className == n.sym.name) methods ~= s;
            n.children = functions(methods);
        }
        Sym[] top;
        foreach (s; r.functions) if (!s.className.length) top ~= s;
        roots ~= functions(top);
    } else roots = functions(r.functions);
    auto out_ = appender!(string[])();
    void emit(Node[] nodes, bool[] ancestors) {
        foreach (i,n; nodes) {
            auto last = i+1 == nodes.length;
            string prefix;
            if (ancestors.length) {
                foreach (a; ancestors[0..$-1]) prefix ~= a ? "    " : "│   ";
                prefix ~= last ? "└── " : "├── ";
            }
            auto name = n.sym.name;
            if (full && !n.type && n.sym.start >= 1 && n.sym.end <= lines.length) {
                auto sig = signature(lines[n.sym.start-1..n.sym.end]);
                if (sig.length) name = sig;
            }
            out_ ~= prefix ~ (n.type ? "class " : ancestors.length ? "method " : "") ~ format("%s (%s-%s)",name,n.sym.start,n.sym.end);
            emit(n.children, ancestors.length ? ancestors[0..$-1] ~ last ~ false : [false]);
        }
    }
    emit(roots,null);
    return out_.data.join("\n");
}

string typeTree(Sym[] types) {
    string[] rows;
    foreach (t; types) {
        rows ~= format("%s (%s-%s) [%s]",t.name,t.start,t.end,t.kind.length ? t.kind : "class");
        foreach (i,f; t.fields) rows ~= "│   " ~ (i+1 == t.fields.length ? "└── " : "├── ") ~ format("%s %s: %s",f.name,f.type,f.line);
    }
    return rows.join("\n");
}

private class DirNode { DirNode[string] children; FileResult result; bool file; }

string directoryTree(FileResult[] results) {
    if (!results.length) return "No functions found";
    import std.path : relativePath, dirSeparator, isAbsolute;
    auto root = new DirNode;
    foreach (r; results) {
        if (!r.functions.length && !r.classes.length) continue;
        string path;
        // Go filepath.Rel rejects an absolute path against the relative base ".".
        try { path = isAbsolute(r.path) ? r.path : relativePath(r.path); } catch (Exception) { path = r.path; }
        auto parts = path.split(dirSeparator);
        auto current = root;
        foreach (i,part; parts) {
            if (!(part in current.children)) current.children[part] = new DirNode;
            current = current.children[part];
            if (i+1 == parts.length) { current.file = true; current.result = r; }
        }
    }
    auto out_ = appender!(char[])();
    void emit(DirNode node) {
        auto names = node.children.keys; names.sort;
        foreach (i,name; names) {
            bool last = i+1 == names.length;
            auto label = name;
            version (Windows) { if (label.length == 2 && label[1] == ':') label = "."; }
            if (label.length) out_ ~= (last ? "└── " : "├── ") ~ label ~ "\n";
            auto child = node.children[name];
            auto prefix = last ? "    " : "│   ";
            if (child.file) {
                auto r = child.result;
                foreach (j,f; r.functions) out_ ~= prefix ~ (j+1 == r.functions.length+r.classes.length ? "└── " : "├── ") ~ format("def %s (line %s)\n",f.name,f.start);
                foreach (j,t; r.classes) out_ ~= prefix ~ (j+1 == r.classes.length && !r.functions.length ? "└── " : "├── ") ~ format("class %s (line %s)\n",t.name,t.start);
            }
            // Go's directory formatter keeps each directory at the root indentation.
            emit(child);
        }
    }
    emit(root); return out_.data.idup;
}

unittest {
    FileResult r;
    r.functions = [Sym("outer",1,10),Sym("inner",2,9),Sym("deep",3,8)];
    assert(functionTree(r) == "outer (1-10)\n├── method inner (2-9)\n└── method deep (3-8)");
    assert(signature(["func f() {", "}"]) == "f() ");
}
