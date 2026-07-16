const fs = require("fs");
const path = require("path");

const input = JSON.parse(fs.readFileSync(0, "utf8"));
const ts = require(input.typescript_lib_path);
const targetPath = path.resolve(input.target_path);

const results = { design: [], quality: [], security: [], debug: [] };
const seen = new Set();
const taintModel = normalizeTaintModel(input.taint_model);

const TAINT_DEFAULT_MAX_DEPTH = 8;
const TAINT_MAX_TAINTS_PER_EXPRESSION = 16;
const TAINT_SOURCE_MEMBERS = ["query", "body", "params", "headers", "cookies"];
const TAINT_SANITIZER_NAMES = new Set([
  "sqlEscape", "shellQuote", "shellEscape", "htmlEncode", "htmlEscape", "encodeHTML",
]);

const directivePatterns = [
  { pattern: /^\s*(?:(?:\/\/)|(?:\/\*+)|\*)\s*@ts-ignore\b/, suffix: "ts-ignore", message: "suppression comment should be reviewed" },
  { pattern: /^\s*(?:(?:\/\/)|(?:\/\*+)|\*)\s*@ts-nocheck\b/, suffix: "ts-nocheck", message: "file-level type checking is disabled" },
  { pattern: /^\s*(?:(?:\/\/)|(?:\/\*+)|\*)\s*@ts-expect-error\b/, suffix: "ts-expect-error", message: "suppression comment should be reviewed" },
];

main();

function main() {
  const program = loadProgram();
  const checker = program.getTypeChecker();

  for (const sourceFile of program.getSourceFiles()) {
    if (!isAnalyzableSourceFile(sourceFile)) {
      continue;
    }
    const relPath = normalizePath(path.relative(targetPath, sourceFile.fileName));
    const flavor = scriptFlavor(relPath);
    if (!flavor) {
      continue;
    }

    analyzeModuleName(sourceFile, relPath);
    analyzeDirectives(sourceFile, relPath, flavor);
    analyzeDesign(sourceFile, relPath);

    const bindings = collectBindings(sourceFile);
    sourceFile.bindings = bindings;
    analyzeTaintFlows(sourceFile, relPath, flavor, bindings, checker);
    visit(sourceFile, sourceFile, relPath, flavor, bindings, checker);
  }

  analyzeTaint(program, checker);

  process.stdout.write(JSON.stringify(results));
}

function loadProgram() {
  const configPath = findConfigPath();
  if (configPath) {
    const config = ts.readConfigFile(configPath, ts.sys.readFile);
    if (config.error) {
      throw new Error(ts.flattenDiagnosticMessageText(config.error.messageText, "\n"));
    }
    const parsed = ts.parseJsonConfigFileContent(
      config.config,
      ts.sys,
      path.dirname(configPath),
      defaultCompilerOptions(),
      configPath,
    );
    return ts.createProgram({
      rootNames: parsed.fileNames.filter((name) => isWithinTarget(path.resolve(name))),
      options: parsed.options,
    });
  }

  return ts.createProgram({
    rootNames: ts.sys.readDirectory(targetPath, scriptExtensions(), undefined, undefined),
    options: defaultCompilerOptions(),
  });
}

function findConfigPath() {
  return ts.findConfigFile(targetPath, ts.sys.fileExists, "tsconfig.json") ||
    ts.findConfigFile(targetPath, ts.sys.fileExists, "jsconfig.json");
}

function defaultCompilerOptions() {
  return {
    allowJs: true,
    checkJs: true,
    noEmit: true,
    skipLibCheck: true,
    target: ts.ScriptTarget.Latest,
    module: ts.ModuleKind.ESNext,
    jsx: ts.JsxEmit.Preserve,
  };
}

function scriptExtensions() {
  return [".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"];
}

function isAnalyzableSourceFile(sourceFile) {
  return !sourceFile.isDeclarationFile &&
    scriptFlavor(sourceFile.fileName) &&
    isWithinTarget(sourceFile.fileName) &&
    !isNodeModulesPath(sourceFile.fileName);
}

function isNodeModulesPath(fileName) {
  return normalizePath(path.resolve(fileName)).split("/").includes("node_modules");
}

function isWithinTarget(fileName) {
  const resolved = path.resolve(fileName);
  return resolved === targetPath || resolved.startsWith(targetPath + path.sep);
}

function normalizePath(value) {
  return value.split(path.sep).join("/");
}

function scriptFlavor(fileName) {
  switch (path.extname(fileName).toLowerCase()) {
    case ".ts":
    case ".tsx":
    case ".mts":
    case ".cts":
      return "typescript";
    case ".js":
    case ".jsx":
    case ".mjs":
    case ".cjs":
      return "javascript";
    default:
      return "";
  }
}

function scriptLabel(flavor) {
  return flavor === "javascript" ? "JavaScript" : "TypeScript";
}

function scriptRuleId(flavor, tsRuleId, jsRuleId) {
  return flavor === "javascript" ? jsRuleId : tsRuleId;
}

function lineNumber(sourceFile, pos) {
  return sourceFile.getLineAndCharacterOfPosition(pos).line + 1;
}

function normalizeTaintModel(model) {
  const normalized = { sources: [], sinks: [] };
  if (!model || typeof model !== "object") {
    return normalized;
  }
  normalized.sources = Array.isArray(model.sources) ? model.sources : [];
  normalized.sinks = Array.isArray(model.sinks) ? model.sinks : [];
  return normalized;
}

function pushFinding(section, sourceFile, relPath, flavor, ruleId, level, message, pos) {
  const line = lineNumber(sourceFile, pos);
  const key = [section, ruleId, relPath, line, message].join("|");
  if (seen.has(key)) {
    return;
  }
  seen.add(key);
  results[section].push({
    rule_id: ruleId,
    level,
    path: relPath,
    line,
    column: 1,
    message,
  });
}

function analyzeModuleName(sourceFile, relPath) {
  const moduleName = normalizedModuleName(relPath);
  for (const forbidden of input.forbidden_package_names || []) {
    if (moduleName !== String(forbidden || "").toLowerCase()) {
      continue;
    }
    pushFinding(
      "design",
      sourceFile,
      relPath,
      scriptFlavor(relPath),
      "design.typescript.generic-module-name",
      "warn",
      `module name "${moduleName}" is too generic`,
      0,
    );
  }
}

function normalizedModuleName(relPath) {
  const lower = path.basename(relPath).toLowerCase();
  for (const ext of [".d.ts", ".tsx", ".ts", ".jsx", ".js", ".mjs", ".cjs", ".mts", ".cts"]) {
    if (lower.endsWith(ext)) {
      return lower.slice(0, -ext.length);
    }
  }
  return lower.slice(0, -path.extname(lower).length);
}

function analyzeDirectives(sourceFile, relPath, flavor) {
  const lines = sourceFile.text.replace(/\r\n/g, "\n").split("\n");
  for (let index = 0; index < lines.length; index++) {
    const line = lines[index];
    for (const directive of directivePatterns) {
      if (!directive.pattern.test(line)) {
        continue;
      }
      pushFinding(
        "quality",
        sourceFile,
        relPath,
        flavor,
        scriptRuleId(flavor, `quality.typescript.${directive.suffix}`, `quality.javascript.${directive.suffix}`),
        "warn",
        `${scriptLabel(flavor)} ${directive.message}`,
        sourceFile.getPositionOfLineAndCharacter(index, 0),
      );
    }
  }
}

