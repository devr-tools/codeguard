const fs = require("fs");
const path = require("path");

const input = JSON.parse(fs.readFileSync(0, "utf8"));
const ts = require(input.typescript_lib_path);
const targetPath = path.resolve(input.target_path);

const results = { design: [], quality: [], security: [], debug: [] };
const seen = new Set();
const taintModel = normalizeTaintModel(input.taint_model);

const directivePatterns = [
  { pattern: /^\s*(?:(?:\/\/)|(?:\/\*+)|\*)\s*@ts-ignore\b/, suffix: "ts-ignore", message: "suppression comment should be reviewed" },
  { pattern: /^\s*(?:(?:\/\/)|(?:\/\*+)|\*)\s*@ts-nocheck\b/, suffix: "ts-nocheck", message: "file-level type checking is disabled" },
  { pattern: /^\s*(?:(?:\/\/)|(?:\/\*+)|\*)\s*@ts-expect-error\b/, suffix: "ts-expect-error", message: "suppression comment should be reviewed" },
];

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
  const rootNames = input.source_files;
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
      rootNames: rootNames || parsed.fileNames.filter((name) => isWithinTarget(path.resolve(name))),
      options: parsed.options,
    });
  }

  return ts.createProgram({
    rootNames: rootNames || ts.sys.readDirectory(targetPath, scriptExtensions(), undefined, undefined),
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
