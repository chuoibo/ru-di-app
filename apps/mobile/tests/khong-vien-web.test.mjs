/** Check input creation sites and transparent style forwarding, not browser painting. */
import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const GOC = fileURLToPath(new URL("../src/rudi", import.meta.url));

function tep(dir) {
  const ra = [];
  for (const ten of readdirSync(dir)) {
    const p = join(dir, ten);
    if (statSync(p).isDirectory()) ra.push(...tep(p));
    else if (ten.endsWith(".tsx")) ra.push(p);
  }
  return ra;
}

function unwrap(node) {
  while (node && (ts.isParenthesizedExpression(node) || ts.isAsExpression(node) ||
    ts.isTypeAssertionExpression(node) || ts.isNonNullExpression(node) || ts.isSatisfiesExpression(node))) node = node.expression;
  return node;
}
function constants(scope) {
  const found = new Map();
  for (const statement of scope.statements ?? []) {
    if (!ts.isVariableStatement(statement) || !(statement.declarationList.flags & ts.NodeFlags.Const)) continue;
    for (const declaration of statement.declarationList.declarations) {
      if (ts.isIdentifier(declaration.name) && declaration.initializer) found.set(declaration.name.text, declaration.initializer);
    }
  }
  return found;
}
const SHADOWED = Symbol("shadowed binding");
function binds(name, text) {
  return ts.isIdentifier(name) ? name.text === text :
    (ts.isObjectBindingPattern(name) || ts.isArrayBindingPattern(name)) && name.elements.some(element => ts.isBindingElement(element) && binds(element.name, text));
}
function initializer(node) {
  for (let scope = node.parent; scope; scope = scope.parent) {
    if (ts.isBlock(scope) || ts.isSourceFile(scope)) {
      for (const statement of scope.statements) {
        if (!ts.isVariableStatement(statement)) continue;
        for (const declaration of statement.declarationList.declarations) if (binds(declaration.name, node.text)) {
          return ts.isIdentifier(declaration.name) && (statement.declarationList.flags & ts.NodeFlags.Const) ? declaration.initializer ?? SHADOWED : SHADOWED;
        }
      }
    }
    if ((ts.isFunctionDeclaration(scope) || ts.isFunctionExpression(scope) || ts.isArrowFunction(scope)) &&
      scope.parameters.some(parameter => binds(parameter.name, node.text))) return SHADOWED;
  }
  return undefined;
}
function opening(node) {
  node = unwrap(node);
  return node && (ts.isJsxSelfClosingElement(node) ? node : ts.isJsxElement(node) ? node.openingElement : undefined);
}
function returned(fn) {
  if (!fn?.body) return undefined;
  if (!ts.isBlock(fn.body)) return opening(fn.body);
  const returns = fn.body.statements.filter(ts.isReturnStatement);
  return returns.length === 1 ? opening(returns[0].expression) : undefined;
}
function enclosingFunction(node) {
  for (let parent = node.parent; parent; parent = parent.parent) {
    if (ts.isFunctionDeclaration(parent) || ts.isFunctionExpression(parent) || ts.isArrowFunction(parent)) return parent;
  }
  return undefined;
}