function analyzeDesign(sourceFile, relPath) {
  const flavor = scriptFlavor(relPath);
  ts.forEachChild(sourceFile, (node) => {
    if (ts.isClassDeclaration(node) || ts.isClassExpression(node)) {
      const name = classLikeName(node);
      const methods = node.members.filter(isCountedClassMember).length;
      if (methods > input.max_methods_per_type) {
        pushFinding(
          "design",
          sourceFile,
          relPath,
          flavor,
          "design.typescript.max-methods-per-type",
          "warn",
          `class ${name} has ${methods} methods; max is ${input.max_methods_per_type}`,
          node.name ? node.name.getStart(sourceFile) : node.getStart(sourceFile),
        );
      }
    }

    if (ts.isInterfaceDeclaration(node)) {
      const members = node.members.length;
      if (members > input.max_interface_members) {
        pushFinding(
          "design",
          sourceFile,
          relPath,
          flavor,
          "design.typescript.max-interface-members",
          "warn",
          `interface ${node.name.text} has ${members} members; max is ${input.max_interface_members}`,
          node.name.getStart(sourceFile),
        );
      }
    }

    if (ts.isTypeAliasDeclaration(node) && ts.isTypeLiteralNode(node.type)) {
      const members = node.type.members.length;
      if (members > input.max_interface_members) {
        pushFinding(
          "design",
          sourceFile,
          relPath,
          flavor,
          "design.typescript.max-interface-members",
          "warn",
          `type ${node.name.text} has ${members} members; max is ${input.max_interface_members}`,
          node.name.getStart(sourceFile),
        );
      }
    }
  });
}

function classLikeName(node) {
  return node.name && node.name.text ? node.name.text : "anonymous";
}

function isCountedClassMember(member) {
  return (ts.isMethodDeclaration(member) || ts.isGetAccessorDeclaration(member) || ts.isSetAccessorDeclaration(member)) &&
      member.name &&
      !ts.isConstructorDeclaration(member) ||
    ts.isPropertyDeclaration(member) &&
      member.initializer &&
      (ts.isArrowFunction(member.initializer) || ts.isFunctionExpression(member.initializer));
}

function visit(node, sourceFile, relPath, flavor, bindings, checker) {
  analyzeQualityNode(node, sourceFile, relPath, flavor);
  analyzeSecurityNode(node, sourceFile, relPath, flavor, bindings, checker);
  analyzeFunctionMetrics(node, sourceFile, relPath, flavor);
  ts.forEachChild(node, (child) => visit(child, sourceFile, relPath, flavor, bindings, checker));
}

function analyzeQualityNode(node, sourceFile, relPath, flavor) {
  if (ts.isDebuggerStatement(node)) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "quality.typescript.debugger-statement", "quality.javascript.debugger-statement"),
      "warn",
      "debugger statements should not reach committed source",
      node.getStart(sourceFile),
    );
  }

  if (node.kind === ts.SyntaxKind.AnyKeyword) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "quality.typescript.explicit-any", "quality.javascript.explicit-any"),
      "warn",
      "explicit any should be reviewed",
      node.getStart(sourceFile),
    );
  }

  if (isDoubleAssertion(node)) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "quality.typescript.double-assertion", "quality.javascript.double-assertion"),
      "warn",
      "double type assertions should be reviewed",
      node.getStart(sourceFile),
    );
  }

  if (ts.isNonNullExpression(node)) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "quality.typescript.non-null-assertion", "quality.javascript.non-null-assertion"),
      "warn",
      "non-null assertions should be reviewed",
      node.getStart(sourceFile),
    );
  }
}

function isDoubleAssertion(node) {
  return isAssertionExpression(node) &&
    isAssertionExpression(node.expression) &&
    isAnyOrUnknownType(node.expression.type);
}

function isAssertionExpression(node) {
  return ts.isAsExpression(node) || ts.isTypeAssertionExpression(node);
}

function isAnyOrUnknownType(node) {
  return !!node && (node.kind === ts.SyntaxKind.AnyKeyword || node.kind === ts.SyntaxKind.UnknownKeyword);
}

function analyzeFunctionMetrics(node, sourceFile, relPath, flavor) {
  if (!isMeasuredFunction(node)) {
    return;
  }

  const metrics = functionMetrics(node, sourceFile);
  if (metrics.length > input.max_function_lines) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      "quality.max-function-lines",
      "warn",
      `function ${metrics.name} has ${metrics.length} lines; max is ${input.max_function_lines}`,
      metrics.pos,
    );
  }
  if (metrics.params > input.max_parameters) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      "quality.max-parameters",
      "warn",
      `function ${metrics.name} has ${metrics.params} parameters; max is ${input.max_parameters}`,
      metrics.pos,
    );
  }
  if (metrics.complexity > input.max_cyclomatic_complexity) {
    pushFinding(
      "quality",
      sourceFile,
      relPath,
      flavor,
      "quality.cyclomatic-complexity",
      "warn",
      `function ${metrics.name} has cyclomatic complexity ${metrics.complexity}; max is ${input.max_cyclomatic_complexity}`,
      metrics.pos,
    );
  }
}

function isMeasuredFunction(node) {
  if (!ts.isFunctionLike(node) || ts.isConstructorDeclaration(node) || !node.body) {
    return false;
  }
  return ts.isArrowFunction(node) ||
    ts.isFunctionDeclaration(node) ||
    ts.isFunctionExpression(node) ||
    ts.isMethodDeclaration(node) ||
    ts.isGetAccessorDeclaration(node) ||
    ts.isSetAccessorDeclaration(node);
}

function functionMetrics(node, sourceFile) {
  const startLine = lineNumber(sourceFile, node.getStart(sourceFile));
  const endLine = lineNumber(sourceFile, node.body.end);
  return {
    name: functionName(node),
    pos: node.name ? node.name.getStart(sourceFile) : node.getStart(sourceFile),
    length: Math.max(1, endLine - startLine + 1),
    params: node.parameters.length,
    complexity: functionComplexity(node.body, node),
  };
}

function functionName(node) {
  if (node.name && ts.isIdentifier(node.name)) {
    return node.name.text;
  }
  if (ts.isVariableDeclaration(node.parent) && ts.isIdentifier(node.parent.name)) {
    return node.parent.name.text;
  }
  if (ts.isPropertyDeclaration(node.parent) && node.parent.name && ts.isIdentifier(node.parent.name)) {
    return node.parent.name.text;
  }
  return "anonymous";
}

function functionComplexity(body, root) {
  let complexity = 1;

  function walk(node) {
    if (node !== root && ts.isFunctionLike(node)) {
      return;
    }
    if (incrementsComplexity(node)) {
      complexity++;
    }
    ts.forEachChild(node, walk);
  }

  walk(body);
  return complexity;
}

function incrementsComplexity(node) {
  return ts.isIfStatement(node) ||
    ts.isForStatement(node) ||
    ts.isForInStatement(node) ||
    ts.isForOfStatement(node) ||
    ts.isWhileStatement(node) ||
    ts.isDoStatement(node) ||
    ts.isCatchClause(node) ||
    ts.isConditionalExpression(node) ||
    ts.isCaseClause(node) ||
    isShortCircuitOperator(node);
}

