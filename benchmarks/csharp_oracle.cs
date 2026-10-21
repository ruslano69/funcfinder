using System;
using System.IO;
using System.Linq;
using System.Collections.Generic;
using System.Text.Json;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;

var files = new List<object>();
var options = new CSharpParseOptions(LanguageVersion.Preview,
    preprocessorSymbols: args.Skip(1)).WithFeatures(new[] {
        new KeyValuePair<string, string>("FileBasedProgram", "true")
    });
foreach (var path in JsonSerializer.Deserialize<string[]>(File.ReadAllText(args[0]))!)
{
    var tree = CSharpSyntaxTree.ParseText(File.ReadAllText(path), options, path);
    var root = tree.GetRoot();
    var text = tree.GetText();
    int Line(int position) => text.Lines.GetLineFromPosition(position).LineNumber + 1;
    var errors = tree.GetDiagnostics().Where(d => d.Severity == DiagnosticSeverity.Error)
        .Select(d => d.ToString()).ToArray();
    var functions = new List<object>();
    var classes = new List<object>();
    foreach (var node in root.DescendantNodes().OfType<BaseTypeDeclarationSyntax>())
    {
        // Attribute/modifier starts belong to the declaration. For body types,
        // exclude an optional trailing semicolon from the neutral body extent.
        bool bodyless = node.CloseBraceToken.RawKind == 0 || node.CloseBraceToken.IsMissing;
        classes.Add(new { name = node.Identifier.ValueText, start = Line(node.SpanStart),
            name_line = Line(node.Identifier.SpanStart),
            end = Line((bodyless ? node.Span.End : node.CloseBraceToken.Span.End) - 1),
            bodyless, node_type = node.Kind().ToString() });
    }
    foreach (var node in root.DescendantNodes())
    {
        SyntaxToken name;
        string label;
        bool bodyless;
        switch (node)
        {
            case MethodDeclarationSyntax n:
                name = n.Identifier; label = name.ValueText;
                bodyless = n.Body == null && n.ExpressionBody == null; break;
            case ConstructorDeclarationSyntax n:
                name = n.Identifier; label = name.ValueText;
                bodyless = n.Body == null && n.ExpressionBody == null; break;
            case DestructorDeclarationSyntax n:
                name = n.Identifier; label = "~" + name.ValueText;
                bodyless = n.Body == null && n.ExpressionBody == null; break;
            case LocalFunctionStatementSyntax n:
                name = n.Identifier; label = name.ValueText;
                bodyless = n.Body == null && n.ExpressionBody == null; break;
            case OperatorDeclarationSyntax n:
                name = n.OperatorToken; label = "operator " + name.Text;
                bodyless = n.Body == null && n.ExpressionBody == null; break;
            case ConversionOperatorDeclarationSyntax n:
                name = n.ImplicitOrExplicitKeyword;
                label = name.Text + " operator " + string.Concat(n.Type.DescendantTokens().Select(t => t.Text));
                bodyless = n.Body == null && n.ExpressionBody == null; break;
            default: continue;
        }
        var attributes = node.ChildNodes().OfType<AttributeListSyntax>().ToArray();
        var headerToken = attributes.Length == 0 ? default
            : node.DescendantTokens().FirstOrDefault(t => t.SpanStart >= attributes[^1].Span.End);
        int headerStart = headerToken.RawKind == 0 ? node.SpanStart : headerToken.SpanStart;
        functions.Add(new { name = label, start = Line(node.SpanStart), header_start = Line(headerStart), name_line = Line(name.SpanStart),
            end = Line(node.Span.End - 1), bodyless, node_type = node.Kind().ToString() });
    }
    var inactive = root.DescendantTrivia(descendIntoTrivia: true)
        .Where(t => t.IsKind(SyntaxKind.DisabledTextTrivia) && t.Span.Length > 0)
        .Select(t => new[] { Line(t.SpanStart), Line(t.Span.End - 1) }).ToArray();
    files.Add(new { path, functions, classes, errors, inactive });
}
Console.WriteLine(JsonSerializer.Serialize(new {
    roslyn_version = typeof(CSharpSyntaxTree).Assembly.GetName().Version!.ToString(),
    language_version = options.LanguageVersion.ToString(), features = options.Features,
    defines = args.Skip(1).ToArray(), files
}));