function scan(sources) {
  const modules = new Map();
  const failures = new Set();
  const wrappers = new Set();
  const checkedWrappers = new Set();
  let dem = 0;
  for (const [file, source] of sources) {
    const ast = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
    // Diagnostic objects retain source trees; only store scalar messages.
    for (const diagnostic of ast.parseDiagnostics) failures.add(`${relative(GOC, file)}: syntax ${diagnostic.code}/${diagnostic.start}: ${ts.flattenDiagnosticMessageText(diagnostic.messageText, "\n")}`);
    const imports = new Map();
    const functions = new Map();
    for (const statement of ast.statements) {
      if (ts.isImportDeclaration(statement) && ts.isStringLiteral(statement.moduleSpecifier)) {
        const bindings = statement.importClause?.namedBindings;
        if (bindings && ts.isNamedImports(bindings)) for (const binding of bindings.elements) {
          if (!statement.importClause.isTypeOnly && !binding.isTypeOnly) imports.set(binding.name.text, { imported: (binding.propertyName ?? binding.name).text, from: statement.moduleSpecifier.text });
        }
        if (bindings && ts.isNamespaceImport(bindings) && !statement.importClause.isTypeOnly) imports.set(bindings.name.text, { imported: "*", from: statement.moduleSpecifier.text });
      }
      if (ts.isFunctionDeclaration(statement) && statement.name) functions.set(statement.name.text, statement);
    }
    modules.set(file, { ast, imports, functions });
  }
  function describe(node, message) {
    const ast = node.getSourceFile();
    const position = ast.getLineAndCharacterOfPosition(node.getStart(ast));
    failures.add(`${relative(GOC, ast.fileName)}:${position.line + 1}: ${message}`);
  }
  function guarantee(node, expected, trail = new Set()) {
    node = unwrap(node);
    if (!node || trail.has(node)) return false;
    trail = new Set(trail).add(node);
    if (expected !== "outline" && ts.isPropertyAccessExpression(node) &&
      ts.isIdentifier(node.expression) && node.expression.text === expected && node.name.text === "style") return true;
    if (ts.isIdentifier(node)) {
      const value = initializer(node);
      if (value) return guarantee(value, expected, trail);
      const binding = modules.get(node.getSourceFile().fileName)?.imports.get(node.text);
      if (expected === "outline" && binding?.imported === "KHONG_VIEN_WEB" && /(?:^|\/)khong-vien-web(?:\.ts)?$/.test(binding.from)) return true;
      return false;
    }
    if (ts.isArrayLiteralExpression(node)) return node.elements.some(element => guarantee(element, expected, trail));
    if (ts.isConditionalExpression(node)) return guarantee(node.whenTrue, expected, trail) && guarantee(node.whenFalse, expected, trail);
    return false;
  }
  function spread(node, state, expected, trail = new Set()) {
    node = unwrap(node);
    if (!node || trail.has(node)) return false;
    trail = new Set(trail).add(node);
    if (ts.isIdentifier(node)) {
      if (expected !== "outline" && node.text === expected) return true;
      return spread(initializer(node), state, expected, trail);
    }
    if (ts.isConditionalExpression(node)) return spread(node.whenTrue, state, expected, trail) && spread(node.whenFalse, state, expected, trail);
    if (!ts.isObjectLiteralExpression(node)) return false;
    for (const property of node.properties) {
      if (ts.isSpreadAssignment(property)) state = spread(property.expression, state, expected, trail);
      else if (property.name) {
        const name = ts.isComputedPropertyName(property.name) ? unwrap(property.name.expression) : property.name;
        if (!name || !(ts.isIdentifier(name) || ts.isStringLiteral(name)) ||
          (ts.isComputedPropertyName(property.name) && !ts.isStringLiteral(name))) state = false;
        else if (name.text === "style") state = ts.isPropertyAssignment(property) && guarantee(property.initializer, expected);
      }
    }
    return state;
  }
  function hasStyle(element, expected) {
    let state = false;
    for (const attribute of element.attributes.properties) {
      if (ts.isJsxSpreadAttribute(attribute)) state = spread(attribute.expression, state, expected);
      else if (attribute.name.getText() === "style") state = !!attribute.initializer && ts.isJsxExpression(attribute.initializer) && guarantee(attribute.initializer.expression, expected);
    }
    return state;
  }
  function functionValue(value) {
    value = unwrap(value);
    if (!value) return undefined;
    if (ts.isFunctionExpression(value) || ts.isArrowFunction(value)) return value;
    if (ts.isCallExpression(value) && ts.isIdentifier(value.expression)) {
      const binding = modules.get(value.getSourceFile().fileName)?.imports.get(value.expression.text);
      if (binding?.from === "react" && binding.imported === "forwardRef") return value.arguments.find(arg => ts.isFunctionExpression(arg) || ts.isArrowFunction(arg));
    }
    return undefined;
  }
  function wrapper(fn, trail) {
    const element = returned(fn);
    if (!element || !component(element.tagName, trail)) return false;
    wrappers.add(fn);
    if (!checkedWrappers.has(fn)) {
      checkedWrappers.add(fn);
      const props = fn.parameters[0]?.name;
      if (!props || !ts.isIdentifier(props) || !hasStyle(element, props.text)) describe(element, "input wrapper drops incoming props.style");
    }
    return true;
  }
  function component(node, trail = new Set()) {
    node = unwrap(node);
    if (!node || trail.has(node)) return false;
    trail = new Set(trail).add(node);
    if (ts.isConditionalExpression(node)) return component(node.whenTrue, trail) && component(node.whenFalse, trail);
    if (ts.isPropertyAccessExpression(node) && ts.isIdentifier(node.expression) && node.name.text === "TextInput") {
      const module = modules.get(node.getSourceFile().fileName);
      const binding = !initializer(node.expression) && module.imports.get(node.expression.text);
      return binding?.imported === "*" && importedInput(module, binding, trail);
    }
    if (!ts.isIdentifier(node)) return false;
    const module = modules.get(node.getSourceFile().fileName);
    const value = initializer(node);
    if (value) {
      const fn = functionValue(value);
      return fn ? wrapper(fn, trail) : component(value, trail);
    }
    const binding = module.imports.get(node.text);
    if (binding?.imported === "TextInput") return importedInput(module, binding, trail);
    const fn = module.functions.get(node.text);
    return fn ? wrapper(fn, trail) : false;
  }
  function importedInput(module, binding, trail) {
    if (binding.from === "react-native") return true;
    if (binding.from.startsWith(".")) {
      const base = resolve(dirname(module.ast.fileName), binding.from);
      const target = [base, `${base}.tsx`, join(base, "index.tsx")].map(file => modules.get(file)).find(Boolean);
      if (target) return wrapper(functionValue(constants(target.ast).get("TextInput")) ?? target.functions.get("TextInput"), trail);
    }
    return false;
  }
  // Discover transparent wrappers before visiting their internal native leaves.
  for (const module of modules.values()) {
    for (const fn of module.functions.values()) wrapper(fn, new Set());
    for (const value of constants(module.ast).values()) {
      const fn = functionValue(value);
      if (fn) wrapper(fn, new Set());
    }
  }
  for (const module of modules.values()) {
    function visit(node) {
      if ((ts.isJsxSelfClosingElement(node) || ts.isJsxOpeningElement(node)) && component(node.tagName)) {
        if (!wrappers.has(enclosingFunction(node))) {
          dem += 1;
          if (!hasStyle(node, "outline")) describe(node, "TextInput style does not retain KHONG_VIEN_WEB");
        }
      }
      ts.forEachChild(node, visit);
    }
    visit(module.ast);
  }
  return { dem, thieu: [...failures].sort() };
}