function isShortCircuitOperator(node) {
  return ts.isBinaryExpression(node) &&
    (node.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken ||
      node.operatorToken.kind === ts.SyntaxKind.BarBarToken ||
      node.operatorToken.kind === ts.SyntaxKind.QuestionQuestionToken);
}

function analyzeTaintFlows(sourceFile, relPath, flavor, bindings, checker) {
  if (taintModel.sources.length === 0 || taintModel.sinks.length === 0) {
    return;
  }
  const symbolTaintCache = new Map();
  const expressionTaintCache = new Map();
  visitTaintNode(sourceFile);

  function visitTaintNode(node) {
    if (ts.isCallExpression(node)) {
      const sink = taintSinkForCall(node, bindings, checker);
      if (sink && callHasTaintedArgument(node, sink, checker, symbolTaintCache, expressionTaintCache)) {
        pushFinding(
          "security",
          sourceFile,
          relPath,
          flavor,
          scriptRuleId(flavor, "security.typescript.untrusted-input-flow", "security.javascript.untrusted-input-flow"),
          "warn",
          `${scriptLabel(flavor)} untrusted input flows to ${sink.label}`,
          node.expression.getStart(sourceFile),
        );
      }
    }

    if (ts.isNewExpression(node)) {
      const sink = taintSinkForNewExpression(node);
      if (sink && callHasTaintedArgument(node, sink, checker, symbolTaintCache, expressionTaintCache)) {
        pushFinding(
          "security",
          sourceFile,
          relPath,
          flavor,
          scriptRuleId(flavor, "security.typescript.untrusted-input-flow", "security.javascript.untrusted-input-flow"),
          "warn",
          `${scriptLabel(flavor)} untrusted input flows to ${sink.label}`,
          node.expression.getStart(sourceFile),
        );
      }
    }

    if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken) {
      const sink = taintSinkForAssignment(node.left);
      if (sink && expressionTaint(node.right, checker, symbolTaintCache, expressionTaintCache)) {
        pushFinding(
          "security",
          sourceFile,
          relPath,
          flavor,
          scriptRuleId(flavor, "security.typescript.untrusted-input-flow", "security.javascript.untrusted-input-flow"),
          "warn",
          `${scriptLabel(flavor)} untrusted input flows to ${sink.label}`,
          node.left.getStart(sourceFile),
        );
      }
    }

    ts.forEachChild(node, visitTaintNode);
  }
}

function callHasTaintedArgument(node, sink, checker, symbolTaintCache, expressionTaintCache) {
  const args = Array.isArray(node.arguments) ? node.arguments : [];
  for (const index of sink.argument_indexes || []) {
    if (index < 0 || index >= args.length) {
      continue;
    }
    if (expressionTaint(args[index], checker, symbolTaintCache, expressionTaintCache)) {
      return true;
    }
  }
  return false;
}

function expressionTaint(node, checker, symbolTaintCache, expressionTaintCache) {
  if (!node) {
    return null;
  }
  if (expressionTaintCache.has(node)) {
    return expressionTaintCache.get(node);
  }
  expressionTaintCache.set(node, null);
  const taint = computeExpressionTaint(node, checker, symbolTaintCache, expressionTaintCache);
  expressionTaintCache.set(node, taint);
  return taint;
}

