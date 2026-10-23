const fs = require('fs');
const path = require('path');
const cp = require('child_process');
const crypto = require('crypto');
// Usage: node benchmarks/script_objects_oracle.cjs PROJECT OUTPUT.json
// Requires PROJECT/node_modules/typescript. No application code is executed.
if (!process.argv[2] || !process.argv[3]) throw new Error('Expected PROJECT OUTPUT.json');
const root = path.resolve(process.argv[2]);
const ts = require(path.join(root, 'node_modules/typescript/lib/typescript.js'));
const paths = cp.execFileSync('git', ['-c', `safe.directory=${root.replaceAll('\\','/')}`, '-C', root, 'ls-files','-z','*.ts','*.tsx','*.js','*.jsx','*.mjs','*.cjs'], {encoding:'utf8'}).split('\0').filter(Boolean);
const files = [];
for (const rel of paths) {
  const file = path.resolve(root, rel), data = fs.readFileSync(file), text = data.toString('utf8');
  const language = /\.(ts|tsx)$/.test(rel) ? 'ts':'js';
  const sf = ts.createSourceFile(file,text,ts.ScriptTarget.Latest,true,rel.endsWith('.tsx')?ts.ScriptKind.TSX:rel.endsWith('.jsx')?ts.ScriptKind.JSX:language==='ts'?ts.ScriptKind.TS:ts.ScriptKind.JS);
  const objects = [], excluded = [];
  function emit(node,name,kind,source) {
    const at=node.getStart(sf), pos=sf.getLineAndCharacterOfPosition(at);
    const lineStart=sf.getPositionOfLineAndCharacter(pos.line,0);
    objects.push({kind,name,line:pos.line+1,column:Buffer.byteLength(text.slice(lineStart,at),'utf8')+1,source:(source??node.getText(sf)).replaceAll('\r\n','\n')});
  }
  function nameOf(node) { return ts.isIdentifier(node)||ts.isPrivateIdentifier(node)?node.getText(sf):ts.isStringLiteral(node)||ts.isNumericLiteral(node)?node.getText(sf):null; }
  function direct(value) { return value && (ts.isArrowFunction(value)||ts.isFunctionExpression(value)); }
  function walk(node) {
    if (ts.isFunctionDeclaration(node)) {
      if(node.name)emit(node,node.name.text,'function'); else excluded.push({kind:'anonymous declaration',line:sf.getLineAndCharacterOfPosition(node.pos).line+1});
      return;
    }
    if(ts.isClassDeclaration(node)||ts.isInterfaceDeclaration(node)||ts.isEnumDeclaration(node)||ts.isTypeAliasDeclaration(node)) {
      if(node.name)emit(node,node.name.text,'type');
      if(ts.isClassDeclaration(node)||ts.isInterfaceDeclaration(node)) for(const member of node.members)walk(member);
      return;
    }
    if(ts.isClassExpression(node)||ts.isFunctionExpression(node)||ts.isArrowFunction(node))return;
    if(ts.isVariableDeclaration(node)) {
      if(ts.isIdentifier(node.name)&&direct(node.initializer)) {
        const keyword=(node.parent.flags&ts.NodeFlags.Const)?'const':(node.parent.flags&ts.NodeFlags.Let)?'let':'var';
        emit(node,node.name.text,'function',keyword+' '+node.getText(sf)+';');
      } else if(node.initializer&&ts.isObjectLiteralExpression(node.initializer))walk(node.initializer);
      return;
    }
    if(ts.isMethodDeclaration(node)||ts.isMethodSignature(node)||ts.isGetAccessorDeclaration(node)||ts.isSetAccessorDeclaration(node)||ts.isConstructorDeclaration(node)) {
      const name=ts.isConstructorDeclaration(node)?'constructor':node.name&&nameOf(node.name);
      if(name!==null)emit(node,name,'function'); else excluded.push({kind:'computed member'});
      return;
    }
    if(ts.isPropertyDeclaration(node)||ts.isPropertyAssignment(node)) {
      const name=node.name&&nameOf(node.name);
      if(name!==null&&direct(node.initializer))emit(node,name,'function');
      else if(node.initializer&&ts.isObjectLiteralExpression(node.initializer))walk(node.initializer);
      return;
    }
    if(ts.isObjectLiteralExpression(node)) { for(const member of node.properties)walk(member);return; }
    if(ts.isCallExpression(node)||ts.isNewExpression(node))return;
    ts.forEachChild(node,walk);
  }
  walk(sf);
  files.push({path:file,language,sha256:crypto.createHash('sha256').update(data).digest('hex'),errors:sf.parseDiagnostics.map(d=>ts.flattenDiagnosticMessageText(d.messageText,' ')),objects,excluded});
}
const implementationRoot = path.resolve(__dirname, '..');
const implementation = {
  base_revision: cp.execFileSync('git', ['-C', implementationRoot, 'rev-parse', 'HEAD'], {encoding:'utf8'}).trim(),
  profile: 'working-tree source hashes; named declarations outside function bodies and call/new arguments; direct function-valued bindings/members; exact UTF-8 positions and editable source',
  sources: Object.fromEntries(['script_objects.go','token_code_edits.go','code_object.go','code_edits.go','javascript_finder.go','typescript_candidates.go','enhanced_sanitizer.go'].map(name => [name,crypto.createHash('sha256').update(fs.readFileSync(path.join(implementationRoot,'internal',name))).digest('hex')])),
};
fs.writeFileSync(process.argv[3],JSON.stringify({typescript:ts.version,root,revision:cp.execFileSync('git',['-c',`safe.directory=${root.replaceAll('\\','/')}`,'-C',root,'rev-parse','HEAD'],{encoding:'utf8'}).trim(),implementation,files},null,2));
console.log(`TypeScript ${ts.version}: ${files.length} tracked files, ${files.reduce((n,f)=>n+f.objects.length,0)} objects`);
