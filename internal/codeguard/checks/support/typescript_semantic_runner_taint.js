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

const TAINT_DEFAULT_MAX_DEPTH = 8;
const TAINT_MAX_TAINTS_PER_EXPRESSION = 16;
const TAINT_SOURCE_MEMBERS = ["query", "body", "params", "headers", "cookies"];
const TAINT_SANITIZER_NAMES = new Set([
  "sqlEscape", "shellQuote", "shellEscape", "htmlEncode", "htmlEscape", "encodeHTML",
]);

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
      return;
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