function computeExpressionTaint(node, checker, symbolTaintCache, expressionTaintCache) {
  const source = directTaintSource(node, checker, bindingsForNode(node));
  if (source) {
    return source;
  }

  if (ts.isIdentifier(node)) {
    return symbolTaint(symbolForNode(node, checker), checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isPropertyAccessExpression(node) || ts.isElementAccessExpression(node)) {
    return expressionTaint(node.expression, checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isCallExpression(node)) {
    return directTaintSource(node, checker, bindingsForNode(node));
  }

  if (ts.isParenthesizedExpression(node) || ts.isAsExpression(node) || ts.isTypeAssertionExpression(node) || ts.isNonNullExpression(node) || ts.isAwaitExpression(node)) {
    return expressionTaint(node.expression, checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isConditionalExpression(node)) {
    return expressionTaint(node.whenTrue, checker, symbolTaintCache, expressionTaintCache) ||
      expressionTaint(node.whenFalse, checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isBinaryExpression(node)) {
    return expressionTaint(node.left, checker, symbolTaintCache, expressionTaintCache) ||
      expressionTaint(node.right, checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isTemplateExpression(node)) {
    for (const span of node.templateSpans) {
      const taint = expressionTaint(span.expression, checker, symbolTaintCache, expressionTaintCache);
      if (taint) {
        return taint;
      }
    }
    return null;
  }

  if (ts.isTemplateSpan(node)) {
    return expressionTaint(node.expression, checker, symbolTaintCache, expressionTaintCache);
  }

  if (ts.isArrayLiteralExpression(node)) {
    for (const element of node.elements) {
      const taint = expressionTaint(element, checker, symbolTaintCache, expressionTaintCache);
      if (taint) {
        return taint;
      }
    }
    return null;
  }

  if (ts.isObjectLiteralExpression(node)) {
    for (const property of node.properties) {
      if (ts.isPropertyAssignment(property) || ts.isShorthandPropertyAssignment(property)) {
        const value = ts.isShorthandPropertyAssignment(property) ? property.name : property.initializer;
        const taint = expressionTaint(value, checker, symbolTaintCache, expressionTaintCache);
        if (taint) {
          return taint;
        }
      }
    }
  }

  return null;
}

function symbolTaint(symbol, checker, symbolTaintCache, expressionTaintCache) {
  if (!symbol) {
    return null;
  }
  if (symbolTaintCache.has(symbol)) {
    return symbolTaintCache.get(symbol);
  }
  symbolTaintCache.set(symbol, null);
  const aliased = checker.getAliasedSymbol && symbol.flags & ts.SymbolFlags.Alias ? checker.getAliasedSymbol(symbol) : symbol;
  if (aliased !== symbol) {
    const taint = symbolTaint(aliased, checker, symbolTaintCache, expressionTaintCache);
    symbolTaintCache.set(symbol, taint);
    return taint;
  }
  const declarations = symbol.declarations || [];
  for (const declaration of declarations) {
    const taint = taintFromDeclaration(declaration, checker, symbolTaintCache, expressionTaintCache);
    if (taint) {
      symbolTaintCache.set(symbol, taint);
      return taint;
    }
  }
  return null;
}

function taintFromDeclaration(declaration, checker, symbolTaintCache, expressionTaintCache) {
  if (ts.isVariableDeclaration(declaration)) {
    if (ts.isIdentifier(declaration.name)) {
      return expressionTaint(declaration.initializer, checker, symbolTaintCache, expressionTaintCache);
    }
    if (ts.isObjectBindingPattern(declaration.name)) {
      return expressionTaint(declaration.initializer, checker, symbolTaintCache, expressionTaintCache);
    }
  }

  if (ts.isBindingElement(declaration)) {
    const pattern = declaration.parent;
    const variableDeclaration = pattern && pattern.parent && ts.isVariableDeclaration(pattern.parent) ? pattern.parent : null;
    if (!variableDeclaration) {
      return null;
    }
    const bindingSource = expressionTaint(variableDeclaration.initializer, checker, symbolTaintCache, expressionTaintCache);
    if (bindingSource) {
      return bindingSource;
    }
  }

  if (ts.isParameter(declaration)) {
    return directTaintSource(declaration.name, checker, bindingsForNode(declaration));
  }

  return null;
}

function directTaintSource(node, checker, bindings) {
  for (const source of taintModel.sources) {
    if (source.kind === "member-access" && isConfiguredMemberSource(node, source, checker)) {
      return source;
    }
    if (source.kind === "call-result" && isConfiguredCallSource(node, source, checker, bindings)) {
      return source;
    }
  }
  return null;
}

function isConfiguredMemberSource(node, source, checker) {
  if (!ts.isPropertyAccessExpression(node) && !ts.isElementAccessExpression(node)) {
    return false;
  }
  const property = accessedPropertyName(node);
  if (!property || !(source.base_property_names || []).includes(property)) {
    return false;
  }
  const base = node.expression;
  if (ts.isIdentifier(base) && (source.base_identifiers || []).includes(base.text)) {
    return true;
  }
  return expressionTypeMatches(base, checker, source.receiver_type_names || source.base_type_names || []);
}

function isConfiguredCallSource(node, source, checker, bindings) {
  if (!ts.isCallExpression(node) || !ts.isPropertyAccessExpression(node.expression)) {
    return false;
  }
  if (!(source.call_members || []).includes(node.expression.name.text)) {
    return false;
  }
  const receiver = node.expression.expression;
  const target = callTarget(node.expression, bindings || emptyBindings());
  return expressionTypeMatches(receiver, checker, source.receiver_type_names || []) ||
    (!!target && target.module === source.module);
}

function expressionTypeMatches(node, checker, expectedNames) {
  if (!node || !checker || !Array.isArray(expectedNames) || expectedNames.length === 0) {
    return false;
  }
  try {
    const type = checker.getTypeAtLocation(node);
    const text = checker.typeToString(type);
    return expectedNames.some((name) => text === name || text.endsWith(`.${name}`) || text.includes(name));
  } catch (error) {
    return false;
  }
}

function taintSinkForCall(node, bindings, checker) {
  for (const sink of taintModel.sinks) {
    if (sink.kind !== "call") {
      continue;
    }
    if (matchesCallSink(node, sink, bindings, checker)) {
      return sink;
    }
  }
  return null;
}

function taintSinkForNewExpression(node) {
  for (const sink of taintModel.sinks) {
    if (sink.kind === "new" && ts.isIdentifier(node.expression) && node.expression.text === sink.member) {
      return sink;
    }
  }
  return null;
}

function taintSinkForAssignment(left) {
  for (const sink of taintModel.sinks) {
    if (sink.kind === "assignment" && propertyAccessMatches(left, sink.property_name)) {
      return sink;
    }
  }
  return null;
}

function matchesCallSink(node, sink, bindings, checker) {
  if (sink.member === "document.write" || sink.member === "document.writeln") {
    return isDocumentWriteMember(node.expression, sink.member);
  }
  if (sink.module) {
    const target = callTarget(node.expression, bindings);
    return !!target && target.module === sink.module && target.member === sink.member;
  }
  if (sink.member === "eval" || sink.member === "Function") {
    return ts.isIdentifier(node.expression) && node.expression.text === sink.member;
  }
  return propertyAccessMatches(node.expression, sink.member);
}

function isDocumentWriteMember(expression, name) {
  return ts.isPropertyAccessExpression(expression) &&
    `${expression.expression.getText()}.${expression.name.text}` === name;
}

function propertyAccessMatches(node, propertyName) {
  return ts.isPropertyAccessExpression(node) && node.name.text === propertyName;
}

function accessedPropertyName(node) {
  if (ts.isPropertyAccessExpression(node)) {
    return node.name.text;
  }
  if (ts.isElementAccessExpression(node) && isStringLiteralArgument(node.argumentExpression)) {
    return literalText(node.argumentExpression);
  }
  return "";
}

function symbolForNode(node, checker) {
  if (!node || !checker) {
    return null;
  }
  try {
    return checker.getSymbolAtLocation(node);
  } catch (error) {
    return null;
  }
}

function bindingsForNode(node) {
  let current = node;
  while (current) {
    if (current.bindings) {
      return current.bindings;
    }
    current = current.parent;
  }
  return emptyBindings();
}

function emptyBindings() {
  return { named: new Map(), namespaces: new Map() };
}

function analyzeSecurityNode(node, sourceFile, relPath, flavor, bindings, checker) {
  if (isRejectUnauthorizedFalse(node)) {
    pushFinding(
      "security",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "security.typescript.insecure-tls", "security.javascript.insecure-tls"),
      "fail",
      `${scriptLabel(flavor)} TLS verification is disabled`,
      node.getStart(sourceFile),
    );
  }

  if (isNodeTLSDisableAssignment(node)) {
    pushFinding(
      "security",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "security.typescript.insecure-tls", "security.javascript.insecure-tls"),
      "fail",
      "NODE_TLS_REJECT_UNAUTHORIZED disables TLS verification",
      node.getStart(sourceFile),
    );
  }

  if (ts.isCallExpression(node)) {
    analyzeCallExpression(node, sourceFile, relPath, flavor, bindings, checker);
  }

  if (ts.isNewExpression(node) && isDynamicFunctionConstructor(node)) {
    pushFinding(
      "security",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "security.typescript.dynamic-code", "security.javascript.dynamic-code"),
      "warn",
      "dynamic code execution should be reviewed",
      node.getStart(sourceFile),
    );
  }

  if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken && isUnsafeHTMLAssignment(node.left)) {
    pushFinding(
      "security",
      sourceFile,
      relPath,
      flavor,
      scriptRuleId(flavor, "security.typescript.unsafe-html-sink", "security.javascript.unsafe-html-sink"),
      "warn",
      "unsafe HTML injection sink should be reviewed",
      node.left.getStart(sourceFile),
    );
  }
}

function analyzeCallExpression(node, sourceFile, relPath, flavor, bindings, checker) {
  const directCallee = callTarget(node.expression, bindings);
  const rule = scriptRuleId;

  if (isEvalCall(node.expression) || isFunctionCall(node.expression)) {
    pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.dynamic-code", "security.javascript.dynamic-code"), "warn", "dynamic code execution should be reviewed", node.expression.getStart(sourceFile));
  }

  if (directCallee && directCallee.module === "child_process") {
    if (directCallee.member === "exec" || directCallee.member === "execSync") {
      pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.shell-execution", "security.javascript.shell-execution"), "warn", `${scriptLabel(flavor)} shell execution primitive should be reviewed`, node.expression.getStart(sourceFile));
    }
    if ((directCallee.member === "spawn" || directCallee.member === "spawnSync") && hasShellTrueOption(node.arguments)) {
      pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.shell-execution", "security.javascript.shell-execution"), "warn", `${scriptLabel(flavor)} shell execution primitive should be reviewed`, node.expression.getStart(sourceFile));
    }
  }

  if (directCallee && directCallee.module === "vm" && isVMExecutionMember(directCallee.member)) {
    pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.vm-dynamic-code", "security.javascript.vm-dynamic-code"), "warn", "Node vm dynamic code execution should be reviewed", node.expression.getStart(sourceFile));
  }

  if (isInsertAdjacentHTML(node.expression) || isDocumentWrite(node.expression)) {
    pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.unsafe-html-sink", "security.javascript.unsafe-html-sink"), "warn", "unsafe HTML injection sink should be reviewed", node.expression.getStart(sourceFile));
  }

  if (isTimerCall(node.expression) && node.arguments.length > 0 && isStringLiteralArgument(node.arguments[0])) {
    pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.string-timer-code", "security.javascript.string-timer-code"), "warn", `${scriptLabel(flavor)} string-based timer execution should be reviewed`, node.expression.getStart(sourceFile));
  }

  if (isPostMessageCall(node.expression) && node.arguments.length > 1 && isWildcardString(node.arguments[1])) {
    pushFinding("security", sourceFile, relPath, flavor, rule(flavor, "security.typescript.postmessage-wildcard", "security.javascript.postmessage-wildcard"), "warn", `${scriptLabel(flavor)} postMessage wildcard origin should be reviewed`, node.expression.getStart(sourceFile));
  }
}

function isRejectUnauthorizedFalse(node) {
  return ts.isPropertyAssignment(node) &&
    propertyName(node.name) === "rejectUnauthorized" &&
    node.initializer.kind === ts.SyntaxKind.FalseKeyword;
}

function isNodeTLSDisableAssignment(node) {
  return ts.isBinaryExpression(node) &&
    node.operatorToken.kind === ts.SyntaxKind.EqualsToken &&
    isNodeTLSIdentifier(node.left) &&
    isZeroLiteral(node.right);
}

function isNodeTLSIdentifier(node) {
  if (ts.isIdentifier(node)) {
    return node.text === "NODE_TLS_REJECT_UNAUTHORIZED";
  }
  if (ts.isPropertyAccessExpression(node)) {
    return node.name.text === "NODE_TLS_REJECT_UNAUTHORIZED";
  }
  if (ts.isElementAccessExpression(node) && isStringLiteralArgument(node.argumentExpression)) {
    return literalText(node.argumentExpression) === "NODE_TLS_REJECT_UNAUTHORIZED";
  }
  return false;
}

function isZeroLiteral(node) {
  return (ts.isStringLiteralLike(node) && node.text === "0") ||
    (ts.isNumericLiteral(node) && node.text === "0");
}

function collectBindings(sourceFile) {
  const named = new Map();
  const namespaces = new Map();

  for (const statement of sourceFile.statements) {
    if (ts.isImportDeclaration(statement) && ts.isStringLiteral(statement.moduleSpecifier)) {
      const moduleName = stripNodePrefix(statement.moduleSpecifier.text);
      const clause = statement.importClause;
      if (!clause) {
        continue;
      }
      if (clause.name) {
        namespaces.set(clause.name.text, moduleName);
      }
      if (clause.namedBindings) {
        if (ts.isNamespaceImport(clause.namedBindings)) {
          namespaces.set(clause.namedBindings.name.text, moduleName);
        } else if (ts.isNamedImports(clause.namedBindings)) {
          for (const element of clause.namedBindings.elements) {
            named.set(element.name.text, {
              module: moduleName,
              member: element.propertyName ? element.propertyName.text : element.name.text,
            });
          }
        }
      }
      continue;
    }

    if (!ts.isVariableStatement(statement)) {
      continue;
    }
    for (const declaration of statement.declarationList.declarations) {
      collectRequireBinding(declaration, named, namespaces);
    }
  }

  return { named, namespaces };
}

function collectRequireBinding(declaration, named, namespaces) {
  const directModule = requireModuleName(declaration.initializer);
  if (directModule) {
    if (ts.isIdentifier(declaration.name)) {
      namespaces.set(declaration.name.text, directModule);
    }
    if (ts.isObjectBindingPattern(declaration.name)) {
      for (const element of declaration.name.elements) {
        const alias = element.name && ts.isIdentifier(element.name) ? element.name.text : "";
        if (!alias) {
          continue;
        }
        named.set(alias, {
          module: directModule,
          member: element.propertyName && ts.isIdentifier(element.propertyName) ? element.propertyName.text : alias,
        });
      }
    }
    return;
  }

  if (!ts.isIdentifier(declaration.name) || !ts.isPropertyAccessExpression(declaration.initializer || {})) {
    return;
  }
  const moduleName = requireModuleName(declaration.initializer.expression);
  if (!moduleName) {
    return;
  }
  named.set(declaration.name.text, {
    module: moduleName,
    member: declaration.initializer.name.text,
  });
}

function requireModuleName(node) {
  if (!node || !ts.isCallExpression(node) || !ts.isIdentifier(node.expression) || node.expression.text !== "require" || node.arguments.length !== 1) {
    return "";
  }
  if (!ts.isStringLiteral(node.arguments[0])) {
    return "";
  }
  return stripNodePrefix(node.arguments[0].text);
}

function stripNodePrefix(moduleName) {
  return String(moduleName || "").replace(/^node:/, "");
}

function callTarget(expression, bindings) {
  if (ts.isIdentifier(expression) && bindings.named.has(expression.text)) {
    return bindings.named.get(expression.text);
  }
  if (ts.isPropertyAccessExpression(expression)) {
    if (ts.isIdentifier(expression.expression) && bindings.namespaces.has(expression.expression.text)) {
      return { module: bindings.namespaces.get(expression.expression.text), member: expression.name.text };
    }
    const moduleName = requireModuleName(expression.expression);
    if (moduleName) {
      return { module: moduleName, member: expression.name.text };
    }
  }
  return null;
}

function hasShellTrueOption(args) {
  return args.some((argument) =>
    ts.isObjectLiteralExpression(argument) &&
    argument.properties.some((property) =>
      ts.isPropertyAssignment(property) &&
      propertyName(property.name) === "shell" &&
      property.initializer.kind === ts.SyntaxKind.TrueKeyword,
    ),
  );
}

function propertyName(name) {
  if (ts.isIdentifier(name) || ts.isStringLiteral(name) || ts.isNumericLiteral(name)) {
    return name.text;
  }
  if (ts.isComputedPropertyName(name) && ts.isStringLiteralLike(name.expression)) {
    return name.expression.text;
  }
  return "";
}

function isVMExecutionMember(member) {
  return member === "runInContext" ||
    member === "runInNewContext" ||
    member === "runInThisContext" ||
    member === "compileFunction";
}

function isEvalCall(expression) {
  return ts.isIdentifier(expression) && expression.text === "eval";
}

function isFunctionCall(expression) {
  return ts.isIdentifier(expression) && expression.text === "Function";
}

function isDynamicFunctionConstructor(node) {
  return ts.isIdentifier(node.expression) && node.expression.text === "Function";
}

function isUnsafeHTMLAssignment(node) {
  return ts.isPropertyAccessExpression(node) &&
    (node.name.text === "innerHTML" || node.name.text === "outerHTML");
}

function isInsertAdjacentHTML(expression) {
  return ts.isPropertyAccessExpression(expression) && expression.name.text === "insertAdjacentHTML";
}

function isDocumentWrite(expression) {
  return ts.isPropertyAccessExpression(expression) &&
    (expression.name.text === "write" || expression.name.text === "writeln") &&
    ts.isIdentifier(expression.expression) &&
    expression.expression.text === "document";
}

function isTimerCall(expression) {
  return ts.isIdentifier(expression) &&
    (expression.text === "setTimeout" || expression.text === "setInterval");
}

function isPostMessageCall(expression) {
  return ts.isIdentifier(expression) && expression.text === "postMessage" ||
    ts.isPropertyAccessExpression(expression) && expression.name.text === "postMessage";
}

function isStringLiteralArgument(node) {
  return ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node);
}