test("mọi TextInput trong vỏ RuDi tắt viền trình duyệt trên web", () => {
  // The same scanner must reject lost caller styles and lost adapter forwarding.
  const fixtureRoot = join(GOC, "__outline_scanner_fixture__");
  const adapter = `import { forwardRef } from "react";
    import { TextInput as NativeInput } from "react-native";
    export const TextInput = forwardRef(function Adapter(props, ref) {
      const overrides = props.manual ? { style: [props.style, { fontSize: 20 }] } : {};
      return <NativeInput {...props} {...overrides} ref={ref} />;
    });`;
  const caller = `import { TextInput as RawInput } from "react-native";
    import { TextInput as MetricInput } from "./Adapter";
    import { KHONG_VIEN_WEB as noOutline } from "./khong-vien-web";
    function Body(props) { return <MetricInput {...props} style={props.large ? [props.style, { minHeight: 40 }] : props.style} />; }
    export function Form(props) {
      const inputStyle = [noOutline];
      const Input = props.manual ? MetricInput : RawInput;
      return <><RawInput style={[noOutline]} /><Input style={props.large ? [inputStyle, {}] : inputStyle} /><Body style={[noOutline]} /></>;
    }
    // <TextInput style={KHONG_VIEN_WEB} /> is not an element.
    const literal = "<TextInput style={KHONG_VIEN_WEB} />";
    type InputRef = Ref<TextInput>;`;
  const fixture = (body = caller, leaf = adapter) => scan(new Map([
    [join(fixtureRoot, "Form.tsx"), body], [join(fixtureRoot, "Adapter.tsx"), leaf],
  ]));
  assert.deepEqual(fixture(), { dem: 3, thieu: [] }, "scanner follows aliases, both dynamic branches and transparent wrappers without counting their leaves");
  for (const [name, body, leaf] of [
    ["caller constant", caller.replace("const inputStyle = [noOutline]", "const inputStyle = []"), adapter],
    ["one caller branch", caller.replace(": inputStyle}", ": []}"), adapter],
    ["wrapper geometry branch", caller.replace("[props.style, { minHeight: 40 }]", "[{ minHeight: 40 }]"), adapter],
    ["adapter override", caller, adapter.replace("[props.style, { fontSize: 20 }]", "{ fontSize: 20 }")],
    ["later unknown spread", caller.replace("<Body style={[noOutline]} />", "<Body style={[noOutline]} {...props.extra} />"), adapter],
    ["computed style override", caller.replace("<Body style={[noOutline]} />", "<Body style={[noOutline]} {...{ ['style']: [] }} />"), adapter],
    ["computed unknown override", caller.replace("<Body style={[noOutline]} />", "<Body style={[noOutline]} {...{ [props.key]: [] }} />"), adapter],
    ["shadowed outline parameter", caller.replace("export function Form(props)", "export function Form(props, noOutline)"), adapter],
    ["mutable outline shadow", caller.replace("const inputStyle = [noOutline]", "let noOutline = {}; const inputStyle = [noOutline]"), adapter],
  ]) {
    const result = fixture(body, leaf);
    assert.equal(result.dem, 3, `${name}: failure cannot be hidden by losing input recognition`);
    assert.ok(result.thieu.length > 0, `${name}: scanner must reject the missing style`);
  }
  const namespace = fixture(`import * as RN from "react-native";\n${caller}\nconst extra = <RN.TextInput />;`);
  assert.equal(namespace.dem, 4, "namespace TextInput also counts as a creation site");
  assert.ok(namespace.thieu.length > 0, "an extra namespace input without an outline style must fail");
  const { dem, thieu } = scan(new Map(tep(GOC).map(file => [file, readFileSync(file, "utf8")])));
  assert.ok(dem >= 6, `chỉ thấy ${dem} TextInput: bộ quét đã hỏng`);
  assert.deepEqual(thieu, [], "TextInput còn viền trình duyệt trên web");
});