function isWildcardString(node) {
  return isStringLiteralArgument(node) && literalText(node) === "*";
}

function literalText(node) {
  return node.text || "";
}

// ===== Cross-module taint analysis =====
//
// Function-summary based propagation: every analyzable function is analyzed
// exactly once (memoized) and produces a summary with
//   - paramSinks:    parameter index -> sink reached inside the function or its callees
//   - paramReturns:  parameter index -> taint flows to the return value
//   - sourceReturns: taint sources inside the function that flow to the return value
// Call sites combine argument taint with callee summaries instead of inlining
// bodies, which keeps the analysis linear in program size and cycle-safe.
// Chain length is capped by taint_max_depth; truncation emits a debug note.

function configuredTaintMaxDepth() {
  const value = Number(input.taint_max_depth);
  if (!Number.isFinite(value) || value === 0) {
    return TAINT_DEFAULT_MAX_DEPTH;
  }
  return value;
}

function analyzeTaint(program, checker) {
  const depthCap = configuredTaintMaxDepth();
  if (depthCap < 0) {
    return;
  }
  const ctx = {
    checker,
    depthCap,
    summaries: new WeakMap(),
    previous: null,
    inProgress: new Set(),
    sawCycle: false,
    debugNotes: new Set(),
  };
  runTaintPass(program, ctx);
  if (ctx.sawCycle) {
    // Recursion cycles were cut with incomplete summaries. Re-run the sweep
    // once, resolving cycle edges with the first-pass summaries, so flows
    // through mutually recursive functions are still reported.
    ctx.previous = ctx.summaries;
    ctx.summaries = new WeakMap();
    runTaintPass(program, ctx);
  }
  for (const note of ctx.debugNotes) {
    results.debug.push(note);
  }
}

function runTaintPass(program, ctx) {
  for (const sourceFile of program.getSourceFiles()) {
    if (!isAnalyzableSourceFile(sourceFile)) {
      continue;
    }
    taintSummaryFor(sourceFile, ctx);
    forEachTaintFunction(sourceFile, (fn) => taintSummaryFor(fn, ctx));
  }
}

function forEachTaintFunction(root, callback) {
  ts.forEachChild(root, function walk(node) {
    if (isTaintAnalyzableFunction(node)) {
      callback(node);
    }
    ts.forEachChild(node, walk);
  });
}

function isTaintAnalyzableFunction(node) {
  return !!node && ts.isFunctionLike(node) && !!node.body && (
    ts.isFunctionDeclaration(node) ||
    ts.isFunctionExpression(node) ||
    ts.isArrowFunction(node) ||
    ts.isMethodDeclaration(node) ||
    ts.isConstructorDeclaration(node) ||
    ts.isGetAccessorDeclaration(node) ||
    ts.isSetAccessorDeclaration(node)
  );
}

function taintSummaryFor(fn, ctx) {
  if (ctx.summaries.has(fn)) {
    return ctx.summaries.get(fn);
  }
  if (ctx.inProgress.has(fn)) {
    // Recursive call cycle: cut the edge. The first pass falls back to an
    // empty summary; the second pass reuses the first-pass summary.
    ctx.sawCycle = true;
    return (ctx.previous && ctx.previous.get(fn)) || emptyTaintSummary();
  }
  ctx.inProgress.add(fn);
  const summary = emptyTaintSummary();
  analyzeTaintBody(fn, summary, ctx);
  ctx.inProgress.delete(fn);
  ctx.summaries.set(fn, summary);
  return summary;
}

function emptyTaintSummary() {
  return { paramSinks: [], paramReturns: new Map(), sourceReturns: [] };
}

function analyzeTaintBody(fn, summary, ctx) {
  const sourceFile = fn.getSourceFile();
  const scope = {
    fn,
    sourceFile,
    relPath: normalizePath(path.relative(targetPath, sourceFile.fileName)),
    env: new Map(),
    params: taintParameterIndexes(fn, ctx),
    summary,
    ctx,
  };
  const body = ts.isSourceFile(fn) ? fn : fn.body;
  walkTaintNode(body, scope);
  if (!ts.isSourceFile(fn) && !ts.isBlock(body)) {
    recordReturnTaints(taintsOfExpression(body, scope), scope);
  }
}

function taintParameterIndexes(fn, ctx) {
  const params = new Map();
  if (ts.isSourceFile(fn)) {
    return params;
  }
  fn.parameters.forEach((parameter, index) => {
    if (!ts.isIdentifier(parameter.name)) {
      return;
    }
    const symbol = ctx.checker.getSymbolAtLocation(parameter.name);
    if (symbol) {
      params.set(symbol, index);
    }
  });
  return params;
}

function walkTaintNode(node, scope) {
  visitTaintNode(node, scope);
  ts.forEachChild(node, (child) => {
    if (ts.isFunctionLike(child)) {
      return; // nested functions get their own summaries
    }
    walkTaintNode(child, scope);
  });
}

function visitTaintNode(node, scope) {
  if (ts.isVariableDeclaration(node) && node.initializer) {
    assignTaintToBinding(node.name, taintsOfExpression(node.initializer, scope), scope);
    return;
  }
  if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken) {
    visitTaintAssignment(node, scope);
    return;
  }
  if (ts.isCallExpression(node)) {
    visitTaintCall(node, scope);
    return;
  }
  if (ts.isNewExpression(node) && isDynamicFunctionConstructor(node)) {
    checkTaintSinkArgument(node, 0, "code", "new Function", scope);
    return;
  }
  if (ts.isReturnStatement(node) && node.expression) {
    recordReturnTaints(taintsOfExpression(node.expression, scope), scope);
  }
}

function visitTaintAssignment(node, scope) {
  if (isUnsafeHTMLAssignment(node.left)) {
    reportTaintsAtSink(taintsOfExpression(node.right, scope), "html", node.left.getText(scope.sourceFile), node.left, scope);
    return;
  }
  if (ts.isIdentifier(node.left)) {
    const symbol = scope.ctx.checker.getSymbolAtLocation(node.left);
    if (symbol) {
      scope.env.set(symbol, taintsOfExpression(node.right, scope));
    }
  }
}

function assignTaintToBinding(name, taints, scope) {
  if (ts.isIdentifier(name)) {
    const symbol = scope.ctx.checker.getSymbolAtLocation(name);
    if (symbol) {
      scope.env.set(symbol, taints);
    }
    return;
  }
  if (ts.isObjectBindingPattern(name) || ts.isArrayBindingPattern(name)) {
    for (const element of name.elements) {
      if (ts.isBindingElement(element)) {
        assignTaintToBinding(element.name, taints, scope);
      }
    }
  }
}

function visitTaintCall(node, scope) {
  const target = resolveTaintCallee(node, scope.ctx);
  if (target) {
    applyCalleeSinkSummary(node, taintSummaryFor(target, scope.ctx), scope);
    return;
  }
  const sink = directTaintSink(node, scope);
  if (sink) {
    checkTaintSinkArgument(node, sink.argIndex, sink.kind, sink.label, scope);
  }
}

function directTaintSink(node, scope) {
  const expression = node.expression;
  if (ts.isIdentifier(expression)) {
    return directIdentifierTaintSink(expression, scope);
  }
  if (ts.isPropertyAccessExpression(expression)) {
    return directMemberTaintSink(expression, scope);
  }
  return null;
}

function directIdentifierTaintSink(expression, scope) {
  const name = expression.text;
  if (name === "eval" || name === "Function") {
    return { argIndex: 0, kind: "code", label: name };
  }
  if (name === "fetch") {
    return { argIndex: 0, kind: "url", label: "fetch" };
  }
  const binding = taintFileBindings(scope).named.get(name);
  if (binding && binding.module === "child_process" && (binding.member === "exec" || binding.member === "execSync")) {
    return { argIndex: 0, kind: "shell", label: name };
  }
  return null;
}

function directMemberTaintSink(expression, scope) {
  const member = expression.name.text;
  const label = expression.getText(scope.sourceFile);
  if (member === "query" || member === "execute") {
    return { argIndex: 0, kind: "sql", label };
  }
  if ((member === "exec" || member === "execSync") && isChildProcessNamespace(expression.expression, scope)) {
    return { argIndex: 0, kind: "shell", label };
  }
  if ((member === "write" || member === "writeln") && ts.isIdentifier(expression.expression) && expression.expression.text === "document") {
    return { argIndex: 0, kind: "html", label };
  }
  if (member === "insertAdjacentHTML") {
    return { argIndex: 1, kind: "html", label };
  }
  return null;
}

function isChildProcessNamespace(expression, scope) {
  return ts.isIdentifier(expression) &&
    taintFileBindings(scope).namespaces.get(expression.text) === "child_process";
}

function taintFileBindings(scope) {
  if (!scope.bindings) {
    scope.bindings = collectBindings(scope.sourceFile);
  }
  return scope.bindings;
}

function checkTaintSinkArgument(node, argIndex, kind, label, scope) {
  const args = node.arguments || [];
  if (args.length <= argIndex) {
    return;
  }
  reportTaintsAtSink(taintsOfExpression(args[argIndex], scope), kind, label, node, scope);
}

function reportTaintsAtSink(taints, kind, label, node, scope) {
  const line = lineNumber(scope.sourceFile, node.getStart(scope.sourceFile));
  const sinkStep = `${label} sink (${scope.relPath}:${line})`;
  for (const taint of taints) {
    if (taint.sanitized.includes(kind)) {
      continue;
    }
    if (taint.kind === "source") {
      pushTaintFinding(taint.steps.concat([sinkStep]), scope.relPath, line);
    } else {
      scope.summary.paramSinks.push({
        param: taint.param,
        kind,
        hops: taint.hops,
        steps: taint.steps.concat([sinkStep]),
        path: scope.relPath,
        line,
      });
    }
  }
}

function applyCalleeSinkSummary(node, summary, scope) {
  if (!summary.paramSinks.length) {
    return;
  }
  const calleeName = taintCalleeName(node.expression) || "callee";
  const line = lineNumber(scope.sourceFile, node.getStart(scope.sourceFile));
  const callStep = `${calleeName} arg (${scope.relPath}:${line})`;
  (node.arguments || []).forEach((argument, index) => {
    const entries = summary.paramSinks.filter((entry) => entry.param === index);
    if (!entries.length) {
      return;
    }
    for (const taint of taintsOfExpression(argument, scope)) {
      for (const entry of entries) {
        applySinkSummaryEntry(taint, entry, callStep, calleeName, line, scope);
      }
    }
  });
}

function applySinkSummaryEntry(taint, entry, callStep, calleeName, line, scope) {
  if (taint.sanitized.includes(entry.kind)) {
    return;
  }
  const hops = taint.hops + entry.hops + 1;
  if (hops > scope.ctx.depthCap) {
    noteTaintDepthCap(calleeName, line, scope);
    return;
  }
  const steps = taint.steps.concat([callStep], entry.steps);
  if (taint.kind === "source") {
    pushTaintFinding(steps, entry.path, entry.line);
    return;
  }
  scope.summary.paramSinks.push({
    param: taint.param,
    kind: entry.kind,
    hops,
    steps,
    path: entry.path,
    line: entry.line,
  });
}

function noteTaintDepthCap(calleeName, line, scope) {
  scope.ctx.debugNotes.add(
    `taint analysis depth cap ${scope.ctx.depthCap} truncated the call chain at ${calleeName} (${scope.relPath}:${line})`,
  );
}

function pushTaintFinding(steps, sinkPath, sinkLine) {
  const flavor = scriptFlavor(sinkPath) || "typescript";
  const ruleId = scriptRuleId(flavor, "security.typescript.taint-flow", "security.javascript.taint-flow");
  const message = "tainted data flow: " + steps.join(" → ");
  const key = ["security", ruleId, sinkPath, sinkLine, message].join("|");
  if (seen.has(key)) {
    return;
  }
  seen.add(key);
  results.security.push({
    rule_id: ruleId,
    level: "warn",
    path: sinkPath,
    line: sinkLine,
    column: 1,
    message,
  });
}

function recordReturnTaints(taints, scope) {
  if (ts.isSourceFile(scope.fn)) {
    return;
  }
  for (const taint of taints) {
    if (taint.kind === "param") {
      const existing = scope.summary.paramReturns.get(taint.param);
      if (existing === undefined || taint.hops < existing) {
        scope.summary.paramReturns.set(taint.param, taint.hops);
      }
    } else if (scope.summary.sourceReturns.length < TAINT_MAX_TAINTS_PER_EXPRESSION) {
      scope.summary.sourceReturns.push(taint);
    }
  }
}

function taintsOfExpression(node, scope) {
  if (!node) {
    return [];
  }
  if (isTaintTransparentWrapper(node)) {
    return taintsOfExpression(node.expression, scope);
  }
  const combined = taintsOfCombiningExpression(node, scope);
  if (combined) {
    return capTaintList(combined);
  }
  const sourceTaint = taintSourceFor(node, scope);
  if (sourceTaint) {
    return [sourceTaint];
  }
  if (ts.isIdentifier(node)) {
    return taintsOfIdentifier(node, scope);
  }
  if (ts.isPropertyAccessExpression(node) || ts.isElementAccessExpression(node)) {
    return taintsOfExpression(node.expression, scope);
  }
  if (ts.isCallExpression(node)) {
    return capTaintList(taintsOfCallResult(node, scope));
  }
  if (ts.isSpreadElement(node)) {
    return taintsOfExpression(node.expression, scope);
  }
  return [];
}

function isTaintTransparentWrapper(node) {
  return ts.isParenthesizedExpression(node) ||
    ts.isAsExpression(node) ||
    ts.isTypeAssertionExpression(node) ||
    ts.isNonNullExpression(node) ||
    ts.isAwaitExpression(node);
}

function taintsOfCombiningExpression(node, scope) {
  if (ts.isBinaryExpression(node)) {
    return taintsOfBinaryExpression(node, scope);
  }
  if (ts.isTemplateExpression(node)) {
    return node.templateSpans.reduce(
      (taints, span) => taints.concat(taintsOfExpression(span.expression, scope)),
      [],
    );
  }
  if (ts.isConditionalExpression(node)) {
    return taintsOfExpression(node.whenTrue, scope).concat(taintsOfExpression(node.whenFalse, scope));
  }
  if (ts.isArrayLiteralExpression(node)) {
    return node.elements.reduce((taints, element) => taints.concat(taintsOfExpression(element, scope)), []);
  }
  if (ts.isObjectLiteralExpression(node)) {
    return node.properties.reduce((taints, property) => {
      if (ts.isPropertyAssignment(property)) {
        return taints.concat(taintsOfExpression(property.initializer, scope));
      }
      if (ts.isShorthandPropertyAssignment(property)) {
        return taints.concat(taintsOfExpression(property.name, scope));
      }
      return taints;
    }, []);
  }
  return null;
}

function taintsOfBinaryExpression(node, scope) {
  const kind = node.operatorToken.kind;
  if (kind === ts.SyntaxKind.PlusToken ||
    kind === ts.SyntaxKind.BarBarToken ||
    kind === ts.SyntaxKind.QuestionQuestionToken ||
    kind === ts.SyntaxKind.CommaToken) {
    return taintsOfExpression(node.left, scope).concat(taintsOfExpression(node.right, scope));
  }
  if (kind === ts.SyntaxKind.EqualsToken || kind === ts.SyntaxKind.PlusEqualsToken) {
    return taintsOfExpression(node.right, scope);
  }
  return [];
}

function taintSourceFor(node, scope) {
  if (!ts.isPropertyAccessExpression(node) || !ts.isIdentifier(node.expression)) {
    return null;
  }
  const base = node.expression.text;
  const member = node.name.text;
  if ((base === "req" || base === "request") && TAINT_SOURCE_MEMBERS.includes(member)) {
    return newSourceTaint(`request ${member}`, node, scope);
  }
  if (base === "process" && member === "argv") {
    return newSourceTaint("process.argv", node, scope);
  }
  return null;
}

function newSourceTaint(label, node, scope) {
  const line = lineNumber(scope.sourceFile, node.getStart(scope.sourceFile));
  return {
    kind: "source",
    steps: [`${label} (${scope.relPath}:${line})`],
    hops: 0,
    sanitized: [],
  };
}

function taintsOfIdentifier(node, scope) {
  const symbol = scope.ctx.checker.getSymbolAtLocation(node);
  if (!symbol) {
    return [];
  }
  if (scope.params.has(symbol)) {
    return [{ kind: "param", param: scope.params.get(symbol), steps: [], hops: 0, sanitized: [] }];
  }
  return scope.env.get(symbol) || [];
}

function taintsOfCallResult(node, scope) {
  const sanitizer = sanitizerKindsFor(taintCalleeName(node.expression));
  if (sanitizer) {
    return sanitizeTaints(taintArgumentUnion(node, scope), sanitizer);
  }
  const target = resolveTaintCallee(node, scope.ctx);
  if (target) {
    return taintsThroughSummary(node, target, scope);
  }
  return taintsThroughOpaqueCall(node, scope);
}

function sanitizerKindsFor(name) {
  if (!name) {
    return null;
  }
  if (name === "encodeURIComponent" || name === "encodeURI") {
    return ["url"];
  }
  if (/^(escape|sanitize)/i.test(name) || TAINT_SANITIZER_NAMES.has(name)) {
    return "all";
  }
  return null;
}

function sanitizeTaints(taints, kinds) {
  if (kinds === "all") {
    return [];
  }
  return taints.map((taint) => ({ ...taint, sanitized: taint.sanitized.concat(kinds) }));
}

function taintArgumentUnion(node, scope) {
  return (node.arguments || []).reduce(
    (taints, argument) => taints.concat(taintsOfExpression(argument, scope)),
    [],
  );
}

function taintsThroughSummary(node, target, scope) {
  const summary = taintSummaryFor(target, scope.ctx);
  const calleeName = taintCalleeName(node.expression) || "callee";
  const line = lineNumber(scope.sourceFile, node.getStart(scope.sourceFile));
  const returnStep = `${calleeName}() return (${scope.relPath}:${line})`;
  const taints = [];
  (node.arguments || []).forEach((argument, index) => {
    if (!summary.paramReturns.has(index)) {
      return;
    }
    const extraHops = summary.paramReturns.get(index);
    for (const taint of taintsOfExpression(argument, scope)) {
      const hops = taint.hops + extraHops + 1;
      if (hops > scope.ctx.depthCap) {
        noteTaintDepthCap(calleeName, line, scope);
        continue;
      }
      taints.push({ ...taint, hops, steps: taint.steps.concat([returnStep]) });
    }
  });
  for (const sourceTaint of summary.sourceReturns) {
    const hops = sourceTaint.hops + 1;
    if (hops > scope.ctx.depthCap) {
      noteTaintDepthCap(calleeName, line, scope);
      continue;
    }
    taints.push({ ...sourceTaint, hops, steps: sourceTaint.steps.concat([returnStep]) });
  }
  return taints;
}

function taintsThroughOpaqueCall(node, scope) {
  const expression = node.expression;
  if (ts.isPropertyAccessExpression(expression)) {
    // String helpers such as value.trim() or value.toString() preserve taint.
    return taintsOfExpression(expression.expression, scope);
  }
  if (ts.isIdentifier(expression) && expression.text === "String") {
    return taintArgumentUnion(node, scope);
  }
  return [];
}

function capTaintList(taints) {
  return taints.length > TAINT_MAX_TAINTS_PER_EXPRESSION
    ? taints.slice(0, TAINT_MAX_TAINTS_PER_EXPRESSION)
    : taints;
}

function taintCalleeName(expression) {
  if (ts.isIdentifier(expression)) {
    return expression.text;
  }
  if (ts.isPropertyAccessExpression(expression)) {
    return expression.name.text;
  }
  return "";
}

function resolveTaintCallee(node, ctx) {
  let nameNode = null;
  if (ts.isIdentifier(node.expression)) {
    nameNode = node.expression;
  } else if (ts.isPropertyAccessExpression(node.expression)) {
    nameNode = node.expression.name;
  } else {
    return null;
  }
  let symbol = ctx.checker.getSymbolAtLocation(nameNode);
  if (!symbol) {
    return null;
  }
  if (symbol.flags & ts.SymbolFlags.Alias) {
    symbol = ctx.checker.getAliasedSymbol(symbol);
  }
  for (const declaration of symbol.declarations || []) {
    const fn = taintFunctionFromDeclaration(declaration);
    if (fn && isAnalyzableSourceFile(fn.getSourceFile())) {
      return fn;
    }
  }
  return null;
}

function taintFunctionFromDeclaration(declaration) {
  if (isTaintAnalyzableFunction(declaration)) {
    return declaration;
  }
  if (ts.isVariableDeclaration(declaration) && isTaintAnalyzableFunction(declaration.initializer)) {
    return declaration.initializer;
  }
  if (ts.isPropertyAssignment(declaration) && isTaintAnalyzableFunction(declaration.initializer)) {
    return declaration.initializer;
  }
  if (ts.isPropertyDeclaration(declaration) && isTaintAnalyzableFunction(declaration.initializer)) {
    return declaration.initializer;
  }
  if (ts.isExportAssignment(declaration) && isTaintAnalyzableFunction(declaration.expression)) {
    return declaration.expression;
  }
  return null;
}
